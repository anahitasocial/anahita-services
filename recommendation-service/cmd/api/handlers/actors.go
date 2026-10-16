package handlers

import (
	"log"
	"net/http"
	"time"

	"recommendation/constants"
	"recommendation/scoring"

	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// Suggestions of who to follow.
//
// What is found, and why, is worked out here from the graph as it stands:
// who follows whom, who was invited by whom, what was liked. No model, and
// nothing kept from one asking to the next. Every suggestion is sent with
// the reasons for it.
//
// Two things are not decided here. Who may be SUGGESTED is the signals' one
// clause, in data/repositories. Who the viewer may SEE is the graph's rule,
// asked of the graph (FilterVisible) for the best of what was found before
// any of it is sent.

type actorsHandler struct {
	*Env
}

// ActorsRequest asks who the viewer might follow.
type ActorsRequest struct {
	// Only people, only groups, or only events. All of them when left out.
	Kind string `query:"kind" validate:"omitempty,oneof=person group event"`
	// `home` (the default), or `onboarding` for somebody who has just joined
	// and follows nobody: nothing worked out from who they follow is used.
	Context string `query:"context" validate:"omitempty,oneof=home onboarding"`
	// The most to send: up to 50, 10 when left out.
	Limit int `query:"limit" validate:"omitempty,min=1,max=50"`
}

// SuggestionItem is one suggestion as it is sent: the profile, and why.
type SuggestionItem struct {
	// The profile's id.
	ActorID int64 `json:"actor_id"`
	// What it scored: higher is better.
	Score float64 `json:"score"`
	// Why, the one that counted most first.
	Reasons []scoring.Reason `json:"reasons"`
	// The profile.
	Actor *responses.NodeItem `json:"actor"`
}

// ActorsResponse is who the viewer might follow, and why.
type ActorsResponse struct {
	// The suggestions, best first.
	Items []*SuggestionItem `json:"items"`
	// What the reasons name, for saying them in words: the people who
	// follow somebody suggested, the hashtags shared. The ones the viewer
	// may see.
	Related []*responses.ActorBrief `json:"related"`
}

// Browse returns who the viewer might follow, best first.
func (h *actorsHandler) Browse(c echo.Context) error {
	viewer := getViewer(c)
	if !signedIn(viewer) {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &ActorsRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if req.Kind == "" {
		req.Kind = constants.KindPerson
	}
	if req.Limit == 0 {
		req.Limit = constants.DefaultLimit
	}

	resp := &ActorsResponse{Items: []*SuggestionItem{}, Related: []*responses.ActorBrief{}}

	prefs, err := h.Repos.Preferences.Read(viewer.ID)
	if err != nil {
		log.Println("recommendations: preferences:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	// Asked not to be suggested this kind: nothing, and nothing looked for.
	if (req.Kind == constants.KindPerson && !prefs.SuggestPeople) ||
		(req.Kind == constants.KindGroup && !prefs.SuggestGroups) ||
		(req.Kind == constants.KindEvent && !prefs.SuggestEvents) {
		return c.JSON(http.StatusOK, resp)
	}

	now := time.Now().UTC()

	found, err := h.Repos.Signals.Find(viewer.ID, req.Kind, prefs.UseActivity, req.Context == constants.ContextOnboarding, now)
	if err != nil {
		log.Println("recommendations: signals:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	candidates := candidateIDs(found)
	active, err := h.Repos.Signals.LastActive(candidates)
	if err != nil {
		log.Println("recommendations: last active:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// An event that is still to come is as alive as anything is, however
	// long ago it was put in the calendar.
	if req.Kind == constants.KindEvent {
		for _, id := range candidates {
			active[id] = now
		}
	}

	// By the viewer's own weights, where they have set any.
	ranked := scoring.Rank(found, active, now, constants.WeightsFor(prefs.Weights))

	// Somebody who has been away for longer than the viewer chose is not
	// suggested at all, whatever else speaks for them. A group that is
	// quiet is only put last: it is still there to be joined and woken.
	if req.Kind == constants.KindPerson {
		ranked = scoring.ActiveWithin(ranked, active, now, prefs.ActiveWithinDays)
	}
	if len(ranked) > constants.GateSize {
		ranked = ranked[:constants.GateSize]
	}
	if len(ranked) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	// Which of them the viewer may see. The graph's answer, in no order;
	// the ranking's order is kept.
	ids := make([]int64, 0, len(ranked))
	for _, suggestion := range ranked {
		ids = append(ids, suggestion.ActorID)
	}
	visible, err := h.Srv.GraphNodeGRPC.FilterVisible(ids, viewer)
	if err != nil {
		log.Println("recommendations: visibility:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	seen := toSet(visible)

	kept := make([]scoring.Suggestion, 0, req.Limit)
	for _, suggestion := range ranked {
		if seen[suggestion.ActorID] && len(kept) < req.Limit {
			kept = append(kept, suggestion)
		}
	}
	if len(kept) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	actors := h.readActors(kept, viewer)
	// An event says when it is, and how many are going.
	if req.Kind == constants.KindEvent {
		list := make([]*entities.AnahitaNode, 0, len(actors))
		for _, actor := range actors {
			list = append(list, actor)
		}
		an_handlers.SetEventDetails(h.Srv.EventsGRPC, list, viewer)
	}
	nodeResp := &responses.NodeResponse{}

	for _, suggestion := range kept {
		actor, ok := actors[suggestion.ActorID]
		if !ok {
			continue
		}
		resp.Items = append(resp.Items, &SuggestionItem{
			ActorID: suggestion.ActorID,
			Score:   suggestion.Score,
			Reasons: suggestion.Reasons,
			Actor:   nodeResp.MapNodeItem(actor),
		})
	}

	resp.Related = h.readRelated(resp.Items, viewer)

	return c.JSON(http.StatusOK, resp)
}

// candidateIDs is everybody any signal found, once each.
func candidateIDs(found map[string][]scoring.Hit) []int64 {
	seen := map[int64]bool{}
	ids := []int64{}

	for _, hits := range found {
		for _, hit := range hits {
			if !seen[hit.ID] {
				seen[hit.ID] = true
				ids = append(ids, hit.ID)
			}
		}
	}

	return ids
}

func toSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// readActors reads the profiles to send, with their pictures made ready.
func (h *actorsHandler) readActors(kept []scoring.Suggestion, viewer *entities.Viewer) map[int64]*entities.AnahitaNode {
	ids := make([]int64, 0, len(kept))
	for _, suggestion := range kept {
		ids = append(ids, suggestion.ActorID)
	}

	req := services.NewActorGRPCRequest().BatchRead
	req.IDs = ids

	list, err := h.Srv.ActorsGRPC.BatchRead(req, viewer)
	if err != nil {
		log.Println("recommendations: reading actors:", err)
		return map[int64]*entities.AnahitaNode{}
	}

	return h.withPictures(list)
}

// withPictures signs the addresses of these nodes' avatars and covers, and
// returns the nodes by id.
func (env *Env) withPictures(list []*entities.AnahitaNode) map[int64]*entities.AnahitaNode {
	nodes := make(map[int64]*entities.AnahitaNode, len(list))

	var avatars []entities.AnahitaNodeAvatarInterface
	var covers []entities.AnahitaNodeCoverInterface
	for _, node := range list {
		if node == nil || node.ID == 0 {
			continue
		}
		nodes[node.ID] = node
		if !node.IsActor() {
			continue
		}
		if node.AvatarFilename != "" {
			avatars = append(avatars, node)
		}
		if node.CoverFilename != "" {
			covers = append(covers, node)
		}
	}

	if env.Srv.FilesGRPC != nil {
		if len(avatars) > 0 {
			services.BatchPresignAvatar(env.Srv.FilesGRPC, avatars)
		}
		if len(covers) > 0 {
			services.BatchPresignCover(env.Srv.FilesGRPC, covers)
		}
	}

	return nodes
}

// readRelated reads what the reasons name, the ones the viewer may see: a
// person who follows somebody suggested is somebody the viewer follows, and
// is asked about all the same.
func (h *actorsHandler) readRelated(items []*SuggestionItem, viewer *entities.Viewer) []*responses.ActorBrief {
	related := []*responses.ActorBrief{}

	seen := map[int64]bool{}
	ids := []int64{}
	for _, item := range items {
		for _, reason := range item.Reasons {
			for _, id := range reason.IDs {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	if len(ids) == 0 {
		return related
	}

	visible, err := h.Srv.GraphNodeGRPC.FilterVisible(ids, viewer)
	if err != nil || len(visible) == 0 {
		return related
	}

	list, err := h.Srv.GraphNodeGRPC.BatchRead(&services.GraphNodeBatchReadRequest{IDs: visible, Viewer: viewer})
	if err != nil {
		log.Println("recommendations: reading what the reasons name:", err)
		return related
	}

	nodes := h.withPictures(list)
	for _, id := range visible {
		if node, ok := nodes[id]; ok {
			related = append(related, responses.NewActorBrief(node))
		}
	}

	return related
}

// dismissedActors reads the profiles somebody turned down, to list them
// back: the ones still there for them to see.
func (env *Env) dismissedActors(ids []int64, viewer *entities.Viewer) []*responses.ActorBrief {
	listed := []*responses.ActorBrief{}
	if len(ids) == 0 {
		return listed
	}

	visible, err := env.Srv.GraphNodeGRPC.FilterVisible(ids, viewer)
	if err != nil || len(visible) == 0 {
		return listed
	}
	seen := toSet(visible)

	list, err := env.Srv.GraphNodeGRPC.BatchRead(&services.GraphNodeBatchReadRequest{IDs: visible, Viewer: viewer})
	if err != nil {
		log.Println("recommendations: reading dismissed actors:", err)
		return listed
	}

	nodes := env.withPictures(list)
	for _, id := range ids {
		if node, ok := nodes[id]; ok && seen[id] {
			listed = append(listed, responses.NewActorBrief(node))
		}
	}

	return listed
}
