package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
)

// Reposts across servers.
//
// A repost here is a node of its own on the reposter's profile, pointing at
// what was reposted. Between servers it is an `Announce`, what Mastodon
// calls a boost, taken back with an `Undo` that names it.
//
// Going out: somebody on the fediverse reposts a note that other servers
// can see (one from elsewhere, or one here that went out). Their followers'
// servers are told, and the note's author's when it is from elsewhere.
//
// Coming in: an `Announce` of something here that went out, or of something
// kept from elsewhere by somebody followed from here, is kept as a repost
// by the person kept for whoever sent it: counted, shown on their profile
// and in the feeds of who follows them, and the author told when the note
// is one of this installation's.

// repostService is the service a repost is filed under, as feed-service
// files the ones made here.
const repostService = "feed-service"

// announceID is what a repost is called between servers.
func (h *Handlers) announceID(uuid string) string {
	return h.addresses.Activity("announce-" + uuid)
}

// reposted is somebody here having reposted something: when it is something
// other servers can see, they are told.
func (h *Handlers) reposted(ctx context.Context, repostID int64) error {
	repost, err := h.Nodes.Read(repostID)
	if err != nil || repost == nil || repost.ID == 0 || repost.UUID == "" {
		return err
	}
	if repost.Type != constants.NodeBaseRepostType || !repost.Enabled || repost.IsRemote() {
		return nil
	}
	if repost.CreatedBy <= 0 || repost.OwnerID != repost.CreatedBy {
		return nil
	}

	reposter, err := h.Nodes.Read(repost.CreatedBy)
	if err != nil || reposter == nil || reposter.IsRemote() || h.federating(reposter) == nil {
		return err
	}

	note, err := h.Nodes.Read(repost.ParentID)
	if err != nil || note == nil || note.ID == 0 {
		return err
	}
	if !isPost(note) || !note.Enabled || note.DeletedAt != nil {
		return nil
	}

	// What is reposted, by the address it has between servers, and where
	// its author is written to when they are elsewhere.
	var object, author, inbox string
	if note.IsRemote() {
		if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(note.URI)) {
			return nil
		}
		kept, err := h.Actors.ReadByNode(note.OwnerID)
		if err != nil || kept == nil {
			return err
		}
		object, author = note.URI, kept.URI
		if inbox = kept.SharedInbox; inbox == "" {
			inbox = kept.Inbox
		}
	} else {
		sent, err := h.Objects.Read(note.ID)
		if err != nil || sent == nil || sent.Kind == repositories.KindRepost {
			// A note that stayed here: so does the repost of it.
			return err
		}
		object = h.addresses.Note(sent.UUID)
	}

	begun, err := h.Objects.Begin(&repositories.Object{
		NodeID: repost.ID, UUID: repost.UUID, ActorID: reposter.ID,
		ReplyInbox: inbox, Kind: repositories.KindRepost,
	})
	if err != nil || !begun {
		// Heard of already, by this copy of the service or another.
		return err
	}

	actor := h.addresses.Actor(reposter.UUID)
	cc := []string{h.addresses.Followers(reposter.UUID)}
	if author != "" {
		cc = append(cc, author)
	}
	published := ""
	if !repost.CreatedAt.IsZero() && repost.CreatedAt.Unix() > 0 {
		published = repost.CreatedAt.UTC().Format(time.RFC3339)
	}

	id := h.announceID(repost.UUID)
	return h.toReaders(ctx, reposter, inbox, ap.NewAnnounce(id, actor, object, published, cc...), "announce:"+id)
}

// unreposted is a repost that went out having been taken back, or gone:
// the servers that were told of it are told to undo it.
func (h *Handlers) unreposted(ctx context.Context, object *repositories.Object) error {
	ended, err := h.Objects.End(object.NodeID)
	if err != nil || !ended {
		return err
	}
	reposter, err := h.Nodes.Read(object.ActorID)
	if err != nil || reposter == nil || reposter.UUID == "" {
		return err
	}

	id := h.announceID(object.UUID)
	return h.toReaders(ctx, reposter, object.ReplyInbox,
		ap.NewUndoAnnounce(h.addresses.Activity(newUUID()), h.addresses.Actor(reposter.UUID), id), "unannounce:"+id)
}

// announced is somebody elsewhere having reposted something.
func (h *Handlers) announced(ctx context.Context, from *remote.Actor, activity *ap.Activity) error {
	what := activity.ObjectID()
	if what == "" || len(what) > 512 {
		return nil
	}

	// Something here that went out: anybody's repost of it counts.
	if note := h.answeredHere(what); note != nil && !note.IsRemote() {
		reposter, err := h.known(ctx, from)
		if err != nil || reposter == nil {
			return err
		}
		author, err := h.Nodes.Read(note.CreatedBy)
		if err != nil || author == nil || author.ID == 0 {
			return err
		}
		relation, err := h.Relations.Read(author.ID, reposter.ID)
		if err != nil {
			return err
		}
		if relation != nil && relation.IsBlocking {
			return nil
		}
		return h.keepRepost(reposter, note, true)
	}

	// Something from elsewhere: kept only when somebody here follows
	// whoever reposted it, the rule everything from elsewhere is kept by.
	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil || kept == nil {
		return err
	}
	followed, err := h.Follows.IsFollowedFromHere(kept.NodeID)
	if err != nil || !followed {
		return err
	}
	reposter, err := h.Nodes.Read(kept.NodeID)
	if err != nil || reposter == nil || reposter.ID == 0 {
		return err
	}

	note := h.answeredHere(what)
	if note == nil {
		// Not known here: it is read from its own server, with whoever
		// wrote it.
		if note, err = h.fetchNote(ctx, what); err != nil || note == nil {
			return err
		}
	}
	return h.keepRepost(reposter, note, false)
}

