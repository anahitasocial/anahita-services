package handlers

import (
	"context"
	"log"
	"regexp"
	"strings"

	"activitypub/ap"
	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
)

// A reply from another server.
//
// It is kept when what it answers is known here, and then by one of two
// rules, by where the thread began:
//
//   - Here. Its author may reply by the thread's own rule, as anybody here
//     must: whoever the root's author lets reply, and the profile's rule
//     for who may comment, where somebody elsewhere counts as a registered
//     reader. Somebody who chose to be on the fediverse chose to be
//     answered from it. Whoever wrote it need not be known already: they
//     are looked up and kept.
//   - Elsewhere. Only when somebody here follows its author, the rule
//     every note from elsewhere is kept by.
//
// What it answers is not fetched when it is not here: a reply to something
// unknown is dropped.
func (h *Handlers) receivedReply(ctx context.Context, from *remote.Actor, note *ap.ReceivedNote) error {
	if note.Type != "Note" || note.Author() != from.ID {
		return nil
	}
	if !sameHost(note.ID, from.ID) || h.Site.IsOurs(note.ID) || len(note.ID) > 512 {
		return nil
	}
	if !note.IsPublic() {
		return nil
	}

	parent := h.answeredHere(note.Answers())
	if parent == nil {
		return nil
	}
	root := parent
	if parent.RootID > 0 {
		var err error
		if root, err = h.Nodes.Read(parent.RootID); err != nil || root == nil || root.ID == 0 {
			return err
		}
	}
	// A thread that is not open to everybody is not one somebody elsewhere
	// could have read, whatever they sent.
	if !root.Enabled || root.Access != constants.AnahitaAccessPublic {
		return nil
	}

	if root.IsRemote() {
		kept, err := h.Actors.ReadByURI(from.ID)
		if err != nil || kept == nil {
			return err
		}
		followed, err := h.Follows.IsFollowedFromHere(kept.NodeID)
		if err != nil || !followed {
			return err
		}
		return h.store(kept.NodeID, note, parent.ID)
	}

	author, err := h.known(ctx, from)
	if err != nil || author == nil {
		return err
	}
	may, err := h.mayAnswer(author, root)
	if err != nil {
		return err
	}
	if !may {
		log.Printf("inbox: a reply from %s to %d is not kept: they may not reply to it", from.ID, root.ID)
		return nil
	}
	return h.store(author.ID, note, parent.ID)
}

// answeredHere finds what a reply answers, when it is something that is
// here and that other servers have: a note of this installation's that
// went out, or a note kept from another server. Nil for anything else.
func (h *Handlers) answeredHere(address string) *entities.AnahitaNode {
	if address == "" || len(address) > 512 {
		return nil
	}

	var (
		found *entities.AnahitaNode
		err   error
	)
	if h.Site.IsOurs(address) {
		uuid := h.addresses.NoteUUIDOf(address)
		if !uuidShape.MatchString(uuid) {
			return nil
		}
		if found, err = h.Nodes.ReadByUUID(uuid); err != nil || found == nil || found.ID == 0 || found.IsRemote() {
			return nil
		}
		// One that other servers were told of. Anything else here is not
		// theirs to answer, whatever address they guessed.
		if sent, err := h.Objects.Read(found.ID); err != nil || sent == nil {
			return nil
		}
	} else if found, err = h.Nodes.ReadByURI(address); err != nil || found == nil || found.ID == 0 || !found.IsRemote() {
		return nil
	}

	if !isPost(found) || !found.Enabled || found.DeletedAt != nil {
		return nil
	}
	return found
}

// mayAnswer says whether somebody elsewhere may reply in a thread that
// began here: the rule anybody here is held to, asked of them.
func (h *Handlers) mayAnswer(author, root *entities.AnahitaNode) (bool, error) {
	// The rule is asked of the root with its owner beside it.
	if root.Owner == nil || root.Owner.ID == 0 {
		owner, err := h.Nodes.Read(root.OwnerID)
		if err != nil || owner == nil || owner.ID == 0 {
			return false, err
		}
		root.Owner = owner
	}

	ownerRel, err := h.Relations.Read(root.OwnerID, author.ID)
	if err != nil {
		return false, err
	}
	authorRel := ownerRel
	if writer := root.GetAuthorID(); writer > 0 && writer != root.OwnerID {
		if authorRel, err = h.Relations.Read(writer, author.ID); err != nil {
			return false, err
		}
	}
	// Somebody the thread's owner or its author has blocked says nothing
	// in it.
	for _, rel := range []*entities.ActorRelation{ownerRel, authorRel} {
		if rel != nil && rel.IsBlocking {
			return false, nil
		}
	}

	// Measured as a reader who is signed in and may write posts: what
	// somebody elsewhere is, here, once their server has vouched for them
	// with a signature. Never an administrator, and never anybody's own.
	reader := entities.NewViewer()
	reader.ID = author.ID
	reader.Authenticated = true
	reader.Username = author.Alias
	reader.Scopes = []string{constants.OAuthScopeMediaWrite}

	return permissions.CanReply(&permissions.ReplyAccessContext{
		Viewer:              reader,
		Root:                root,
		RootOwnerViewerRel:  ownerRel,
		RootAuthorViewerRel: authorRel,
	}), nil
}

// A link around somebody's name, as a post from another server has it:
// [@ana@example.org](https://example.org/people/ana).
var namedLink = regexp.MustCompile(`\[@([^\]\s]+)\]\((https?://[^)\s]+)\)`)

// namedHere puts the name alone where a post from another server links to
// somebody of this installation: "@ana", as it is written here, which the
// web app shows as it shows any mention.
//
// Mastodon begins a reply with the name of whoever is answered, as a link
// to their profile. Left as it came, a reply under Ana's note would begin
// with her whole handle and a link out to her own page. Somebody on another
// server keeps their link: there is no page for them here by that name.
func (h *Handlers) namedHere(markdown string) string {
	return namedLink.ReplaceAllStringFunc(markdown, func(link string) string {
		parts := namedLink.FindStringSubmatch(link)
		address := parts[2]
		if !h.Site.IsOurs(address) {
			return link
		}

		alias := h.addresses.AliasOf(address)
		if alias == "" {
			if person := h.personByUUID(h.addresses.UUIDOf(address)); person != nil {
				alias = person.Alias
			}
		}
		if alias == "" {
			return link
		}
		// Only when the link says it is to that person: a link that reads
		// as one name and leads to another is left to be seen for what it
		// is.
		written := strings.ToLower(strings.SplitN(parts[1], "@", 2)[0])
		if written != strings.ToLower(alias) {
			return link
		}
		return "@" + alias
	})
}
