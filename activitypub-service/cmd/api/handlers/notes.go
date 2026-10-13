package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"

	libhandlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/federation"

	"github.com/labstack/echo/v4"
)

// Posts going out.
//
// WHAT GOES OUT CANNOT BE CALLED BACK, so the rule for what does is narrow
// and is asked every time, of the post as it is now:
//
//   - a note or a photo post, written here, by somebody who is on the
//     fediverse. A photo post is sent as a note with its pictures;
//   - on their own profile, not in a group and not on somebody else's;
//   - open to everybody: not to members, not to followers;
//   - or a reply in a thread that is open to everybody, to something other
//     servers can see: see answers;
//   - not hidden, and not removed;
//   - and a visitor may see it, by the graph's own reckoning.
//
// A post that stops being all of these after it went out is taken back from
// the servers that have it, as far as they will.

// goesOut says whether a post is one that other servers are told of, and
// who wrote it; and for a reply, what it answers.
func (h *Handlers) goesOut(note *entities.AnahitaNode) (*entities.AnahitaNode, *answer) {
	if note == nil || note.ID == 0 || note.UUID == "" || note.IsRemote() {
		return nil, nil
	}
	// A note, or a photo post, which other servers are sent as a note with
	// its pictures.
	isPhoto := note.Type == constants.NodeMediumPhotoEntityType
	if (note.Type != constants.NodeMediumNoteEntityType && !isPhoto) || !note.Enabled || note.DeletedAt != nil || note.Hidden {
		return nil, nil
	}
	if note.CreatedBy <= 0 || note.OwnerID != note.CreatedBy {
		return nil, nil
	}

	// A reply says nothing of its own about who may read it: its thread
	// does. And it goes out only when what it answers is something other
	// servers can see.
	var answering *answer
	if note.ParentID > 0 || note.RootID > 0 {
		if answering = h.answers(note); answering == nil {
			return nil, nil
		}
	} else if note.Access != constants.AnahitaAccessPublic {
		return nil, nil
	}

	author, err := h.Nodes.Read(note.CreatedBy)
	if err != nil || h.federating(author) == nil {
		return nil, nil
	}
	if !libhandlers.MaySee(h.Nodes, note.ID, entities.NewViewer()) {
		return nil, nil
	}
	return author, answering
}

// answer is what a reply answers, as it is known between servers.
type answer struct {
	// The address of what is answered.
	ID string
	// Whoever wrote it, when they are on another server: their address,
	// what they are called, and where they are written to.
	Actor  string
	Handle string
	Inbox  string
}

// answers finds what a reply answers, or nil when the reply is not one
// that goes out: its thread is not open to everybody, or what it answers
// is nothing other servers have.
//
// What other servers have is a note that came from one of them, or a note
// here that itself went out. So a thread of somebody's own notes goes out
// whole, each on the heels of the one before, and a reply under a note
// that stayed here stays here too.
func (h *Handlers) answers(reply *entities.AnahitaNode) *answer {
	if reply.ParentID <= 0 || reply.RootID <= 0 {
		return nil
	}

	root, err := h.Nodes.Read(reply.RootID)
	if err != nil || root == nil || root.ID == 0 || !root.Enabled || root.Access != constants.AnahitaAccessPublic {
		return nil
	}

	parent := root
	if reply.ParentID != reply.RootID {
		if parent, err = h.Nodes.Read(reply.ParentID); err != nil || parent == nil || parent.ID == 0 {
			return nil
		}
	}
	if !parent.Enabled || parent.DeletedAt != nil || !isPost(parent) {
		return nil
	}

	if !parent.IsRemote() {
		sent, err := h.Objects.Read(parent.ID)
		if err != nil || sent == nil {
			return nil
		}
		return &answer{ID: h.addresses.Note(sent.UUID)}
	}

	// From another server: whoever wrote it is told.
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(parent.URI)) {
		return nil
	}
	kept, err := h.Actors.ReadByNode(parent.OwnerID)
	if err != nil || kept == nil {
		return nil
	}
	writer, err := h.Nodes.Read(parent.OwnerID)
	if err != nil || writer == nil || writer.Alias == "" {
		return nil
	}

	inbox := kept.SharedInbox
	if inbox == "" {
		inbox = kept.Inbox
	}
	return &answer{ID: parent.URI, Actor: kept.URI, Handle: "@" + writer.Alias, Inbox: inbox}
}

