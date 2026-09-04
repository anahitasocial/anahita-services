package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/requests"
	an_responses "anahita-libs/responses"
	"fmt"
	"log"
	"net/http"
	"person/responses"
	"time"

	"github.com/labstack/echo/v4"
)

type personHandler struct {
	*Env
}

// NewPersonHandler creates a new user handler
func NewPersonHandler(env *Env) *personHandler {
	return &personHandler{
		env,
	}
}

// Browse returns a list of person
func (h *personHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewPersonRequest().Browse
		gReq   = services.NewActorGRPCRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.People.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	type Response struct {
		People     []*responses.PersonNodeItem `json:"data"`
		Pagination an_responses.Pagination     `json:"pagination"`
	}
	resp := &Response{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.Types = []string{constants.NodeActorPersonEntityType}
	gReq.Limit = req.Limit
	gReq.Offset = req.Offset
	gReq.Ordering = req.Ordering
	gReq.Direction = req.Direction
	gReq.Search = req.Search
	gReq.Disabled = req.Disabled
	if err := c.Validate(gReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	resp.Pagination.Total, err = h.Srv.ActorsGRPC.Count(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	actors, err := h.Srv.ActorsGRPC.Browse(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if len(actors) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	actorIds := extractIdsFromActors(actors)
	people, err := h.Repos.People.Browse(actorIds)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, actors)

	actorsMap := mapActorNodesById(actors)
	for _, person := range people {
		person.AnahitaNode = *actorsMap[person.ID]
		if err := h.onAfterFetched(c, person); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		personResp := &responses.PersonResponse{}
		resp.People = append(resp.People, personResp.MapNodeItem(person))
	}

	return c.JSON(http.StatusOK, resp)
}

// Read returns a person
func (h *personHandler) Read(c echo.Context) error {
	var (
		req    = requests.NewPersonRequest().Read
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	person, err := h.Repos.People.ReadByUsername(req.Username)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actor, err := h.Srv.ActorsGRPC.Read(person.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	person.AnahitaNode = *actor

	actorAccessCtx, err := h.getActorAccessContext(person, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.People.CanRead(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.onAfterFetched(c, person); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{&person.AnahitaNode})

	personResp := &responses.PersonResponse{}
	resp := personResp.MapNode(person)

	return c.JSON(http.StatusOK, resp)
}

// Edit updates a person's details
func (h *personHandler) Edit(c echo.Context) error {
	var (
		req    = requests.NewPersonRequest().Edit
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	person, err := h.fetchPerson(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(person, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.People.CanEdit(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	person.GivenName = req.GivenName
	person.FamilyName = req.FamilyName
	person.Body = req.Body
	person.ModifiedBy = viewer.ID
	person.UpdateTime = time.Now()

	if req.Gender == constants.UserGenderMale ||
		req.Gender == constants.UserGenderFemale ||
		req.Gender == constants.UserGenderNeutral {
		person.Gender = req.Gender
	}

	if err = h.Repos.People.Edit(person); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	_, err = h.Srv.ActorsGRPC.Edit(&person.AnahitaNode, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	person, err = h.fetchPerson(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Repos.MentionRelations.Update(person, person.Mentions, viewer); err != nil {
		log.Println(err)
		return err
	}

	// publish event
	go func() {
		routing_key := fmt.Sprintf(event.EventFormatNodeActorEdited, "person", "person")
		payload := &event.EventPayload{
			Name:           routing_key,
			SubjectID:      viewer.ID,
			ObjectID:       person.ID,
			TargetID:       person.ID,
			ViewerID:       viewer.ID,
			ViewerUserType: viewer.UserType,
		}
		log.Println("Publishing event", routing_key)
		if err = h.Publisher.Publish(payload, routing_key); err != nil {
			log.Println(err)
		}
	}()

	person, err = h.fetchPerson(c, person.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{&person.AnahitaNode})

	personResp := &responses.PersonResponse{}
	resp := personResp.MapNode(person)

	return c.JSON(http.StatusOK, resp)
}

// Delete deletes a person
func (h *personHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewPersonRequest().Delete
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	person, err := h.Repos.People.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(person, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.People.CanDelete(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// ONE write, and a soft one.
	//
	// This used to delete the people_people row here and then call graph-grpc to
	// delete the node — two hard deletes across two services with no transaction
	// between them. A failure on the second leg left an account nobody could log
	// into and nobody could remove, because this handler finds the person
	// through the row that had already gone.
	//
	// The credentials row now stays put: it is what the person signs in with to
	// restore themselves during the grace period. The sweeper removes it when
	// the window closes.
	if err = h.Srv.ActorsGRPC.Delete(person.ID, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// publish event
	go func() {
		routing_key := fmt.Sprintf(event.EventFormatNodeActorDeleted, "person", "person")
		payload := &event.EventPayload{
			Name:           routing_key,
			SubjectID:      viewer.ID,
			ObjectID:       person.ID,
			TargetID:       person.ID,
			ViewerID:       viewer.ID,
			ViewerUserType: viewer.UserType,
		}
		log.Println("Publishing event", routing_key)
		if err = h.Publisher.Publish(payload, routing_key); err != nil {
			log.Println(err)
		}
	}()

	return c.NoContent(http.StatusOK)
}

// onAfterFetched sets the authorized fields for a person
func (h *personHandler) onAfterFetched(c echo.Context, node *entities.Person) error {
	var (
		err    error
		viewer = getViewer(c)
	)

	actorAccessCtx, err := h.getActorAccessContext(node, viewer)
	if err != nil {
		return err
	}

	// Setting Features
	node.Features, err = node.GetFeatures()
	if err != nil {
		log.Println("Error fetching features for actor:", err)
	}

	node.Authorized.Edit, _ = h.Perms.People.CanEdit(actorAccessCtx)
	node.Authorized.Delete, _ = h.Perms.People.CanDelete(actorAccessCtx)
	node.Authorized.Administration, _ = h.Perms.People.CanAdministrate(actorAccessCtx)

	// Socialgraph relationships with the viewer
	node.IsFollowingViewer = actorAccessCtx.ActorViewerRel.IsFollowing
	node.IsLeadingViewer = actorAccessCtx.ActorViewerRel.IsLeading
	node.IsBlockedByViewer = actorAccessCtx.ActorViewerRel.IsBlocking

	return nil
}

// fetchPerson fetches a person by ID and then assigns the actor node
func (h *personHandler) fetchPerson(c echo.Context, id int64) (*entities.Person, error) {
	viewer := getViewer(c)

	person, err := h.Repos.People.ReadByID(id)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	actor, err := h.Srv.ActorsGRPC.Read(person.ID, viewer)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	person.AnahitaNode = *actor

	if err := h.onAfterFetched(c, person); err != nil {
		return nil, err
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{&person.AnahitaNode})

	return person, nil
}