// unannounced is somebody elsewhere having taken a repost back.
func (h *Handlers) unannounced(from *remote.Actor, inner *ap.Activity) error {
	if inner.ActorID() != "" && inner.ActorID() != from.ID {
		return nil
	}
	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil || kept == nil {
		return err
	}
	note := h.answeredHere(inner.ObjectID())
	if note == nil {
		return nil
	}
	has, err := h.Reposts.IsReposted(kept.NodeID, note.ID)
	if err != nil || !has {
		return err
	}
	if err := h.Reposts.Delete(&services.RepostDeleteRequest{ParentID: note.ID, AuthorID: kept.NodeID}); err != nil {
		return err
	}

	// The author here was told of the repost; the news of it goes with it.
	if h.Announce == nil || note.IsRemote() {
		return nil
	}
	payload := &event.EventPayload{
		Name:          event.RemoteRepostDeleted,
		SourceService: repostService,
		SubjectID:     kept.NodeID,
		SubjectType:   constants.NodeActorActivityPubPersonEntityType,
		ObjectID:      note.ID,
		ObjectType:    note.Type,
		TargetID:      kept.NodeID,
		TargetType:    constants.NodeActorActivityPubPersonEntityType,
		ViewerID:      kept.NodeID,
	}
	h.later(func() {
		if err := h.Announce(payload, event.RemoteRepostDeleted); err != nil {
			log.Printf("saying that a repost of %d was taken back: %v", note.ID, err)
		}
	})
	return nil
}

// keepRepost records a repost by somebody elsewhere, once, and says so on
// the bus when its author is somebody here to be told.
func (h *Handlers) keepRepost(reposter, note *entities.AnahitaNode, tell bool) error {
	has, err := h.Reposts.IsReposted(reposter.ID, note.ID)
	if err != nil || has {
		return err
	}

	viewer := entities.NewViewer()
	viewer.ID = reposter.ID
	repost, err := h.Reposts.Add(&services.RepostAddRequest{
		OwnerID: reposter.ID, ParentID: note.ID, Service: repostService, Access: note.Access, Viewer: viewer,
	})
	if err != nil || repost == nil || !tell || h.Announce == nil {
		return err
	}

	// As feed-service says it of a repost made here, so that the author is
	// told. This service's own listener leaves it: it is not a repost by
	// somebody here.
	parts := strings.Split(note.Type, ".")
	kind := "note"
	if len(parts) > 3 {
		kind = parts[3]
	}
	name := fmt.Sprintf(event.EventFormatNodeRepostAdded, kind)
	payload := &event.EventPayload{
		Name:          name,
		SourceService: repostService,
		SubjectID:     reposter.ID,
		SubjectType:   reposter.Type,
		ObjectID:      repost.ID,
		ObjectType:    constants.NodeBaseRepostType,
		TargetID:      reposter.ID,
		TargetType:    reposter.Type,
		ViewerID:      reposter.ID,
	}
	h.later(func() {
		if err := h.Announce(payload, name); err != nil {
			log.Printf("saying that %s reposted %d: %v", reposter.Alias, note.ID, err)
		}
	})
	return nil
}

// fetchNote reads a note from its own server and keeps it, with whoever
// wrote it: something reposted by somebody followed from here, that nobody
// here had seen. Only a note that begins a thread, open to everybody, by
// somebody on the same server as the note.
func (h *Handlers) fetchNote(ctx context.Context, address string) (*entities.AnahitaNode, error) {
	if h.Fetcher == nil || h.Site.IsOurs(address) {
		return nil, nil
	}
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(address)) {
		return nil, nil
	}

	var note ap.ReceivedNote
	if _, err := h.Fetcher.Object(ctx, address, &note); err != nil {
		// Gone, private, or a server that is away: nothing to show.
		log.Printf("a reposted note at %s could not be read: %v", address, err)
		return nil, nil
	}
	if note.Type != "Note" || note.ID == "" || !sameHost(note.ID, address) || len(note.ID) > 512 {
		return nil, nil
	}
	if !note.IsPublic() || note.IsReply() || !sameHost(note.Author(), note.ID) || h.accountRefused(note.Author()) {
		return nil, nil
	}

	writer, err := remote.Fetch(ctx, h.Fetcher, note.Author(), h.Site.IsOurs)
	if err != nil || writer == nil {
		return nil, nil
	}
	author, err := h.known(ctx, writer)
	if err != nil || author == nil {
		return nil, err
	}
	if err := h.store(author.ID, &note, 0); err != nil {
		return nil, err
	}
	kept, err := h.Nodes.ReadByURI(note.ID)
	if err != nil || kept == nil || kept.ID == 0 {
		return nil, err
	}
	return kept, nil
}

// RereadFromTheCommandLine reads a post that is kept here again from its
// own server and keeps what it says now: its words, and the card of the
// link in it. Only a post that is already kept, by the address it has
// there; where it stands in a thread does not change.
func (h *Handlers) RereadFromTheCommandLine(ctx context.Context, address string) error {
	kept, err := h.Nodes.ReadByURI(address)
	if err != nil {
		return err
	}
	if kept == nil || kept.ID == 0 || !kept.IsRemote() || !isPost(kept) {
		return fmt.Errorf("no post from another server is kept by that address")
	}
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(address)) {
		return fmt.Errorf("that server is refused")
	}

	var note ap.ReceivedNote
	if _, err := h.Fetcher.Object(ctx, address, &note); err != nil {
		return err
	}
	if note.Type != "Note" || note.ID != kept.URI {
		return fmt.Errorf("what is at that address now is not that post")
	}
	return h.store(kept.OwnerID, &note, kept.ParentID)
}
