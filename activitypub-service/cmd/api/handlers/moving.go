package handlers

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"time"

	"activitypub/ap"
	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/permissions"
)

// Somebody followed, blocked or muted from here, on another server, moving
// their account (roadmap row 124, part 2; docs/plans/moving.md).
//
// WHO IS BELIEVED. A Move is signed by the old account, as everything in
// the inbox is checked to be, and names it as what moved. Its target, the
// new account, is read afresh from its own server, and must name the old
// one in its alsoKnownAs: only the person who holds both can have put it
// there. Neither may be refused here, and the two are not the same.
//
// WHAT FOLLOWS. The old profile points to the new one, by an edge the web
// app shows as "Moved to". Then, in the background, everybody here who
// blocked the old account blocks the new one; who muted it mutes the new
// one, for as long and as quietly; and who followed it, or asked to,
// follows the new one, or asks to, and stops following the old. Each is
// done through the graph as by hand, so the Follow, Undo and Block go out
// as they would, and each follower is told.
//
// ONCE. What is moved is not there to move again: a second Move, or the
// sweep finding a follow left by a restart, carries over only what is left.

var (
	errMoveNotTheirs = errors.New("a Move of somebody else")
	errMoveNoAlias   = errors.New("the new account does not name the old one")
)

// moved is a Move received from `from`.
func (h *Handlers) moved(ctx context.Context, from *remote.Actor, activity *ap.Activity) error {
	if activity.ObjectID() != from.ID {
		log.Printf("moving: %s sent a Move of %s", from.ID, activity.ObjectID())
		return nil
	}
	to := activity.TargetID()
	if to == "" || to == from.ID {
		return nil
	}

	// The old profile, as kept here. Nobody here knows them: nobody here
	// has anything to carry over.
	kept, err := h.Actors.ReadByURI(from.ID)
	if err != nil {
		return err
	}
	if kept == nil {
		return nil
	}

	newer, err := h.believedMove(ctx, from.ID, to)
	if err != nil {
		log.Printf("moving: %s to %s not believed: %v", from.ID, to, err)
		return nil
	}

	old, err := h.Nodes.Read(kept.NodeID)
	if err != nil || old == nil || old.ID == 0 {
		return err
	}
	if err := h.pointTo(old, newer); err != nil {
		return err
	}
	log.Printf("moving: %s moved to %s", old.Alias, newer.Alias)

	// To a person here: moving in, or moving back to an account that moved
	// away, whose signpost comes down and which is whole again.
	if !newer.IsRemote() {
		if account, err := h.Accounts.Read(newer.ID); err == nil && account != nil && account.MovedTo == from.ID {
			if err := h.Accounts.SetMoved(newer.ID, ""); err != nil {
				return err
			}
			log.Printf("moving: %s moved back from %s", newer.Alias, old.Alias)
			h.later(func() { h.describedAgain(context.Background(), newer, "back:"+from.ID) })
		}
	}

	h.later(func() { h.carryOver(old, newer) })
	return nil
}

// believedMove reads the account moved to from its own server and keeps it,
// when it names the old one as another of its own.
func (h *Handlers) believedMove(ctx context.Context, oldURI, newURI string) (*entities.AnahitaNode, error) {
	// Somebody here, who names the old account among their own (moving
	// in, or back).
	if h.Site.IsOurs(newURI) {
		return h.believedMoveHere(oldURI, newURI)
	}
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(newURI)) {
		return nil, errAccountRefused
	}
	if h.accountRefused(newURI) {
		return nil, errAccountRefused
	}
	actor, err := remote.Fetch(ctx, h.Fetcher, newURI, h.Site.IsOurs)
	if err != nil {
		return nil, err
	}
	if actor.ID != newURI {
		return nil, errMoveNotTheirs
	}
	if !slices.Contains(actor.AlsoKnownAs, oldURI) {
		return nil, errMoveNoAlias
	}
	return h.keep(actor)
}

