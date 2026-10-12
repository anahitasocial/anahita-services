package handlers

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/fetch"
	"activitypub/httpsig"
	"activitypub/keys"
	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/federation"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

type rsaKey = rsa.PrivateKey

// The inbox: where other servers send what somebody did.
//
// IT IS THE ONE DOOR IN THIS INSTALLATION THAT ANYBODY ON THE INTERNET MAY
// KNOCK AT, WITH ANYTHING. So nothing that arrives is acted on, or even
// read for what it says, until it is known who sent it:
//
//  1. it must be signed, and the signature must check against the key of
//     whoever it names, fetched from that person's own server;
//  2. the person the message says did the thing must be the one who signed
//     it. Nobody is believed about what somebody else did;
//  3. only then is it read, and only the few things this installation
//     understands are done. Everything else is taken and ignored.

// The most of a message that is read. One that follows somebody is a few
// hundred bytes; a long post with its attachments listed, a few kilobytes.
const inboxMaxBytes = 1 << 20

// How much one address may send. A busy server sends a great deal, to one
// inbox for everybody here; this is well above that and well below what it
// takes to keep this service from anything else.
var inboxLimit = ratelimit.Limit{Max: 600, Window: time.Minute}

// Inbox takes what another server sends.
func (h *Handlers) Inbox(c echo.Context) error {
	if ok, _ := h.siteFederates(); !ok {
		return c.NoContent(http.StatusNotFound)
	}

	req := c.Request()

	// So much from one address in a minute, and no more: nothing here is
	// cheap enough to do for whoever asks as often as they like. Counted
	// before anything is read.
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "activitypub_inbox", c.RealIP(), inboxLimit) {
		return c.NoContent(http.StatusTooManyRequests)
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, inboxMaxBytes+1))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if len(body) > inboxMaxBytes {
		return c.NoContent(http.StatusRequestEntityTooLarge)
	}

	sig, err := httpsig.Parse(req)
	if err != nil {
		return c.NoContent(http.StatusUnauthorized)
	}

	var activity ap.Activity
	if err := json.Unmarshal(body, &activity); err != nil || activity.Type == "" || activity.ActorID() == "" {
		return c.NoContent(http.StatusBadRequest)
	}

	// A server that is refused is not listened to, and is not asked for
	// anything either: its key is not fetched.
	if h.Blocks != nil && (h.Blocks.IsBlocked(hostOf(sig.KeyID)) || h.Blocks.IsBlocked(hostOf(activity.ActorID()))) {
		return c.NoContent(http.StatusForbidden)
	}

	// Somebody saying they are gone, whom nobody here ever heard of. A
	// large server tells every server it knows each time an account is
	// closed; nearly all of those are strangers here. There is nothing to
	// remove, so there is nothing to check, and their server (which no
	// longer has their key to give) is not asked.
	if activity.Type == "Delete" && activity.ObjectID() == activity.ActorID() {
		if kept, err := h.Actors.ReadByURI(activity.ActorID()); err == nil && kept == nil {
			return c.NoContent(http.StatusAccepted)
		}
	}

	ctx, cancel := context.WithTimeout(req.Context(), 3*fetch.Timeout)
	defer cancel()

	from, err := h.signedBy(ctx, req, body, sig)
	if err != nil {
		log.Printf("inbox: %s from %q refused: %v", activity.Type, sig.KeyID, err)
		return c.NoContent(http.StatusUnauthorized)
	}
	// Nobody is believed about what somebody else did.
	if from.ID != activity.ActorID() {
		log.Printf("inbox: %s signed by %s says it was done by %s", activity.Type, from.ID, activity.ActorID())
		return c.NoContent(http.StatusUnauthorized)
	}

	switch activity.Type {
	case "Follow":
		err = h.followed(ctx, from, &activity)
	case "Undo":
		if inner := activity.Inner(); inner != nil && inner.Type == "Follow" {
			err = h.unfollowed(from, inner)
		}
	case "Accept", "Reject":
		err = h.answered(from, &activity, activity.Type == "Accept")
	case "Update":
		// Themselves: their name, their words about themselves, their key.
		if activity.ObjectID() == from.ID {
			err = h.refreshed(ctx, from)
		} else {
			err = h.received(from, &activity)
		}
	case "Create":
		err = h.received(from, &activity)
	case "Delete":
		err = h.withdrawn(from, &activity)
	}
	if err != nil {
		// Ours to have done and not done. The sender tries again.
		log.Printf("inbox: %s from %s: %v", activity.Type, from.ID, err)
		return c.NoContent(http.StatusServiceUnavailable)
	}

	// Taken. Whether anything was done with it is not the sender's to know.
	return c.NoContent(http.StatusAccepted)
}

