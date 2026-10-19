package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/proto/graphpb"
	"context"
	"fmt"
	"graph/data/validators"
	"graph/requests"
	"log"
	"strings"
)

type mediumHandler struct {
	graphpb.MediumServiceServer
	env *Env
}

func NewMediumHandler(env *Env) *mediumHandler {
	return &mediumHandler{env: env}
}

// Count return the total number of media
func (h *mediumHandler) Count(ctx context.Context, req *graphpb.MediumCountRequest) (*graphpb.MediumCountResponse, error) {
	log.Printf("Received Count request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().Count
	rReq.Viewer = viewer
	if req.OwnerId > 0 {
		rReq.OwnerIDs = []int64{req.OwnerId}
	}
	if len(req.Types) > 0 {
		rReq.Types = req.Types
	}
	rReq.Search = req.Search
	rReq.AccessCheck = req.AccessCheck
	rReq.TopLevelOnly = !req.IncludeReplies
	rReq.Library = req.Library
	rReq.Drafts = req.Drafts

	total, err := h.env.Repos.Nodes.Count(rReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.MediumCountResponse{
		Total: total,
	}, nil
}

// Browse return a list of media
func (h *mediumHandler) Browse(ctx context.Context, req *graphpb.MediumBrowseRequest) (*graphpb.MediumBrowseResponse, error) {
	log.Printf("Received Browse request: %v", req)
	var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().Browse
	if req.OwnerId > 0 {
		rReq.OwnerIDs = []int64{req.OwnerId}
	}
	if len(req.Types) > 0 {
		rReq.Types = req.Types
	}
	rReq.Search = req.Search
	rReq.Offset = req.Offset
	rReq.Limit = req.Limit
	rReq.Ordering = req.Ordering
	rReq.OrderingDir = req.Direction
	rReq.AccessCheck = req.AccessCheck
	rReq.TopLevelOnly = !req.IncludeReplies
	rReq.Library = req.Library
	rReq.Drafts = req.Drafts
	rReq.Viewer = viewer

	nodes, err := h.env.Repos.Nodes.Browse(rReq)
	if err != nil {
		return nil, err
	}

	targetIds := extractNodeIDs(nodes)
	subscriptionMap, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	isLikedMap, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		node.IsSubscribedByViewer = subscriptionMap[node.ID].IsSubscribed
		node.IsLikedByViewer = isLikedMap[node.ID].IsLiked
		node.IsRepostedByViewer = isRepostedMap[node.ID]
	}

	h.markSaved(viewer.ID, nodes)

	return &graphpb.MediumBrowseResponse{
		Media: nodesToGrpc(nodes),
	}, nil
}

// Read return a single medium
func (h *mediumHandler) Read(ctx context.Context, req *graphpb.MediumReadRequest) (*graphpb.MediumReadResponse, error) {
	log.Printf("Received Read request: %v", req.Id)
	// Media only, for the same reason the actor read is scoped: /photos/ and
	// /topics/ both answered 200 with a hashtag for a hashtag's id.
	node, err := h.env.Repos.Nodes.ReadOfType(req.Id, constants.NodeMediumPrefixEntityType)
	if err != nil {
		return nil, err
	}

	node.IsSubscribedByViewer, err = h.env.Repos.SubscriptionsRel.IsSubscribedToOne(req.Viewer.Id, node.ID)
	if err != nil {
		return nil, err
	}

	node.IsLikedByViewer, err = h.env.Repos.LikesRel.IsLikedOne(req.Viewer.Id, node.ID)
	if err != nil {
		return nil, err
	}

	isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(req.Viewer.Id, []int64{node.ID})
	if err != nil {
		return nil, err
	}
	node.IsRepostedByViewer = isRepostedMap[node.ID]

	// log.Println("Medium subscription status:", medium.IsSubscribedByViewer)
	// log.Println("Medium like status:", node.IsLikedByViewer)
	// log.Println("Medium repost status:", node.IsRepostedByViewer)

	h.markSaved(req.Viewer.Id, []*entities.AnahitaNode{node})

	return &graphpb.MediumReadResponse{
		Medium: nodeToGrpc(node),
	}, nil
}

// BatchRead return a list of media by their IDs
func (h *mediumHandler) BatchRead(ctx context.Context, req *graphpb.MediumBatchReadRequest) (*graphpb.MediumBatchReadResponse, error) {
	log.Printf("Received BatchRead request: %v", req.Ids)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().BatchRead
	rReq.IDs = req.Ids
	rReq.Ordering = req.Ordering
	rReq.OrderingDir = req.Direction

	nodes, err := h.env.Repos.Nodes.BatchRead(rReq)
	if err != nil {
		return nil, err
	}

	targetIds := extractNodeIDs(nodes)
	subscriptionMap, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(req.Viewer.Id, targetIds)
	if err != nil {
		return nil, err
	}

	isLikedMap, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		node.IsSubscribedByViewer = subscriptionMap[node.ID].IsSubscribed
		node.IsRepostedByViewer = isRepostedMap[node.ID]
		node.IsLikedByViewer = isLikedMap[node.ID].IsLiked
	}

	h.markSaved(viewer.ID, nodes)

	return &graphpb.MediumBatchReadResponse{
		Media: nodesToGrpc(nodes),
	}, nil
}