// believedMoveHere is the person here an address is, when they name the old
// account as another of their own and are on the fediverse.
func (h *Handlers) believedMoveHere(oldURI, newURI string) (*entities.AnahitaNode, error) {
	_, uuid, ok := strings.Cut(newURI, "/ap/users/")
	if !ok || uuid == "" || strings.Contains(uuid, "/") || h.addresses.Actor(uuid) != newURI {
		return nil, errMoveNotTheirs
	}
	person, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || person == nil || person.ID == 0 || !person.IsPerson() {
		return nil, errMoveNotTheirs
	}
	account, err := h.Accounts.Read(person.ID)
	if err != nil || account == nil || !account.Enabled {
		return nil, errMoveNotTheirs
	}
	if !slices.Contains(account.AlsoKnownAs, oldURI) {
		return nil, errMoveNoAlias
	}
	return person, nil
}

// pointTo has the old profile point to the new one, in place of wherever it
// pointed before; a move back, from the new to the old, is no longer one.
func (h *Handlers) pointTo(old, newer *entities.AnahitaNode) error {
	if h.Edges == nil {
		return errors.New("moving: no graph edges")
	}
	moves := []string{constants.EdgeMovedEntityType}
	if err := h.Edges.BatchDeleteByFilters(&services.GraphEdgeBatchDeleteByFiltersRequest{
		NodeAID: newer.ID, NodeBID: old.ID, Types: moves,
	}); err != nil {
		return err
	}
	before, err := h.Edges.BatchReadByFilters(&services.GraphEdgeBatchReadByFiltersRequest{
		NodeAID: old.ID, Types: moves,
	})
	if err != nil {
		return err
	}
	for _, edge := range before {
		if edge.NodeBID == newer.ID {
			return nil
		}
		if err := h.Edges.Delete(edge.ID); err != nil {
			return err
		}
	}
	edge := entities.NewAnahitaEdge()
	edge.Type = constants.EdgeMovedEntityType
	edge.NodeAID, edge.NodeAType = old.ID, old.Type
	edge.NodeBID, edge.NodeBType = newer.ID, newer.Type
	// Said by the person who moved, from their old account.
	edge.CreatedBy = old.ID
	_, err = h.Edges.Add(edge)
	return err
}

// movedTo is the profile a profile moved to, or nil.
func (h *Handlers) movedTo(oldID int64) (*entities.AnahitaNode, error) {
	edges, err := h.Edges.BatchReadByFilters(&services.GraphEdgeBatchReadByFiltersRequest{
		NodeAID: oldID, Types: []string{constants.EdgeMovedEntityType},
	})
	if err != nil || len(edges) == 0 {
		return nil, err
	}
	return h.Nodes.Read(edges[0].NodeBID)
}

// Carried is what one carrying over did, for the log and the tests.
type Carried struct {
	Blocked, Muted, Followed, Asked, Left int
}

// carryOver carries what people here have with the old profile over to the
// new one: blocks, then mutes, then follows, so that nobody follows
// somebody they have just blocked.
func (h *Handlers) carryOver(old, newer *entities.AnahitaNode) Carried {
	var carried Carried
	now := time.Now()

	each := func(kind string, do func(*entities.AnahitaEdge, *entities.AnahitaNode)) {
		edges, err := h.Edges.BatchReadByFilters(&services.GraphEdgeBatchReadByFiltersRequest{
			NodeBID: old.ID, Types: []string{kind},
		})
		if err != nil {
			log.Printf("moving: the %s of %s: %v", kind, old.Alias, err)
			return
		}
		for _, edge := range edges {
			person, err := h.Nodes.Read(edge.NodeAID)
			if err != nil || person == nil || person.ID == 0 || person.IsRemote() || !person.IsPerson() {
				continue
			}
			do(edge, person)
		}
	}

	each(constants.EdgeBlockEntityType, func(_ *entities.AnahitaEdge, person *entities.AnahitaNode) {
		mine, err := h.Relations.Read(person.ID, newer.ID)
		if err != nil || mine == nil || mine.IsBlocking {
			return
		}
		if err := h.BlockEdges.Add(person, newer, viewerOf(person)); err != nil {
			log.Printf("moving: %s blocking %s: %v", person.Alias, newer.Alias, err)
			return
		}
		carried.Blocked++
	})

	each(constants.EdgeMuteEntityType, func(edge *entities.AnahitaEdge, person *entities.AnahitaNode) {
		if !edge.EndsAt.IsZero() && !edge.EndsAt.After(now) {
			return
		}
		mine, err := h.Relations.Read(person.ID, newer.ID)
		if err != nil || mine == nil || mine.IsMuting || mine.IsBlocking {
			return
		}
		hide := edge.Meta != constants.MuteMetaNotifications
		if err := h.MuteEdges.Add(person, newer, viewerOf(person), hide, edge.EndsAt); err != nil {
			log.Printf("moving: %s muting %s: %v", person.Alias, newer.Alias, err)
			return
		}
		carried.Muted++
	})

	for _, kind := range []string{constants.EdgeFollowEntityType, constants.EdgeFollowRequestEntityType} {
		asked := kind == constants.EdgeFollowRequestEntityType
		each(kind, func(_ *entities.AnahitaEdge, person *entities.AnahitaNode) {
			switch h.followInstead(person, old, newer, asked) {
			case event.RemoteMovedFollowed:
				carried.Followed++
			case event.RemoteMovedAsked:
				carried.Asked++
			default:
				carried.Left++
			}
		})
	}

	log.Printf("moving: %s to %s: %+v", old.Alias, newer.Alias, carried)
	return carried
}

