package handlers

import (
	"context"
	"log"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// markSaved says, on every post about to be sent to a viewer, whether that
// viewer has saved it. For a repost the mark goes on the post it reposts,
// which is what a feed's buttons act on.
//
// It is said to the viewer about themselves and to nobody else: the lookup
// is by the viewer's own id. A failure leaves the marks off and is logged;
// it does not fail the read.
func (h *mediumHandler) markSaved(viewerID int64, nodes []*entities.AnahitaNode) {
	markSaved(h.env, viewerID, nodes)
}

func markSaved(env *Env, viewerID int64, nodes []*entities.AnahitaNode) {
	if viewerID <= 0 || len(nodes) == 0 {
		return
	}

	target := func(node *entities.AnahitaNode) *entities.AnahitaNode {
		if node == nil {
			return nil
		}
		if node.Type == constants.NodeBaseRepostType {
			return node.Parent
		}
		return node
	}

	ids := []int64{}
	for _, node := range nodes {
		if post := target(node); post != nil && post.ID > 0 {
			ids = append(ids, post.ID)
		}
	}

	saved, err := env.Repos.Bookmarks.IsSavedMany(viewerID, ids)
	if err != nil {
		log.Printf("reading what %d saved failed: %v", viewerID, err)
		return
	}

	for _, node := range nodes {
		if post := target(node); post != nil {
			post.IsSavedByViewer = saved[post.ID]
		}
	}
}

// SetSaved saves a post for the viewer, or takes it out of what they saved.
// That the viewer may see the post is the calling service's to check.
func (h *mediumHandler) SetSaved(ctx context.Context, req *graphpb.SavedSetRequest) (*graphpb.SavedSetResponse, error) {
	if req.GetViewerId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "somebody signed in is required")
	}

	node, err := h.env.Repos.Nodes.Read(req.GetNodeId())
	if err != nil {
		return nil, err
	}
	if !node.IsMedium() {
		return nil, status.Error(codes.InvalidArgument, "only a post can be saved")
	}

	if err := h.env.Repos.Bookmarks.Set(req.GetViewerId(), node, req.GetSaved()); err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.SavedSetResponse{Success: true}, nil
}

// ListSaved returns what the viewer saved, most recently saved first, as
// they may see it now.
func (h *mediumHandler) ListSaved(ctx context.Context, req *graphpb.SavedListRequest) (*graphpb.SavedListResponse, error) {
	viewer := entities.NewViewer()
	if req.Viewer != nil {
		viewer.ID = req.Viewer.Id
		viewer.PersonType = req.Viewer.PersonType
		viewer.Username = req.Viewer.Username
		viewer.Email = req.Viewer.Email
	}

	nodes, err := h.env.Repos.Bookmarks.List(viewer, req.GetLimit(), req.GetOffset())
	if err != nil {
		log.Println(err)
		return nil, err
	}

	total, err := h.env.Repos.Bookmarks.Count(viewer)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	ids := extractNodeIDs(nodes)
	if len(ids) > 0 {
		isLikedMap, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, ids)
		if err != nil {
			return nil, err
		}
		isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, ids)
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			node.IsLikedByViewer = isLikedMap[node.ID].IsLiked
			node.IsRepostedByViewer = isRepostedMap[node.ID]
			// Everything in this list is saved: that is what the list is.
			node.IsSavedByViewer = true
		}
	}

	return &graphpb.SavedListResponse{
		Nodes: nodesToGrpc(nodes),
		Total: total,
	}, nil
}

// ListCitedBy returns the posts that cite a post, newest first, as the
// viewer may see them: the articles and topics that show it within their
// text (an edge.medium.citation.v1 from each to it). Its author's own are
// among them, though they do not count toward its citation_count.
func (h *mediumHandler) ListCitedBy(ctx context.Context, req *graphpb.CitedByListRequest) (*graphpb.CitedByListResponse, error) {
	viewer := entities.NewViewer()
	if req.Viewer != nil {
		viewer.ID = req.Viewer.Id
		viewer.PersonType = req.Viewer.PersonType
		viewer.Username = req.Viewer.Username
		viewer.Email = req.Viewer.Email
	}

	browse := &graphpb.BrowseNeighboursRequest{
		NodeId:    req.GetCitedId(),
		EdgeTypes: []string{constants.EdgeMediumCitationType},
		Direction: constants.GraphQueryDirectionInbound,
		Limit:     req.GetLimit(),
		Offset:    req.GetOffset(),
		Ordering:  constants.OrderingRecent,
	}
	nodes, err := h.env.Repos.Graph.BrowseNeighbours(browse, viewer)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	total, err := h.env.Repos.Graph.CountNeighbours(browse, viewer)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	ids := extractNodeIDs(nodes)
	if len(ids) > 0 && viewer.ID > 0 {
		isLikedMap, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, ids)
		if err != nil {
			return nil, err
		}
		isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, ids)
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			node.IsLikedByViewer = isLikedMap[node.ID].IsLiked
			node.IsRepostedByViewer = isRepostedMap[node.ID]
		}
	}

	return &graphpb.CitedByListResponse{
		Nodes: nodesToGrpc(nodes),
		Total: total,
	}, nil
}

// ListAuthored returns what one person wrote, a page at a time, for their
// own copy of it. It asks nothing about who is asking: the services that
// call it do so for the person themselves. See the repository.
func (h *mediumHandler) ListAuthored(ctx context.Context, req *graphpb.AuthoredListRequest) (*graphpb.AuthoredListResponse, error) {
	nodes, err := h.env.Repos.Nodes.Authored(req.GetPersonId(), req.GetAfterId(), req.GetLimit())
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.AuthoredListResponse{Nodes: nodesToGrpc(nodes)}, nil
}