// Edit update a medium
func (h *mediumHandler) Edit(ctx context.Context, req *graphpb.MediumEditRequest) (*graphpb.MediumEditResponse, error) {
	log.Printf("Received Edit request: %v", req)

	node, err := h.env.Repos.Nodes.Read(req.Medium.Id)
	if err != nil {
		return nil, err
	}

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	// Whether this edit publishes a draft: it then becomes a post from now,
	// and is announced as one made now.
	wasDraft := node.Draft

	applyEditableMediumFields(node, req.Medium)

	v := validators.NewMediumValidator()
	if err := v.ValidateEdit(node); err != nil {
		return nil, err
	}

	node, err = h.env.Repos.Nodes.Edit(node)
	if err != nil {
		return nil, err
	}

	published := wasDraft && !node.Draft
	if published {
		if err := h.env.Repos.Nodes.Published(node.ID); err != nil {
			return nil, err
		}
		if node, err = h.env.Repos.Nodes.Read(node.ID); err != nil {
			return nil, err
		}
	}

	// What it cites counts it as a citer, or no more, as its audience and
	// whether it is published say.
	h.recountCitedBy(node.ID)

	node.IsSubscribedByViewer, err = h.env.Repos.SubscriptionsRel.IsSubscribedToOne(viewer.ID, node.ID)
	if err != nil {
		return nil, err
	}

	node.IsLikedByViewer, err = h.env.Repos.LikesRel.IsLikedOne(viewer.ID, node.ID)
	if err != nil {
		return nil, err
	}

	isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, []int64{node.ID})
	if err != nil {
		return nil, err
	}
	node.IsRepostedByViewer = isRepostedMap[node.ID]

	// publish event. Not for a post in its author's media library only:
	// what listens is what announces a post, to other servers, to the
	// owner's subscribers, to a hashtag or a mention, and such a post is
	// announced to nobody (repositories/listed.go).
	//
	// Nor for a draft, which nobody else sees; and a draft just published
	// is announced as a post made now, as if it had been added, so that
	// what tells of a new post tells of it (the user, 2026-10-16).
	go func() {
		if node.LibraryOnly || node.Draft {
			return
		}
		entityName := strings.Split(node.Type, ".")[3]
		srvName := strings.Split(node.Service, "-")[0]
		format := event.EventFormatNodeMediumEdited
		if published {
			format = event.EventFormatNodeMediumAdded
		}
		routing_key := fmt.Sprintf(format, srvName, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    node.Service,
			SubjectID:        viewer.ID,
			SubjectType:      constants.NodeActorPersonEntityType,
			ObjectID:         node.ID,
			ObjectType:       node.Type,
			TargetID:         node.Owner.ID,
			TargetType:       node.Owner.Type,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		log.Println("Publishing event", routing_key)
		if err = h.env.Publisher.Publish(payload, routing_key); err != nil {
			log.Println(err)
		}
	}()

	return &graphpb.MediumEditResponse{
		Medium: nodeToGrpc(node),
	}, nil
}

