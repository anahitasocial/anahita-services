package handlers

import (
	"context"
	"errors"
	"log"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/feeds"
	"anahita-libs/proto/graphpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type feedCustomHandler struct {
	graphpb.FeedCustomServiceServer
	env *Env
}

func NewFeedCustomHandler(env *Env) *feedCustomHandler {
	return &feedCustomHandler{env: env}
}

// Browse returns a page of a custom feed, as the viewer may see it.
//
// The definition is read here again, by the code that checked it when it
// was saved: this service builds a query from it and takes nobody's word
// that it is sound.
func (h *feedCustomHandler) Browse(ctx context.Context, req *graphpb.FeedCustomBrowseRequest) (*graphpb.FeedCustomBrowseResponse, error) {
	def, err := feeds.Parse([]byte(req.Definition))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	viewer := entities.NewViewer()
	if req.Viewer != nil {
		viewer.ID = req.Viewer.Id
		viewer.PersonType = req.Viewer.PersonType
		viewer.Username = req.Viewer.Username
		viewer.Email = req.Viewer.Email
	}

	before := feeds.Cursor{}
	if req.BeforeId > 0 && req.BeforeTime > 0 {
		before = feeds.Cursor{CreatedAt: time.Unix(req.BeforeTime, 0).UTC(), ID: req.BeforeId}
	}

	results, err := h.env.Repos.FeedCustom.Browse(def, req.OwnerId, before, req.Limit, viewer)
	if err != nil {
		// Too much to look through in the time a query is given: said as
		// such, so the editor can ask for the feed to be narrowed.
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, string(feeds.RefusedTooBroad))
		}
		log.Println("custom feed:", err)
		return nil, err
	}

	ids := make([]int64, 0, len(results))
	for _, node := range results {
		ids = append(ids, node.ID)
	}

	liked, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, ids)
	if err != nil {
		return nil, err
	}
	reposted, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, ids)
	if err != nil {
		return nil, err
	}

	for _, node := range results {
		node.IsLikedByViewer = liked[node.ID].IsLiked
		_, node.IsRepostedByViewer = reposted[node.ID]
	}

	markSaved(h.env, viewer.ID, results)

	return &graphpb.FeedCustomBrowseResponse{
		Nodes: nodesToGrpc(results),
	}, nil
}
