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
	"anahita-libs/event"
	"anahita-libs/federation"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
	"anahita-libs/videos"
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
		} else if inner != nil && inner.Type == "Like" {
			err = h.unliked(ctx, from, inner)
		} else if inner != nil && inner.Type == "Announce" {
			err = h.unannounced(from, inner)
		}
	case "Like":
		err = h.liked(ctx, from, &activity)
	case "Announce":
		err = h.announced(ctx, from, &activity)
	case "Accept", "Reject":
		err = h.answered(from, &activity, activity.Type == "Accept")
	case "Update":
		// Themselves: their name, their words about themselves, their key.
		if activity.ObjectID() == from.ID {
			err = h.refreshed(ctx, from)
		} else {
			err = h.received(ctx, from, &activity)
		}
	case "Create":
		err = h.received(ctx, from, &activity)
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
// everybody. A reply is kept by rules of its own: see receivedReply.
func (h *Handlers) received(ctx context.Context, from *remote.Actor, activity *ap.Activity) error {
	note := activity.Note()
	if note == nil {
		return nil
	}
	if note.IsReply() {
		return h.receivedReply(ctx, from, note)
	}

	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil || kept == nil {
		return err
	}
	return h.keepNote(kept, note)
}

// keepNote keeps one post of somebody elsewhere, if it is one that is kept.
// The same rule whether it was sent here or read from the list of what they
// have posted.
func (h *Handlers) keepNote(author *repositories.RemoteActor, note *ap.ReceivedNote) error {
	if note == nil || note.Type != "Note" || note.Author() != author.URI {
		return nil
	}
	// Its address must be on its author's own server.
	if !sameHost(note.ID, author.URI) || h.Site.IsOurs(note.ID) || len(note.ID) > 512 {
		return nil
	}
	if !note.IsPublic() || note.IsReply() {
		return nil
	}

	followed, err := h.Follows.IsFollowedFromHere(author.NodeID)
	if err != nil || !followed {
		return err
	}

	return h.store(author.NodeID, note, 0)
}

// store keeps a note that has been found to be one that is kept: on its
// author's profile, and under what it answers when it answers something.
func (h *Handlers) store(authorID int64, note *ap.ReceivedNote, parentID int64) error {
	// With pictures it is a photo post, which is what a post with pictures
	// is here. What it was first kept as it stays, whatever came after.
	pictures := h.picturesToKeep(note)
	kind := constants.NodeMediumNoteEntityType
	if known, err := h.Nodes.ReadByURI(note.ID); err == nil && known != nil && known.ID != 0 {
		kind = known.Type
	} else if len(pictures) > 0 {
		kind = constants.NodeMediumPhotoEntityType
	}

	node := entities.NewAnahitaNode()
	node.Type = kind
	node.URI = note.ID
	node.OriginDomain = hostOf(note.ID)
	node.OwnerID = authorID
	node.ParentID = parentID
	node.Body = cutRunes(h.namedHere(federation.WithoutCustomEmoji(federation.PostToMarkdown(note.Content), note.Tag)), receivedNoteMax)
	node.Language = note.Language()
	if published, err := time.Parse(time.RFC3339, note.Published); err == nil {
		node.CreatedAt = published
	}
	if node.Body == "" && kind != constants.NodeMediumPhotoEntityType {
		// Nothing to show: no words, and no picture that is kept.
		return nil
	}
	// The card of the link in it, read by this installation itself.
	node.Meta = h.withCard(node.Body)

	saved, created, err := h.Nodes.UpsertRemote(node)
	if err != nil {
		return err
	}
	h.announceKept(saved, authorID, created)

	// Its pictures are fetched after it is kept, and not waited for: the
	// server that sent it is answered now, and a picture takes a while.
	if saved != nil && saved.Type == constants.NodeMediumPhotoEntityType && len(pictures) > 0 {
		h.later(func() { h.copyPostPictures(saved, pictures) })
	}
	return nil
}

// isPost says whether something is a post other servers can be spoken to
// about: a note, a photo post or a video post, which between servers are a
// note with pictures or with a video. Video posts were left out of this
// when they began to go out, so a reply under one, a repost of one and a
// reply to one from elsewhere were all refused.
func isPost(node *entities.AnahitaNode) bool {
	if node == nil {
		return false
	}
	return node.Type == constants.NodeMediumNoteEntityType ||
		node.Type == constants.NodeMediumPhotoEntityType ||
		node.Type == videos.NodeType
}

// announceKept says that a post from another server was kept or changed,
// so that it is given its hashtags: the ones it names are made if they are
// not there, as for a post written here, and it is found under them.
//
// Not waited for, and not tried again: a post whose hashtags were missed
// is still kept and shown, and is given them the next time it changes.
func (h *Handlers) announceKept(note *entities.AnahitaNode, authorID int64, created bool) {
	if h.Announce == nil || note == nil || note.ID == 0 || authorID == 0 {
		return
	}

	name := event.RemoteNoteEvent(created)
	payload := &event.EventPayload{
		Name:          name,
		SourceService: "activitypub-service",
		OwnerID:       authorID,
		OwnerType:     constants.NodeActorActivityPubPersonEntityType,
		SubjectID:     authorID,
		SubjectType:   constants.NodeActorActivityPubPersonEntityType,
		ObjectID:      note.ID,
		ObjectType:    note.Type,
		// Whoever wrote it is who a hashtag it is the first to use is
		// put down to.
		ViewerID: authorID,
	}

	h.later(func() {
		if err := h.Announce(payload, name); err != nil {
			log.Printf("saying that %s was kept: %v", note.URI, err)
		}
	})
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
	// What it was here, to say afterwards that it is gone, and what
	// pictures of it are in storage.
	was, _ := h.Nodes.ReadByURI(gone)
	var pictures []*entities.PhotoFile
	if was != nil && was.Type == constants.NodeMediumPhotoEntityType && h.PhotoFiles != nil {
		pictures, _ = h.PhotoFiles.BrowsePhotoFiles([]int64{was.ID})
	}

	// The graph takes it away only if it is theirs.
	deleted, err := h.Nodes.DeleteRemote(gone, kept.NodeID)
	if err != nil || !deleted || was == nil || was.ID == 0 {
		return err
	}
	if len(pictures) > 0 {
		h.later(func() { h.deletePostPictures(was, pictures) })
	}
	if h.Announce == nil {
		return nil
	}

	// Whoever was told of it, a reply to something of theirs, is not left
	// with news that leads nowhere.
	payload := &event.EventPayload{
		Name:          event.RemoteNoteDeleted,
		SourceService: "activitypub-service",
		OwnerID:       kept.NodeID,
		OwnerType:     constants.NodeActorActivityPubPersonEntityType,
		SubjectID:     kept.NodeID,
		SubjectType:   constants.NodeActorActivityPubPersonEntityType,
		ObjectID:      was.ID,
		ObjectType:    was.Type,
		ViewerID:      kept.NodeID,
	}
	h.later(func() {
		if err := h.Announce(payload, event.RemoteNoteDeleted); err != nil {
			log.Printf("saying that %s is gone: %v", gone, err)
		}
	})
	return nil
}

// forget removes somebody from another server and everything of theirs.
func (h *Handlers) forget(kept *repositories.RemoteActor) error {
	// What this service keeps goes first, so that the follows' going is not
	// taken for somebody here unfollowing, and answered with an Undo.
	if err := h.Follows.DeleteAllWith(kept.NodeID); err != nil {
		return err
	}
	// Their picture, which is in this installation's storage.
	if node, err := h.Nodes.Read(kept.NodeID); err == nil {
		h.deleteAvatar(node)
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
