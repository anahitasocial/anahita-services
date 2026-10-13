package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
)

// What people on Bluesky do to what was written from here.
//
// What is heard from Bluesky as it happens is what the accounts followed
// from here do. Somebody nobody here follows, who likes or answers a post
// that went out from here, is not among them. But the account the post
// went out as is told, as it would be in Bluesky's own app, and so each
// connected account is asked now and then what it was told: who liked or
// reposted something written from here, who answered it, and who followed
// the account.
//
// Each is kept the way the same thing from the fediverse is: whoever did
// it is kept as somebody of another server, and their like, repost, reply
// or follow is theirs, of the post or the person here. The person here is
// told as they are of anybody's.
//
// Only what is said to have been done is known this way, not what was
// taken back: a like that is withdrawn on Bluesky stays here.

// How many of the latest things an account was told of are read at once.
const reactionsRead = 50

// CheckReactions reads what each connected account was told of since it
// was last asked.
func (h *Handlers) CheckReactions() {
	if !h.canWrite() || h.Likes == nil || h.Reposts == nil || h.FollowEdges == nil || h.Relations == nil {
		return
	}
	accounts, err := h.Accounts.Working()
	if err != nil {
		log.Println("the connected Bluesky accounts could not be read:", err)
		return
	}
	for _, account := range accounts {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		if err := h.reactionsTo(ctx, account); err != nil {
			log.Printf("what %s was told of on Bluesky: %v", account.Handle, err)
		}
		cancel()
	}
}

// RunReactions does that now and then, until `ctx` is done.
func (h *Handlers) RunReactions(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.CheckReactions()
		}
	}
}

func (h *Handlers) reactionsTo(ctx context.Context, account *repositories.Account) error {
	token, err := h.token(ctx, account, false)
	if err != nil {
		return err
	}
	told, err := h.Writer.Notifications(ctx, account.PDS, token, reactionsRead)
	if errors.Is(err, bsky.ErrExpired) || errors.Is(err, bsky.ErrDenied) {
		if token, err = h.token(ctx, account, true); err != nil {
			return err
		}
		told, err = h.Writer.Notifications(ctx, account.PDS, token, reactionsRead)
	}
	if err != nil {
		return err
	}

	// What is new since last time, oldest first.
	type timed struct {
		at time.Time
		n  *bsky.Notification
	}
	var fresh []timed
	for i := range told {
		at, err := time.Parse(time.RFC3339, told[i].IndexedAt)
		if err != nil || !at.After(account.NotifiedAt) {
			continue
		}
		fresh = append(fresh, timed{at, &told[i]})
	}
	if len(fresh) == 0 {
		return nil
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].at.Before(fresh[j].at) })

	person, err := h.Nodes.Read(account.PersonID)
	if err != nil || person == nil || person.ID == 0 {
		return err
	}

	for _, each := range fresh {
		if err := h.reaction(ctx, account, person, each.n); err != nil {
			// Left where it is, to be tried again next time, with what
			// came after it.
			return err
		}
		if err := h.Accounts.SetNotified(account.PersonID, each.at); err != nil {
			return err
		}
	}
	return nil
}

// reaction keeps one thing somebody did.
func (h *Handlers) reaction(ctx context.Context, account *repositories.Account, person *entities.AnahitaNode, told *bsky.Notification) error {
	if !bsky.ValidDID(told.Author.DID) || told.Author.DID == account.DID || h.refused(strings.ToLower(told.Author.Handle)) {
		return nil
	}

	switch told.Reason {
	case "like", "repost":
		post := h.sentFromHere(account, told.ReasonSubject)
		if post == nil {
			return nil
		}
		actor, err := h.reactor(person, told)
		if err != nil || actor == nil {
			return err
		}
		if told.Reason == "like" {
			return h.likedFromBluesky(actor, post, person)
		}
		return h.repostedFromBluesky(actor, post)

	case "reply":
		reply := &bsky.Post{}
		if json.Unmarshal(told.Record, reply) != nil {
			return nil
		}
		parent := h.sentFromHere(account, reply.Answers())
		_, rkey, ok := bsky.ParsePostURI(told.URI)
		if parent == nil || !ok {
			return nil
		}
		// Written from here by somebody else connected: it is here.
		if h.ours(told.Author.DID, repositories.SentPost, rkey) {
			return nil
		}
		actor, err := h.reactor(person, told)
		if err != nil || actor == nil {
			return err
		}
		kept, err := h.Actors.ReadByNode(actor.ID)
		if err != nil || kept == nil {
			return err
		}
		_, err = h.storePost(kept, rkey, reply, parent.ID)
		return err

	case "follow":
		actor, err := h.reactor(person, told)
		if err != nil || actor == nil {
			return err
		}
		relation, err := h.Relations.Read(person.ID, actor.ID)
		if err != nil || (relation != nil && relation.IsLeading) {
			return err
		}
		viewer := entities.NewViewer()
		viewer.ID = actor.ID
		add := services.NewSocialgraphFollowRequest().Add
		add.Source, add.Target, add.Viewer = actor, person, viewer
		return h.FollowEdges.Add(add)
	}
	return nil
}

