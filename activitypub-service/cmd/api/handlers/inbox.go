package handlers

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/fetch"
	"activitypub/httpsig"
	"activitypub/keys"
	"activitypub/remote"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"

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

// Inbox takes what another server sends.
func (h *Handlers) Inbox(c echo.Context) error {
	if ok, _ := h.siteFederates(); !ok {
		return c.NoContent(http.StatusNotFound)
	}

	req := c.Request()
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
	waiting := relation != nil && relation.HasRequested

	switch {
	case already:
		// Asked again by somebody who follows already: told yes again.

	case person.AllowFollowRequest:
		// A profile that approves its followers: the request waits for the
		// person here, like any other, and nothing is answered yet.
		follow.State = repositories.FollowPending
		if !waiting {
			add := services.NewSocialgraphFollowRequest().Add
			add.Source, add.Target, add.Viewer, add.IsFollowRequest = follower, person, viewer, true
			if err := h.FollowEdges.Add(add); err != nil {
				return err
			}
		}
		return h.Follows.Save(follow)

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
