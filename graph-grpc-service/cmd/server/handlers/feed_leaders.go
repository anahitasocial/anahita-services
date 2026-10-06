package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
)

type feedLeadersHandler struct {
	graphpb.FeedLeadersServiceServer
	env *Env
}

func NewFeedLeadersHandler(env *Env) *feedLeadersHandler {
	return &feedLeadersHandler{env: env}
}

// Count return the total number of feed nodes from leaders
func (h *feedLeadersHandler) Count(ctx context.Context, req *graphpb.FeedLeadersCountRequest) (*graphpb.FeedLeadersCountResponse, error) {
	log.Printf("Received FeedLeaders Count request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	total, err := h.env.Repos.FeedLeaders.Count(req, viewer)
	if err != nil {
		return nil, err
	}

	return &graphpb.FeedLeadersCountResponse{
		Total: total,
	}, nil
}

// Browse return a list of feed nodes from leaders
func (h *feedLeadersHandler) Browse(ctx context.Context, req *graphpb.FeedLeadersBrowseRequest) (*graphpb.FeedLeadersBrowseResponse, error) {
	log.Printf("Received FeedLeaders Browse request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	results, err := h.env.Repos.FeedLeaders.Browse(req, viewer)
	if err != nil {
		return nil, err
	}

	objectIDs := make([]int64, 0)
	for _, item := range results {
		if item.Type == constants.NodeBaseRepostType {
			objectIDs = append(objectIDs, item.Parent.ID)
		} else {
			objectIDs = append(objectIDs, item.ID)
		}
	}

	isLikedByViewerMap, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, objectIDs)
	if err != nil {
		return nil, err
	}

	repostedMap, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, objectIDs)
	if err != nil {
		return nil, err
	}

	// log.Println("repostStatuses:", repostStatuses)
	isRepostedByViewerStatuses := make(map[int64]bool, len(objectIDs))
	for _, id := range objectIDs {
		_, ok := repostedMap[id]
		isRepostedByViewerStatuses[id] = ok
	}

	// iterate through the stories and get the comments
	for _, node := range results {
		if node.Type == constants.NodeBaseRepostType {
			node.Parent.IsLikedByViewer = isLikedByViewerMap[node.Parent.ID].IsLiked
			node.Parent.IsRepostedByViewer = isRepostedByViewerStatuses[node.Parent.ID]
		} else {
			node.IsLikedByViewer = isLikedByViewerMap[node.ID].IsLiked
			node.IsRepostedByViewer = isRepostedByViewerStatuses[node.ID]
		}
	}

	return &graphpb.FeedLeadersBrowseResponse{
		Nodes: nodesToGrpc(results),
	}, nil
}

// How far back, and how many, when the caller does not say or says too much.
const (
	activeLeadersHoursDefault = 24
	activeLeadersHoursMax     = 168
	activeLeadersLimitDefault = 30
	activeLeadersLimitMax     = 50
)

// Active returns the actors the viewer follows that have posted lately.
func (h *feedLeadersHandler) Active(ctx context.Context, req *graphpb.FeedLeadersActiveRequest) (*graphpb.FeedLeadersActiveResponse, error) {
	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	hours := req.GetHours()
	if hours == 0 {
		hours = activeLeadersHoursDefault
	} else if hours > activeLeadersHoursMax {
		hours = activeLeadersHoursMax
	}

	limit := req.GetLimit()
	if limit == 0 {
		limit = activeLeadersLimitDefault
	} else if limit > activeLeadersLimitMax {
		limit = activeLeadersLimitMax
	}

	leaders, err := h.env.Repos.FeedLeaders.Active(viewer, hours, limit, req.GetMediumEntityTypes())
	if err != nil {
		log.Println(err)
		return nil, err
	}

	resp := &graphpb.FeedLeadersActiveResponse{}
	for _, leader := range leaders {
		resp.Leaders = append(resp.Leaders, &graphpb.ActiveLeader{
			ActorId:  leader.ActorID,
			LatestAt: leader.LatestAt.Unix(),
			Count:    leader.Count,
		})
	}

	return resp, nil
}