// sentFromHere is the post here that was written to Bluesky under an
// address, or nil: only what went out from here is anybody's to like or
// answer here.
func (h *Handlers) sentFromHere(account *repositories.Account, uri string) *entities.AnahitaNode {
	did, rkey, ok := bsky.ParsePostURI(uri)
	if !ok || did != account.DID {
		return nil
	}
	sent, err := h.Sent.ReadByName(did, repositories.SentPost, rkey)
	if err != nil || sent == nil {
		return nil
	}
	post, err := h.Nodes.Read(sent.LocalID)
	if err != nil || post == nil || post.ID == 0 || post.DeletedAt != nil || !post.Enabled {
		return nil
	}
	return post
}

// reactor is whoever did something, kept as somebody of another server.
// Nil for somebody the person here has blocked, and for somebody here who
// connected that account, whose doings here are already here.
func (h *Handlers) reactor(person *entities.AnahitaNode, told *bsky.Notification) (*entities.AnahitaNode, error) {
	node, err := h.keep(&bsky.Profile{
		DID: told.Author.DID, Handle: told.Author.Handle, DisplayName: told.Author.DisplayName,
		Description: told.Author.Description, Avatar: told.Author.Avatar,
	})
	if errors.Is(err, errConnectedHere) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	relation, err := h.Relations.Read(person.ID, node.ID)
	if err != nil {
		return nil, err
	}
	if relation != nil && relation.IsBlocking {
		return nil, nil
	}
	return node, nil
}

func (h *Handlers) likedFromBluesky(liker, post, author *entities.AnahitaNode) error {
	has, err := h.Likes.IsLiked(liker.ID, post.ID)
	if err != nil || has {
		return err
	}
	viewer := entities.NewViewer()
	viewer.ID = liker.ID
	if err := h.Likes.Add(liker, post, viewer); err != nil {
		return err
	}

	// Said on the bus as like-service says it, so that the author is told
	// as for a like made here.
	if h.Announce == nil {
		return nil
	}
	kind := strings.Split(post.Service, "-")[0]
	if kind == "" {
		kind = "text"
	}
	name := fmt.Sprintf(event.EventFormatEdgeVoteupAdded, kind)
	payload := &event.EventPayload{
		Name: name, SourceService: "like-service",
		OwnerID: author.ID, SubjectID: liker.ID, ObjectID: post.ID, TargetID: author.ID, ViewerID: liker.ID,
	}
	h.later(func() {
		if err := h.Announce(payload, name); err != nil {
			log.Printf("saying that %s liked %d: %v", liker.Alias, post.ID, err)
		}
	})
	return nil
}

func (h *Handlers) repostedFromBluesky(reposter, post *entities.AnahitaNode) error {
	has, err := h.Reposts.IsReposted(reposter.ID, post.ID)
	if err != nil || has {
		return err
	}
	viewer := entities.NewViewer()
	viewer.ID = reposter.ID
	repost, err := h.Reposts.Add(&services.RepostAddRequest{
		OwnerID: reposter.ID, ParentID: post.ID, Service: repostService, Access: post.Access, Viewer: viewer,
	})
	if err != nil || repost == nil || h.Announce == nil {
		return err
	}

	// As feed-service says it of a repost made here, so that the author
	// is told.
	parts := strings.Split(post.Type, ".")
	kind := "note"
	if len(parts) > 3 {
		kind = parts[3]
	}
	name := fmt.Sprintf(event.EventFormatNodeRepostAdded, kind)
	payload := &event.EventPayload{
		Name: name, SourceService: repostService,
		SubjectID: reposter.ID, SubjectType: reposter.Type, ObjectID: repost.ID, ObjectType: constants.NodeBaseRepostType,
		TargetID: reposter.ID, TargetType: reposter.Type, ViewerID: reposter.ID,
	}
	h.later(func() {
		if err := h.Announce(payload, name); err != nil {
			log.Printf("saying that %s reposted %d: %v", reposter.Alias, post.ID, err)
		}
	})
	return nil
}
