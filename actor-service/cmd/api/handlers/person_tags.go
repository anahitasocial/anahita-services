package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libs_services "anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/permissions"
	libs_responses "anahita-libs/responses"
	"anahita-libs/videos"

	"github.com/labstack/echo/v4"
)

// People in a picture or a video (roadmap row 103). Not a mention, which
// says "this is for you", but "this person is in it": an edge from the
// person to the post (an_constants.EdgeTagPersonType), made by the post's
// author, waiting for the person's approval unless they chose otherwise
// (permissions.TagDecision). A tag waiting is seen by nobody but the two of
// them.

// TagPath names a person and a post they are, or are to be, in.
type TagPath struct {
	// The person's id.
	ID int64 `param:"id"`
	// The photo's or the video's id.
	PostID int64 `param:"post_id"`
}

// TaggedInPath names a photo or a video.
type TaggedInPath struct {
	// The photo's or the video's id.
	PostID int64 `param:"post_id"`
}

// TaggedPerson is one person in a post, as the post's list of them answers.
type TaggedPerson struct {
	// Who.
	Person *libs_responses.NodeItem `json:"person"`
	// Waiting for their approval: shown only to them and the post's author.
	Pending bool `json:"pending"`
}

// taggable is whether a post can have people in it: a photo or a video.
func taggable(post *entities.AnahitaNode) bool {
	return post != nil &&
		(post.Type == an_constants.NodeMediumPhotoEntityType || post.Type == videos.NodeType)
}

// authorOf is who wrote a post, and so who tags people in it.
func authorOf(post *entities.AnahitaNode) int64 {
	if post.CreatedBy > 0 {
		return post.CreatedBy
	}
	return post.Author.ID
}

// readTaggablePost reads a post the viewer may see that can have people in
// it, or answers why not and says so with false: c.NoContent's error is
// nil once it has written, so it cannot be the signal.
func (h *personHandler) readTaggablePost(c echo.Context, viewer *entities.Viewer) (*entities.AnahitaNode, bool) {
	id, err := strconv.ParseInt(c.Param("post_id"), 10, 64)
	if err != nil || id <= 0 {
		_ = c.NoContent(http.StatusNotFound)
		return nil, false
	}
	visible, err := h.Srv.GraphNodeGRPC.FilterVisible([]int64{id}, viewer)
	if err != nil {
		log.Println("tags: visibility:", err)
		_ = c.NoContent(http.StatusInternalServerError)
		return nil, false
	}
	if len(visible) == 0 {
		_ = c.NoContent(http.StatusNotFound)
		return nil, false
	}
	post, err := h.Srv.GraphNodeGRPC.Read(id)
	if err != nil || post == nil || !taggable(post) {
		_ = c.NoContent(http.StatusNotFound)
		return nil, false
	}
	return post, true
}

// postTags is the tag edges of a post, by the person tagged.
func (h *personHandler) postTags(post *entities.AnahitaNode) (map[int64]*entities.AnahitaEdge, error) {
	req := libs_services.NewGraphEdgeRequest().BatchReadByFilters
	req.Types = []string{an_constants.EdgeTagPersonType}
	req.NodeBID = post.ID
	req.Ordering = "created_at"
	req.OrderingDir = "asc"
	edges, err := h.Srv.GraphEdgeGRPC.BatchReadByFilters(req)
	if err != nil {
		return nil, err
	}
	tags := make(map[int64]*entities.AnahitaEdge, len(edges))
	for _, edge := range edges {
		tags[edge.NodeAID] = edge
	}
	return tags, nil
}

// publishTag tells whoever listens of a tag: requested (waiting), added
// (approved, or made at once) or deleted. The subject is the post's author,
// the object the post, the target the person.
func (h *personHandler) publishTag(format string, post *entities.AnahitaNode, personID int64, viewer *entities.Viewer) {
	routingKey := fmt.Sprintf(format, "actor-service")
	payload := &event.EventPayload{
		Name:             routingKey,
		SourceService:    "actor-service",
		SubjectID:        authorOf(post),
		ObjectID:         post.ID,
		ObjectType:       post.Type,
		TargetID:         personID,
		ViewerID:         viewer.ID,
		ViewerPersonType: viewer.PersonType,
	}
	if h.Publisher == nil {
		return
	}
	if err := h.Publisher.Publish(payload, routingKey); err != nil {
		log.Println("tags: publish:", err)
	}
}

