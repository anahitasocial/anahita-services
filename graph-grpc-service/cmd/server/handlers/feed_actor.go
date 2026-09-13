package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
)

type feedActorHandler struct {
	graphpb.FeedActorServiceServer
	env *Env
}

func NewFeedActorHandler(env *Env) *feedActorHandler {
	return &feedActorHandler{env: env}
}

// Count return the total number of feed nodes from leaders
func (h *feedActorHandler) Count(ctx context.Context, req *graphpb.FeedActorCountRequest) (*graphpb.FeedActorCountResponse, error) {
	log.Printf("Received FeedActor Count request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	total, err := h.env.Repos.FeedActor.Count(req, viewer)
	if err != nil {
		return nil, err
	}

	return &graphpb.FeedActorCountResponse{
		Total: total,
	}, nil
}

// Browse return a list of feed nodes from an actor
func (h *feedActorHandler) Browse(ctx context.Context, req *graphpb.FeedActorBrowseRequest) (*graphpb.FeedActorBrowseResponse, error) {
	log.Printf("Received FeedActor Browse request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	results, err := h.env.Repos.FeedActor.Browse(req, viewer)
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
	// log.Println("isLikedByViewerStatuses:", isLikedByViewerStatuses)

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
	for _, item := range results {
		if item.Type == constants.NodeBaseRepostType {
			item.Parent.IsLikedByViewer = isLikedByViewerMap[item.Parent.ID].IsLiked
			item.Parent.IsRepostedByViewer = isRepostedByViewerStatuses[item.Parent.ID]
		} else {
			item.IsLikedByViewer = isLikedByViewerMap[item.ID].IsLiked
			item.IsRepostedByViewer = isRepostedByViewerStatuses[item.ID]
		}
	}

	return &graphpb.FeedActorBrowseResponse{
		Nodes: nodesToGrpc(results),
	}, nil
}