// How long a photo post's pictures are waited for before it is left to the
// sweep. A variable, so that tests do not wait.
var picturesWait = 2 * time.Second

func (h *Handlers) hasPictures(postID int64) bool {
	files, err := h.PhotoFiles.BrowsePhotoFiles([]int64{postID})
	return err == nil && len(files) > 0
}

// described is a note as other servers are sent it, a reply with what it
// answers.
func (h *Handlers) described(note, author *entities.AnahitaNode, answering *answer) *ap.Note {
	sent := ap.NewNote(h.addresses, note, author, h.links())
	if answering != nil {
		sent.Answering(answering.ID, answering.Actor, answering.Handle)
	}
	// A photo post goes with its pictures, each at an address of this
	// installation's that leads to it for as long as the post goes out.
	if note.Type == constants.NodeMediumPhotoEntityType && h.PhotoFiles != nil {
		if files, err := h.PhotoFiles.BrowsePhotoFiles([]int64{note.ID}); err == nil {
			sent.WithPictures(h.addresses, note, files)
		}
	}
	return sent
}

// Picture sends a server that asks for one of a photo post's pictures to
// where it is: a signed address in storage, good for a short while. Only a
// picture of a post that goes out, which is asked each time, so that a
// post closed to its followers stops giving its pictures to anybody.
func (h *Handlers) Picture(c echo.Context) error {
	uuid := c.Param("uuid")
	position, err := strconv.Atoi(c.Param("position"))
	if !uuidShape.MatchString(uuid) || err != nil || position < 0 || position >= postPicturesMax {
		return c.NoContent(http.StatusNotFound)
	}
	if ok, _ := h.siteFederates(); !ok || h.PhotoFiles == nil || h.Files == nil {
		return c.NoContent(http.StatusNotFound)
	}

	post, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || post == nil || post.Type != constants.NodeMediumPhotoEntityType {
		return c.NoContent(http.StatusNotFound)
	}
	if author, _ := h.goesOut(post); author == nil {
		return c.NoContent(http.StatusNotFound)
	}

	files, err := h.PhotoFiles.BrowsePhotoFiles([]int64{post.ID})
	if err != nil || position >= len(files) {
		return c.NoContent(http.StatusNotFound)
	}
	sizes, err := post.PortraitsFor(files[position].Filename)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	// The largest copy kept: other servers make their own smaller ones.
	stored, ok := sizes[entities.PortraitSizeOriginal]
	if !ok || stored.Url == "" {
		return c.NoContent(http.StatusNotFound)
	}

	request := services.NewFileRequest().Presign
	request.Path = stored.Url
	address, err := h.Files.Presign(request)
	if err != nil || address == "" {
		return c.NoContent(http.StatusNotFound)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=180")
	return c.Redirect(http.StatusFound, address)
}

// toReaders queues something about a post for everybody it is for: its
// author's followers and, for a reply, whoever it answers.
func (h *Handlers) toReaders(ctx context.Context, author *entities.AnahitaNode, replyInbox string, activity any, what string) error {
	if err := h.toFollowers(ctx, author, activity, what); err != nil {
		return err
	}
	if replyInbox == "" {
		return nil
	}
	// The same inbox twice is one row in the queue: a follower's server
	// that is also the server of whoever is answered is sent it once.
	return h.send(ctx, author.ID, replyInbox, activity, what)
}

// links says where the people and hashtags a post names are, for the links
// made of them. Somebody here is linked only when they are on the
// fediverse; somebody elsewhere, only when they are known here.
func (h *Handlers) links() federation.Links {
	return federation.Links{
		Hashtag: h.addresses.Hashtag,
		Mention: func(user, host string) string {
			if host == "" || h.isOurDomain(host) {
				if person := h.federating(h.personByAlias(user)); person != nil {
					return h.addresses.Actor(person.UUID)
				}
				return ""
			}
			req := services.NewGraphNodeRequest().ReadByAlias
			req.Alias = strings.ToLower(user) + "@" + host
			req.NodeType = constants.NodeActorActivityPubPersonEntityType
			if known, err := h.Nodes.ReadByAlias(req); err == nil && known != nil && known.URI != "" {
				return known.URI
			}
			return ""
		},
	}
}

// Note serves a post made here to a server that asks for it.
func (h *Handlers) Note(c echo.Context) error {
	uuid := c.Param("uuid")
	if !uuidShape.MatchString(uuid) {
		return c.NoContent(http.StatusNotFound)
	}
	if ok, _ := h.siteFederates(); !ok {
		return c.NoContent(http.StatusNotFound)
	}

	note, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || note == nil {
		return c.NoContent(http.StatusNotFound)
	}
	author, answering := h.goesOut(note)
	if author == nil {
		return c.NoContent(http.StatusNotFound)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=180")
	return activityJSON(c, h.described(note, author, answering).WithContext())
}

// Outbox says how many posts a person has sent out, and not which. A server
// that follows them is sent each as it is made.
func (h *Handlers) Outbox(c echo.Context) error {
	person := h.actorOf(c)
	if person == nil {
		return c.NoContent(http.StatusNotFound)
	}
	count, err := h.Objects.Count(person.ID)
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return activityJSON(c, ap.NewCollection(h.addresses.Outbox(person.UUID), count))
}

// toFollowers queues something for every server that has a follower of a
// person here.
func (h *Handlers) toFollowers(ctx context.Context, author *entities.AnahitaNode, activity any, what string) error {
	inboxes, err := h.Follows.FollowerInboxes(author.ID)
	if err != nil {
		return err
	}
	for _, inbox := range inboxes {
		if err := h.send(ctx, author.ID, inbox, activity, what); err != nil {
			return err
		}
	}
	return nil
}

// posted is a post having been made here, or changed: other servers are
// told, or told to drop it, by what it is now.
func (h *Handlers) posted(ctx context.Context, nodeID int64) error {
	note, err := h.Nodes.Read(nodeID)
	if err != nil {
		return err
	}
	if note == nil || note.ID == 0 {
		return h.removed(ctx, nodeID)
	}

	author, answering := h.goesOut(note)
	if author == nil {
		// Not one that goes out. If it did before (it was open to
		// everybody and is not now), it is taken back.
		return h.removed(ctx, nodeID)
	}

	// A photo post is heard of the moment it is made, a moment before its
	// pictures are listed. It is not sent without them: it is looked at
	// once more, and failing that is left for the sweep, which finds a post
	// that should have gone out and did not.
	if note.Type == constants.NodeMediumPhotoEntityType && h.PhotoFiles != nil {
		if !h.hasPictures(note.ID) {
			time.Sleep(picturesWait)
			if !h.hasPictures(note.ID) {
				return nil
			}
		}
	}

	sent := h.described(note, author, answering)
	replyInbox := ""
	if answering != nil {
		replyInbox = answering.Inbox
	}

	begun, err := h.Objects.Begin(&repositories.Object{NodeID: note.ID, UUID: note.UUID, ActorID: author.ID, ReplyInbox: replyInbox})
	if err != nil {
		return err
	}
	if begun {
		if err := h.toReaders(ctx, author, replyInbox, ap.NewCreate(h.addresses, sent, note.UUID), "create:"+note.UUID); err != nil {
			return err
		}
		return h.Objects.Told(note.ID, note.UpdatedAt)
	}

	// Known already: this is a change to it. Only when it says it was
	// changed, so that hearing of its making twice sends nothing twice.
	if sent.Updated == "" {
		return nil
	}
	if err := h.toReaders(ctx, author, replyInbox, ap.NewUpdate(h.addresses.Activity(newUUID()), sent),
		fmt.Sprintf("update:%s:%s", note.UUID, sent.Updated)); err != nil {
		return err
	}
	// What other servers now have, so that the sweep can tell a change
	// they were never sent from one they were.
	return h.Objects.Told(note.ID, note.UpdatedAt)
}

// removed is a post having gone, or having stopped being one that goes out:
// the servers that were told of it are told to drop it. A post they were
// never told of is nothing to them, and nothing is sent.
func (h *Handlers) removed(ctx context.Context, nodeID int64) error {
	object, err := h.Objects.Read(nodeID)
	if err != nil || object == nil {
		return err
	}
	// A repost is taken back by undoing it, not by saying it is gone.
	if object.Kind == repositories.KindRepost {
		return h.unreposted(ctx, object)
	}

	ended, err := h.Objects.End(nodeID)
	if err != nil || !ended {
		return err
	}

	author, err := h.Nodes.Read(object.ActorID)
	if err != nil || author == nil || author.UUID == "" {
		return err
	}
	return h.toReaders(ctx, author, object.ReplyInbox,
		ap.NewDelete(h.addresses.Activity(newUUID()), h.addresses.Actor(author.UUID), h.addresses.Note(object.UUID)),
		"delete:"+object.UUID)
}