// signedBy says who signed a request, having checked that they did.
//
// The key is the one kept for them when they are known, and otherwise the
// one their own server gives for them now. A signature that fails against a
// kept key is tried once more against a fresh one: people's keys do change,
// and a kept one that is out of date must not shut them out for good.
func (h *Handlers) signedBy(ctx context.Context, req *http.Request, body []byte, sig *httpsig.Signature) (*remote.Actor, error) {
	if h.Site.IsOurs(sig.KeyID) {
		return nil, errors.New("signed in the name of this installation")
	}

	now := time.Now()

	if kept, err := h.Actors.ReadByKeyID(sig.KeyID); err == nil && kept != nil {
		if key, err := keys.ParsePublic(kept.PublicKey); err == nil {
			if err := httpsig.Verify(req, body, sig, key, now); err == nil {
				return &remote.Actor{
					ID: kept.URI, Inbox: kept.Inbox, SharedInbox: kept.SharedInbox,
					KeyID: kept.KeyID, PublicKeyPEM: kept.PublicKey, Locked: kept.Locked,
				}, nil
			} else if !errors.Is(err, httpsig.ErrBadSignature) {
				// Wrong for a reason a new key would not mend.
				return nil, err
			}
		}
	}

	owner, err := remote.KeyOwner(ctx, h.Fetcher, sig.KeyID)
	if err != nil {
		return nil, err
	}
	actor, err := remote.Fetch(ctx, h.Fetcher, owner, h.Site.IsOurs)
	if err != nil {
		return nil, err
	}
	if actor.KeyID != sig.KeyID {
		return nil, errors.New("the key named is not the one its owner has")
	}
	key, err := keys.ParsePublic(actor.PublicKeyPEM)
	if err != nil {
		return nil, err
	}
	if err := httpsig.Verify(req, body, sig, key, now); err != nil {
		return nil, err
	}
	return actor, nil
}

// followed is somebody elsewhere asking to follow somebody here.
func (h *Handlers) followed(ctx context.Context, from *remote.Actor, activity *ap.Activity) error {
	if activity.ID == "" || len(activity.ID) > 512 {
		return nil
	}

	// Who they asked to follow: somebody here, who is on the fediverse.
	person := h.federating(h.personByUUID(h.addresses.UUIDOf(activity.ObjectID())))
	if person == nil {
		return nil
	}

	follower, err := h.known(ctx, from)
	if err != nil {
		return err
	}
	actor := h.addresses.Actor(person.UUID)

	relation, err := h.Relations.Read(person.ID, follower.ID)
	if err != nil {
		return err
	}

	// Somebody the person here has blocked is told no.
	if relation != nil && relation.IsBlocking {
		return h.send(ctx, person.ID, from.Inbox,
			ap.NewReject(h.addresses.Activity(newUUID()), actor, from.ID, activity.ID), "reject:"+activity.ID)
	}

	follow := &repositories.Follow{
		LocalID: person.ID, RemoteID: follower.ID, Direction: repositories.FollowIn,
		ActivityID: activity.ID, State: repositories.FollowAccepted,
	}
	viewer := entities.NewViewer()
	viewer.ID = follower.ID

	already := relation != nil && relation.IsLeading

	switch {
	case already:
		// Asked again by somebody who follows already: told yes again.

	default:
		add := services.NewSocialgraphFollowRequest().Add
		add.Source, add.Target, add.Viewer = follower, person, viewer
		if err := h.FollowEdges.Add(add); err != nil {
			return err
		}
	}

	if err := h.Follows.Save(follow); err != nil {
		return err
	}
	return h.send(ctx, person.ID, from.Inbox,
		ap.NewAccept(h.addresses.Activity(newUUID()), actor, from.ID, activity.ID), "accept:"+activity.ID)
}

