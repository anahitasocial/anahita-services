package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
//   - a note, written here, by somebody who is on the fediverse;
//   - on their own profile, not in a group and not on somebody else's;
//   - open to everybody: not to members, not to followers;
//   - not a reply. Replies are not sent yet;
//   - and a visitor may see it, by the graph's own reckoning.
//
// A post that stops being all of these after it went out is taken back from
// the servers that have it, as far as they will.

// goesOut says whether a post is one that other servers are told of, and
// who wrote it.
func (h *Handlers) goesOut(note *entities.AnahitaNode) *entities.AnahitaNode {
	if note == nil || note.ID == 0 || note.UUID == "" || note.IsRemote() {
		return nil
	}
	if note.Type != constants.NodeMediumNoteEntityType || !note.Enabled {
		return nil
	}
	if note.Access != constants.AnahitaAccessPublic {
		return nil
	}
	if note.ParentID > 0 || note.RootID > 0 {
		return nil
	}
	if note.CreatedBy <= 0 || note.OwnerID != note.CreatedBy {
		return nil
	}

	author, err := h.Nodes.Read(note.CreatedBy)
	if err != nil || h.federating(author) == nil {
		return nil
	}
	if !libhandlers.MaySee(h.Nodes, note.ID, entities.NewViewer()) {
		return nil
	}
	return author
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
	author := h.goesOut(note)
	if author == nil {
		return c.NoContent(http.StatusNotFound)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=180")
	return activityJSON(c, ap.NewNote(h.addresses, note, author, h.links()).WithContext())
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

	author := h.goesOut(note)
	if author == nil {
		// Not one that goes out. If it did before (it was open to
		// everybody and is not now), it is taken back.
		return h.removed(ctx, nodeID)
	}

	sent := ap.NewNote(h.addresses, note, author, h.links())

	begun, err := h.Objects.Begin(&repositories.Object{NodeID: note.ID, UUID: note.UUID, ActorID: author.ID})
	if err != nil {
		return err
	}
	if begun {
		return h.toFollowers(ctx, author, ap.NewCreate(h.addresses, sent, note.UUID), "create:"+note.UUID)
	}

	// Known already: this is a change to it. Only when it says it was
	// changed, so that hearing of its making twice sends nothing twice.
	if sent.Updated == "" {
		return nil
	}
	return h.toFollowers(ctx, author, ap.NewUpdate(h.addresses.Activity(newUUID()), sent),
		fmt.Sprintf("update:%s:%s", note.UUID, sent.Updated))
}

// removed is a post having gone, or having stopped being one that goes out:
// the servers that were told of it are told to drop it. A post they were
// never told of is nothing to them, and nothing is sent.
func (h *Handlers) removed(ctx context.Context, nodeID int64) error {
	object, err := h.Objects.Read(nodeID)
	if err != nil || object == nil {
		return err
	}
	ended, err := h.Objects.End(nodeID)
	if err != nil || !ended {
		return err
	}

	author, err := h.Nodes.Read(object.ActorID)
	if err != nil || author == nil || author.UUID == "" {
		return err
	}
	return h.toFollowers(ctx, author,
		ap.NewDelete(h.addresses.Activity(newUUID()), h.addresses.Actor(author.UUID), h.addresses.Note(object.UUID)),
		"delete:"+object.UUID)
}