// followInstead has a follower of the old profile follow the new one, or
// ask to, and stop following the old; the follower is told. It says which
// it did, as the event that told them, or "" for neither.
func (h *Handlers) followInstead(person, old, newer *entities.AnahitaNode, asked bool) string {
	viewer := viewerOf(person)
	done := ""

	theirs, err := h.Relations.Read(newer.ID, person.ID)
	mine, err2 := h.Relations.Read(person.ID, newer.ID)
	if err != nil || err2 != nil || theirs == nil || mine == nil {
		// Tried again by the sweep: the old follow is left.
		return ""
	}

	switch {
	// Following them there already: only the old follow goes.
	case theirs.IsLeading || theirs.HasRequested:
	// Not across a block, either way.
	case theirs.IsBlocking || mine.IsBlocking:
	default:
		rules := &permissions.FollowContext{
			Viewer: viewer, Source: person, Target: newer,
			TargetViewerRel: theirs, TargetSourceRel: theirs,
		}
		asking := false
		switch {
		case permissions.CanFollow(rules):
		case permissions.CanRequestToFollow(rules):
			asking = true
		default:
			rules = nil
		}
		if rules != nil {
			err := h.FollowEdges.Add(&services.SocialgraphFollowAddRequest{
				Source: person, Target: newer, Viewer: viewer, IsFollowRequest: asking,
			})
			if err != nil {
				log.Printf("moving: %s following %s: %v", person.Alias, newer.Alias, err)
				return ""
			}
			done = event.RemoteMovedFollowed
			if asking {
				done = event.RemoteMovedAsked
			}
		}
	}

	remove := services.NewSocialgraphFollowRequest().Delete
	remove.Source, remove.Target, remove.Viewer = person, old, viewer
	remove.IsFollowRequest = asked
	if err := h.FollowEdges.Delete(remove); err != nil {
		log.Printf("moving: %s unfollowing %s: %v", person.Alias, old.Alias, err)
	}

	if done != "" && h.Announce != nil {
		payload := &event.EventPayload{
			Name:          done,
			SourceService: "activitypub-service",
			SubjectID:     old.ID,
			SubjectType:   old.Type,
			ObjectID:      newer.ID,
			ObjectType:    newer.Type,
			TargetID:      person.ID,
			TargetType:    person.Type,
			ViewerID:      person.ID,
		}
		if err := h.Announce(payload, done); err != nil {
			log.Printf("moving: telling %s: %v", person.Alias, err)
		}
	}
	return done
}

// viewerOf is a person here as if signed in: what is carried over is done
// as they would do it themselves, by the same rules.
func viewerOf(person *entities.AnahitaNode) *entities.Viewer {
	viewer := entities.NewViewer()
	viewer.ID = person.ID
	viewer.Authenticated = true
	viewer.PersonType = person.PersonType
	return viewer
}

// movedLeft carries over what the sweep found left on profiles that moved:
// follows a restart, or a lost message, left behind.
func (h *Handlers) movedLeft(oldID int64) error {
	if h.Edges == nil {
		return errNotOne
	}
	newer, err := h.movedTo(oldID)
	if err != nil || newer == nil || newer.ID == 0 {
		return errNotOne
	}
	old, err := h.Nodes.Read(oldID)
	if err != nil || old == nil || old.ID == 0 {
		return errNotOne
	}
	h.carryOver(old, newer)
	return nil
}