// answered is somebody elsewhere saying yes or no to a follow that
// somebody here asked for.
//
// It is matched to a follow this installation is waiting on: from the
// person who signed, of somebody here, and by what the follow was called
// when it was sent. An answer to a question nobody asked does nothing, so
// nobody can be made to follow somebody by being told they were accepted.
func (h *Handlers) answered(from *remote.Actor, activity *ap.Activity, yes bool) error {
	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil || kept == nil {
		return err
	}

	// What was answered: the Follow quoted whole, or only its address.
	followID := activity.ObjectID()
	var localUUID string
	if inner := activity.Inner(); inner != nil {
		if inner.Type != "" && inner.Type != "Follow" {
			return nil
		}
		localUUID = h.addresses.UUIDOf(inner.ActorID())
	}

	var follow *repositories.Follow
	if person := h.personByUUID(localUUID); person != nil {
		follow, err = h.Follows.Read(person.ID, kept.NodeID, repositories.FollowOut)
	} else if followID != "" {
		follow, err = h.Follows.ReadByActivity(followID)
	}
	if err != nil || follow == nil || follow.Direction != repositories.FollowOut || follow.RemoteID != kept.NodeID {
		return err
	}
	if followID != "" && followID != follow.ActivityID {
		return nil
	}

	if yes {
		follow.State = repositories.FollowAccepted
		return h.Follows.Save(follow)
	}

	// Told no, or dropped later: the follow here is taken away.
	follower, err := h.Nodes.Read(follow.LocalID)
	if err != nil || follower == nil {
		return err
	}
	followed, err := h.Nodes.Read(follow.RemoteID)
	if err != nil || followed == nil {
		return err
	}
	// Forgotten first, so that the follow's going is not taken for the
	// person here unfollowing, and answered with an Undo.
	if err := h.Follows.Delete(follow.LocalID, follow.RemoteID, repositories.FollowOut); err != nil {
		return err
	}
	viewer := entities.NewViewer()
	viewer.ID = follower.ID
	remove := services.NewSocialgraphFollowRequest().Delete
	remove.Source, remove.Target, remove.Viewer = follower, followed, viewer
	return h.FollowEdges.Delete(remove)
}

// The longest post that is kept. Longer ones are cut, and can be read whole
// where they were written.
const receivedNoteMax = 20000

// received is somebody elsewhere having posted, or changed a post.
//
// A post is kept only when somebody here follows whoever wrote it: this
// installation is not a copy of the fediverse, and what nobody here asked
// for is not stored. And only when it is the sender's own, open to
// everybody, and not a reply: replies are not kept yet.
func (h *Handlers) received(from *remote.Actor, activity *ap.Activity) error {
	note := activity.Note()
	if note == nil || note.Type != "Note" || note.Author() != from.ID {
		return nil
	}
	// Its address must be on its author's own server.
	if !sameHost(note.ID, from.ID) || h.Site.IsOurs(note.ID) || len(note.ID) > 512 {
		return nil
	}
	if !note.IsPublic() || note.IsReply() {
		return nil
	}

	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil || kept == nil {
		return err
	}
	followed, err := h.Follows.IsFollowedFromHere(kept.NodeID)
	if err != nil || !followed {
		return err
	}

	node := entities.NewAnahitaNode()
	node.Type = constants.NodeMediumNoteEntityType
	node.URI = note.ID
	node.OriginDomain = hostOf(note.ID)
	node.OwnerID = kept.NodeID
	node.Body = cutRunes(federation.ToMarkdown(note.Content), receivedNoteMax)
	node.Language = note.Language()
	if published, err := time.Parse(time.RFC3339, note.Published); err == nil {
		node.CreatedAt = published
	}
	if node.Body == "" {
		// Nothing to show: a picture alone, which is not kept yet.
		return nil
	}

	_, _, err = h.Nodes.UpsertRemote(node)
	return err
}

