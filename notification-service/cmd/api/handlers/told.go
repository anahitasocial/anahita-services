package handlers

import (
	"log"

	"anahita-libs/constants"
	an_entities "anahita-libs/data/entities"
)

// Whose notifications (roadmap row 146): what is yours, or your channel's,
// tells you of everyone; what you take part in tells you of the people you
// follow, and of whoever answers you (docs/notifications.md, Who tells you
// what).

// peopleBehind is who is told for what a profile wrote: the person, or, for
// a channel (roadmap row 147), each of its admins, who write as it. A
// profile on another server is told by its own server: nobody here.
func (h *eventHandler) peopleBehind(actorID int64) []int64 {
	if actorID <= 0 {
		return nil
	}
	node, err := h.Srv.GraphNodeGRPC.Read(actorID)
	if err != nil || node == nil || node.ID == 0 || node.IsRemote() {
		return nil
	}
	if node.Type != constants.NodeActorChannelEntityType {
		return []int64{node.ID}
	}
	admins, err := h.Srv.ActorAdminsGRPC.Browse(node.ID)
	if err != nil {
		log.Printf("the admins of %d, to tell them: %v", node.ID, err)
		return nil
	}
	ids := []int64{}
	for _, admin := range admins {
		if admin != nil && admin.ID > 0 && !admin.IsRemote() {
			ids = append(ids, admin.ID)
		}
	}
	return ids
}

// subscribersFollowing is the people here subscribed to a post who follow
// whoever did something to it: what one takes part in tells one of the
// people one follows, and of nobody else.
func (h *eventHandler) subscribersFollowing(post *an_entities.AnahitaNode, actorID int64) ([]int64, error) {
	subscribers, err := h.Srv.SubscriptionsGRPC.BrowseSources(post)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	for _, subscriber := range subscribers {
		if subscriber != nil && subscriber.ID > 0 && subscriber.ID != actorID && !subscriber.IsRemote() {
			ids = append(ids, subscriber.ID)
		}
	}
	if len(ids) == 0 {
		return ids, nil
	}
	// Each subscriber as actor A, whoever acted as actor B: IsFollowing is
	// A following B.
	rels, err := h.Srv.SocialgraphGRPC.Browse(ids, actorID)
	if err != nil {
		return nil, err
	}
	following := []int64{}
	for _, id := range ids {
		if rel, ok := rels[id]; ok && rel != nil && rel.IsFollowing {
			following = append(following, id)
		}
	}
	return following, nil
}

// subscribeTo makes a person here a subscriber of a post they took part
// in, by replying or reposting: they are told after of the people they
// follow there. A profile that is not a person, or one elsewhere, is not.
// A failure is logged: the reply or the repost stands.
func (h *eventHandler) subscribeTo(person, post *an_entities.AnahitaNode) {
	if person == nil || post == nil || person.ID <= 0 || post.ID <= 0 ||
		person.IsRemote() || person.Type != constants.NodeActorPersonEntityType {
		return
	}
	viewer := an_entities.NewViewer()
	viewer.ID = person.ID
	if err := h.Srv.SubscriptionsGRPC.Add(person, post, viewer); err != nil {
		log.Printf("subscribing %d to %d, which they took part in: %v", person.ID, post.ID, err)
	}
}

// uniqueOthers is these people once each, without whoever acted.
func uniqueOthers(actorID int64, groups ...[]int64) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	for _, group := range groups {
		for _, id := range group {
			if id <= 0 || id == actorID || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