// TaggedIn is who is in a post.
//
//	GET /people/tagged-in/:post_id
//
//	200  the people, in the order they were tagged; one waiting only for
//	     them and the post's author
//	404  there is no such photo or video the viewer can see
func (h *personHandler) TaggedIn(c echo.Context) error {
	viewer := getViewer(c)
	post, ok := h.readTaggablePost(c, viewer)
	if !ok {
		return nil
	}
	tags, err := h.postTags(post)
	if err != nil {
		log.Println("tags: read:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	author := authorOf(post)
	var ids []int64
	for personID, edge := range tags {
		if edge.Meta == an_constants.TagPending && viewer.ID != author && viewer.ID != personID {
			continue
		}
		ids = append(ids, personID)
	}

	list := libs_responses.Unpaged[TaggedPerson]{Data: []TaggedPerson{}}
	if len(ids) == 0 {
		return c.JSON(http.StatusOK, list)
	}
	req := libs_services.NewGraphNodeRequest().BatchRead
	req.IDs = ids
	req.Viewer = viewer
	people, err := h.Srv.GraphNodeGRPC.BatchRead(req)
	if err != nil {
		log.Println("tags: people:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	batchPresignPaths(h.Env, people)
	byID := mapActorNodesById(people)

	mapper := &libs_responses.NodeResponse{}
	for _, edge := range orderedTags(tags) {
		person, ok := byID[edge.NodeAID]
		if !ok {
			continue
		}
		list.Data = append(list.Data, TaggedPerson{
			Person:  mapper.MapNodeItem(person),
			Pending: edge.Meta == an_constants.TagPending,
		})
	}
	return c.JSON(http.StatusOK, list)
}

// orderedTags is a post's tags in the order they were made.
func orderedTags(tags map[int64]*entities.AnahitaEdge) []*entities.AnahitaEdge {
	list := make([]*entities.AnahitaEdge, 0, len(tags))
	for _, edge := range tags {
		list = append(list, edge)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// AddTag tags a person in a post, for the post's author.
//
//	POST /people/:id/tags/:post_id
//
//	201  tagged
//	202  tagged, waiting for their approval
//	403  the viewer did not write the post
//	404  no such person, or no such photo or video the viewer can see
//	409  `not_taggable`, the person does not allow it, or is not here;
//	     `too_many`, the post has as many people as it may
func (h *personHandler) AddTag(c echo.Context) error {
	viewer := getViewer(c)
	post, ok := h.readTaggablePost(c, viewer)
	if !ok {
		return nil
	}
	if authorOf(post) != viewer.ID {
		return c.NoContent(http.StatusForbidden)
	}
	personID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || personID <= 0 {
		return c.NoContent(http.StatusNotFound)
	}
	person, err := h.Srv.GraphNodeGRPC.Read(personID)
	if err != nil || person == nil {
		return c.NoContent(http.StatusNotFound)
	}

	tags, err := h.postTags(post)
	if err != nil {
		log.Println("tags: read:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if edge, ok := tags[person.ID]; ok {
		if edge.Meta == an_constants.TagPending {
			return c.NoContent(http.StatusAccepted)
		}
		return c.NoContent(http.StatusCreated)
	}
	if len(tags) >= an_constants.TagMaxPerPost {
		return c.JSON(http.StatusConflict, libs_responses.Refusal{Error: "too_many"})
	}

	rel, err := h.Srv.SocialgraphGRPC.Read(person.ID, viewer.ID)
	if err != nil {
		log.Println("tags: relation:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	allowed, pending := permissions.TagDecision(person, viewer, rel)
	if !allowed {
		return c.JSON(http.StatusConflict, libs_responses.Refusal{Error: "not_taggable"})
	}

	edge := entities.NewAnahitaEdge()
	edge.Type = an_constants.EdgeTagPersonType
	edge.NodeAID = person.ID
	edge.NodeAType = person.Type
	edge.NodeBID = post.ID
	edge.NodeBType = post.Type
	edge.CreatedBy = viewer.ID
	edge.Meta = an_constants.TagApproved
	if pending {
		edge.Meta = an_constants.TagPending
	}
	if _, err := h.Srv.GraphEdgeGRPC.Add(edge); err != nil {
		log.Println("tags: add:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if pending {
		h.publishTag(event.EventFormatEdgeTagRequested, post, person.ID, viewer)
		return c.NoContent(http.StatusAccepted)
	}
	h.publishTag(event.EventFormatEdgeTagAdded, post, person.ID, viewer)
	return c.NoContent(http.StatusCreated)
}

// ApproveTag is the person tagged saying yes.
//
//	PUT /people/:id/tags/:post_id
//
//	200  approved, or it was already
//	403  the viewer is not the person tagged
//	404  no such tag, or no such photo or video the viewer can see
func (h *personHandler) ApproveTag(c echo.Context) error {
	viewer := getViewer(c)
	personID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || personID != viewer.ID {
		return c.NoContent(http.StatusForbidden)
	}
	post, ok := h.readTaggablePost(c, viewer)
	if !ok {
		return nil
	}
	tags, err := h.postTags(post)
	if err != nil {
		log.Println("tags: read:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	edge, ok := tags[personID]
	if !ok {
		return c.NoContent(http.StatusNotFound)
	}
	if edge.Meta != an_constants.TagPending {
		return c.NoContent(http.StatusOK)
	}
	edge.Meta = an_constants.TagApproved
	edge.ModifiedBy = viewer.ID
	if _, err := h.Srv.GraphEdgeGRPC.Edit(edge); err != nil {
		log.Println("tags: approve:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	h.publishTag(event.EventFormatEdgeTagAdded, post, personID, viewer)
	return c.NoContent(http.StatusOK)
}

// RemoveTag takes a person out of a post: the person, declining or
// changing their mind, or the post's author.
//
//	DELETE /people/:id/tags/:post_id
//
//	204  removed, or there was nothing to remove
//	403  the viewer is neither the person nor the post's author
//	404  no such photo or video the viewer can see
func (h *personHandler) RemoveTag(c echo.Context) error {
	viewer := getViewer(c)
	personID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || personID <= 0 {
		return c.NoContent(http.StatusNotFound)
	}
	post, ok := h.readTaggablePost(c, viewer)
	if !ok {
		return nil
	}
	if viewer.ID != personID && viewer.ID != authorOf(post) {
		return c.NoContent(http.StatusForbidden)
	}
	tags, err := h.postTags(post)
	if err != nil {
		log.Println("tags: read:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	edge, ok := tags[personID]
	if !ok {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.Srv.GraphEdgeGRPC.Delete(edge.ID); err != nil {
		log.Println("tags: remove:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	h.publishTag(event.EventFormatEdgeTagDeleted, post, personID, viewer)
	return c.NoContent(http.StatusNoContent)
}