// withdrawn is somebody elsewhere having taken a post away.
func (h *Handlers) withdrawn(from *remote.Actor, activity *ap.Activity) error {
	gone := activity.ObjectID()
	if gone == "" || !sameHost(gone, from.ID) {
		return nil
	}

	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil || kept == nil {
		return err
	}

	// Themselves: their account was closed there. They go from here, with
	// what they posted and everybody's follows of them and theirs of
	// everybody.
	if gone == from.ID {
		return h.forget(kept)
	}
	// The graph takes it away only if it is theirs.
	_, err = h.Nodes.DeleteRemote(gone, kept.NodeID)
	return err
}

// forget removes somebody from another server and everything of theirs.
func (h *Handlers) forget(kept *repositories.RemoteActor) error {
	// What this service keeps goes first, so that the follows' going is not
	// taken for somebody here unfollowing, and answered with an Undo.
	if err := h.Follows.DeleteAllWith(kept.NodeID); err != nil {
		return err
	}
	if _, err := h.Nodes.DeleteRemoteActor(kept.URI); err != nil {
		return err
	}
	return h.Actors.Delete(kept.NodeID)
}

// refreshed is somebody elsewhere having changed their profile. What they
// sent is not taken at its word: they are read again from their own server,
// and believed as far as anybody is.
func (h *Handlers) refreshed(ctx context.Context, from *remote.Actor) error {
	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil || kept == nil {
		// Nobody kept here: nothing to bring up to date.
		return err
	}
	fetched, err := remote.Fetch(ctx, h.Fetcher, from.ID, h.Site.IsOurs)
	if err != nil {
		return err
	}
	_, err = h.keep(fetched)
	return err
}

func hostOf(address string) string {
	parsed, err := url.Parse(address)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Host)
}

func sameHost(a, b string) bool {
	host := hostOf(a)
	return host != "" && host == hostOf(b)
}

func cutRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}

// unfollowed is somebody elsewhere taking a follow back, or a request to.
func (h *Handlers) unfollowed(from *remote.Actor, follow *ap.Activity) error {
	// Whose follow it is, is whoever signed: an Undo of somebody else's
	// follow undoes nothing.
	if follow.ActorID() != "" && follow.ActorID() != from.ID {
		return nil
	}

	// Somebody here, whether or not they are still on the fediverse: a
	// follow may always be taken back.
	person := h.personByUUID(h.addresses.UUIDOf(follow.ObjectID()))
	if person == nil || person.IsRemote() {
		return nil
	}

	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil || kept == nil {
		return err
	}
	follower, err := h.Nodes.Read(kept.NodeID)
	if err != nil || follower == nil || follower.ID == 0 {
		return err
	}

	viewer := entities.NewViewer()
	viewer.ID = follower.ID

	// The follow, and the request to, whichever there is.
	for _, request := range []bool{false, true} {
		remove := services.NewSocialgraphFollowRequest().Delete
		remove.Source, remove.Target, remove.Viewer, remove.IsFollowRequest = follower, person, viewer, request
		if err := h.FollowEdges.Delete(remove); err != nil {
			return err
		}
	}

	return h.Follows.Delete(person.ID, follower.ID, repositories.FollowIn)
}

// known is the node kept for somebody elsewhere, made now if this is the
// first that is heard of them.
func (h *Handlers) known(ctx context.Context, from *remote.Actor) (*entities.AnahitaNode, error) {
	if kept, err := h.Actors.ReadByURI(from.ID); err == nil && kept != nil {
		if node, err := h.Nodes.Read(kept.NodeID); err == nil && node != nil && node.ID != 0 {
			return node, nil
		}
	}

	// Checked against a kept key, there is no description to keep them by:
	// it is read now.
	if from.Username == "" {
		fetched, err := remote.Fetch(ctx, h.Fetcher, from.ID, h.Site.IsOurs)
		if err != nil {
			return nil, err
		}
		from = fetched
	}
	return h.keep(from)
}