// Add create a new medium
func (h *mediumHandler) Add(ctx context.Context, req *graphpb.MediumAddRequest) (*graphpb.MediumAddResponse, error) {
	log.Printf("Received Add request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	node := newMediumFromRequest(req.Medium)

	// A reply. Where it sits in its thread is worked out here from what it
	// answers, and never taken from the request: the root is what decides
	// who may see the reply, so it is not for a caller to name.
	if req.Medium.ParentId > 0 {
		parent, err := h.env.Repos.Nodes.Read(req.Medium.ParentId)
		if err != nil {
			return nil, err
		}
		if err := placeInThread(node, parent); err != nil {
			return nil, err
		}
	}

	v := validators.NewMediumValidator()
	if err := v.ValidateAdd(node); err != nil {
		return nil, err
	}

	node, err := h.env.Repos.Nodes.Add(node)
	if err != nil {
		return nil, err
	}

	// Its locations, before it is announced below.
	if len(req.LocationIds) > 0 {
		h.tagLocations(node, req.LocationIds, viewer.ID)
	}

	if node.RootID > 0 {
		if err := h.env.Repos.Replies.Recount(node.RootID, node.ParentID); err != nil {
			log.Printf("counting the replies of %d failed: %v", node.RootID, err)
		}
	}

	// A note that quotes a post is one more quote of it.
	if node.QuoteID > 0 {
		if err := h.env.Repos.Quotes.Recount(node.QuoteID); err != nil {
			log.Printf("counting the quotes of %d failed: %v", node.QuoteID, err)
		}
	}

	// publish event, as on an edit: not for a post in a media library only,
	// nor for a draft, which is announced when it is published.
	go h.announceAdded(node, viewer, 0)

	return &graphpb.MediumAddResponse{
		Medium: nodeToGrpc(node),
	}, nil
}

// announceAdded says a post was made: not one in a media library only,
// nor a draft, which is announced when it is published. `part` is where it
// is in a thread written in one go, or 0.
func (h *mediumHandler) announceAdded(node *entities.AnahitaNode, viewer *entities.Viewer, part int) {
	if node.LibraryOnly || node.Draft {
		return
	}
	entityName := strings.Split(node.Type, ".")[3]
	srvName := strings.Split(node.Service, "-")[0]
	routing_key := fmt.Sprintf(event.EventFormatNodeMediumAdded, srvName, entityName)
	payload := &event.EventPayload{
		Name:             routing_key,
		SourceService:    node.Service,
		SubjectID:        viewer.ID,
		SubjectType:      constants.NodeActorPersonEntityType,
		ObjectID:         node.ID,
		ObjectType:       node.Type,
		TargetID:         node.Owner.ID,
		TargetType:       node.Owner.Type,
		ViewerID:         viewer.ID,
		ViewerPersonType: viewer.PersonType,
		Part:             part,
	}
	h.env.Publisher.Publish(payload, routing_key)
}

// Delete remove a medium
func (h *mediumHandler) Delete(ctx context.Context, req *graphpb.MediumDeleteRequest) (*graphpb.MediumDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req.Id)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	node, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	// A reply that has been answered is emptied and kept, so what was said
	// under it still hangs from something. Anything else goes, with
	// everything under it: a post takes its whole thread.
	answered := false
	if node.RootID > 0 {
		if answered, err = h.env.Repos.Replies.HasReplies(node.ID); err != nil {
			return nil, err
		}
	}

	// What it cites, read while its edges are still there: they go with it.
	cited := h.citedBy(node.ID)

	if answered {
		err = h.env.Repos.Replies.Tombstone(node.ID, viewer.ID)
	} else {
		_, err = h.env.Repos.Nodes.DeleteSubtree(node.ID)
	}
	if err != nil {
		return nil, err
	}

	if node.RootID > 0 {
		if err := h.env.Repos.Replies.Recount(node.RootID, node.ParentID); err != nil {
			log.Printf("counting the replies of %d failed: %v", node.RootID, err)
		}
	}

	// One quote fewer of what it quoted.
	if node.QuoteID > 0 {
		if err := h.env.Repos.Quotes.Recount(node.QuoteID); err != nil {
			log.Printf("counting the quotes of %d failed: %v", node.QuoteID, err)
		}
	}

	// And one citer fewer, perhaps, of what it cited.
	h.recount(cited)

	// The list of a photo post's images goes with the post. The stored images
	// themselves are removed by the service that asked for this.
	if err := h.env.Repos.PhotoFiles.DeleteForNodes([]int64{node.ID}); err != nil {
		log.Printf("deleting the image list of %d failed: %v", node.ID, err)
	}

	// publish event
	go func() {
		entityName := strings.Split(node.Type, ".")[3]
		srvName := strings.Split(node.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatNodeMediumDeleted, srvName, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    node.Service,
			SubjectID:        viewer.ID,
			SubjectType:      constants.NodeActorPersonEntityType,
			ObjectID:         node.ID,
			ObjectType:       node.Type,
			TargetID:         node.Owner.ID,
			TargetType:       node.Owner.Type,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.env.Publisher.Publish(payload, routing_key)
	}()

	return &graphpb.MediumDeleteResponse{}, nil
}

// recountCitedBy puts right the citation counts of what a post cites, after
// a change to the post: published, its audience changed, taken down. Never
// in the way of the change itself.
func (h *mediumHandler) recountCitedBy(citingID int64) {
	h.recount(h.citedBy(citingID))
}

// citedBy is what a post cites, or nothing when it cannot be read.
func (h *mediumHandler) citedBy(citingID int64) []int64 {
	if h.env.Repos.Citations == nil {
		return nil
	}
	cited, err := h.env.Repos.Citations.CitedBy(citingID)
	if err != nil {
		log.Printf("reading what %d cites failed: %v", citingID, err)
		return nil
	}
	return cited
}

// recount puts right the citation counts of what was cited.
func (h *mediumHandler) recount(cited []int64) {
	if h.env.Repos.Citations == nil || len(cited) == 0 {
		return
	}
	if err := h.env.Repos.Citations.Recount(cited); err != nil {
		log.Printf("counting the citations of %v failed: %v", cited, err)
	}
}
