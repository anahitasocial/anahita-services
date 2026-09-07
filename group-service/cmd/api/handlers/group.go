package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"anahita-libs/validators"
	"group/constants"
	"log"
	"net/http"

	"github.com/gosimple/slug"
	"github.com/labstack/echo/v4"

	an_constants "anahita-libs/constants"
)

type groupHandler struct {
	*Env
}

// NewUserHandler creates a new user handler
func NewGroupHandler(env *Env) *groupHandler {
	return &groupHandler{
		env,
	}
}

// Browse returns a list of group
func (h *groupHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewActorRequest().Browse
		gReq   = services.NewActorGRPCRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Groups.CanBrowse(viewer); !ok {
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
		Data       []*responses.ActorNodeItem `json:"data"`
		Pagination responses.Pagination       `json:"pagination"`
	}
	resp := &Response{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.Types = []string{constants.NodeActorGroupType}
	gReq.Offset = req.Offset
	gReq.Limit = req.Limit
	gReq.Search = req.Search
	gReq.Ordering = req.Ordering
	gReq.Direction = req.Direction
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

	nodes, err := h.Srv.ActorsGRPC.Browse(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	for _, node := range nodes {
		if err = h.onAfterFetched(c, node); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	batchPresignPaths(h.Env, nodes)

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Read returns a group
func (h *groupHandler) Read(c echo.Context) error {
	var (
		req    = requests.NewActorRequest().Read
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanRead(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err = h.onAfterFetched(c, actor); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{actor})
	batchPresignPaths(h.Env, actor.Administrators)

	actorResp := &responses.ActorResponse{}
	resp := actorResp.MapNode(actor)

	return c.JSON(http.StatusOK, resp)
}

// Edit updates a group's details
func (h *groupHandler) Edit(c echo.Context) error {
	var (
		req    = requests.NewActorRequest().Edit
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Normalise before validating: people type "example.com", and http_url
	// requires a scheme. Doing it afterwards would mean the scheme rule
	// never saw the value it exists to check.
	req.WebsiteURL = validators.NormalizeWebsiteURL(req.WebsiteURL)

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanEdit(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	actor.Name = req.Name
	actor.Alias = slug.Make(req.Name)
	actor.Body = req.Body
	actor.WebsiteURL = req.WebsiteURL
	actor.Enabled = req.Enabled
	actor.ModifiedBy = viewer.ID
	actor, err = h.Srv.ActorsGRPC.Edit(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actor, err = h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err = h.onAfterFetched(c, actor); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{actor})
	batchPresignPaths(h.Env, actor.Administrators)

	actorResp := &responses.ActorResponse{}
	resp := actorResp.MapNode(actor)

	return c.JSON(http.StatusOK, resp)
}

// Add adds a group
func (h *groupHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewActorRequest().Add
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Normalise before validating: people type "example.com", and http_url
	// requires a scheme. Doing it afterwards would mean the scheme rule
	// never saw the value it exists to check.
	req.WebsiteURL = validators.NormalizeWebsiteURL(req.WebsiteURL)

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Create the group entity
	actor := entities.NewAnahitaNode()
	actor.Access = an_constants.AnahitaAccessAdmins
	actor.Type = constants.NodeActorGroupType
	actor.Service = constants.GroupService
	actor.Name = req.Name
	actor.Alias = slug.Make(req.Name)
	actor.Body = req.Body
	actor.WebsiteURL = req.WebsiteURL
	actor.Enabled = req.Enabled
	actor.CreatedBy = viewer.ID
	actor.ModifiedBy = viewer.ID
	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanAdd(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	actor, err = h.Srv.ActorsGRPC.Add(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Add the viewer as the group admin
	admin, err := h.Srv.ActorsGRPC.ReadOfType(viewer.ID, an_constants.NodeActorPersonEntityType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actor, err = h.Srv.ActorsGRPC.ReadOfType(actor.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	admin, err = h.Srv.ActorAdminsGRPC.Add(actor, admin, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Add the admin as a follower to the group
	followReq := services.NewSocialgraphFollowRequest().Add
	followReq.Source = actor
	followReq.Target = admin
	followReq.Viewer = viewer
	if err := h.Srv.SocialgraphFollowGRPC.Add(followReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, actor); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{actor})
	batchPresignPaths(h.Env, actor.Administrators)

	actorResp := &responses.ActorResponse{}
	resp := actorResp.MapNode(actor)

	return c.JSON(http.StatusCreated, resp)
}

// Restore reverses a soft delete, within the grace period.
//
//	POST /groups/:id/restore
//
// A person restores their own account by signing in — the password is the
// proof and the login page offers it. A group cannot sign in, so its
// administrators restore it here instead, from the group's own settings page,
// which stays reachable while the group is only marked deleted.
//
// Gated on CanDelete, the same permission as deleting: undoing an action should
// never require more authority than doing it did.
//
// Status codes:
//
//	200 — restored
//	400 — bad id
//	403 — not permitted
//	404 — no such group, or the grace period has already run out
//	500 — graph failure
func (h *groupHandler) Restore(c echo.Context) error {
	var (
		err    error
		req    = requests.NewActorRequest().Delete
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Groups.CanDelete(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	restored, err := h.Srv.ActorsGRPC.Restore(actor.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Nothing to restore: the sweeper reached it first, or it was never
	// deleted. 404 rather than 200 — telling the caller a group is back when
	// it is gone for good is the one wrong answer here.
	if !restored {
		log.Printf("group restore: nothing to restore (group=%d)", actor.ID)
		return c.NoContent(http.StatusNotFound)
	}

	log.Printf("group restore: restored (group=%d by=%d)", actor.ID, viewer.ID)

	actorResp := &responses.ActorResponse{}

	return c.JSON(http.StatusOK, actorResp.MapNode(actor))
}

// Delete soft-deletes a group.
//
// The NOTE that stood here — "this should be a soft delete and we should not
// delete the group" — is now true: ActorsGRPC.Delete marks and disables rather
// than purging, and the sweeper does the destructive part once the grace period
// has expired.
func (h *groupHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewActorRequest().Delete
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Groups.CanDelete(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// Prove it, don't just be signed in.
	//
	// Deleting a group takes a shared space away from everybody in it, and
	// takes their posts with it when the grace period ends. That it belongs to
	// several people is the argument FOR the gate rather than against: the
	// person clicking is spending other people's content, not only their own.
	//
	// Keyed on the viewer — the administrator doing it is who has to prove it.
	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("group delete:", err)
		return stepUpRefusal(c, err)
	}

	if err = h.Srv.ActorsGRPC.Delete(actor.ID, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One proof authorises one deletion.
	h.clearStepUp(viewer.ID)

	return c.NoContent(http.StatusOK)
}

// onAfterFetched is called after a group is fetched
func (h *groupHandler) onAfterFetched(c echo.Context, item *entities.AnahitaNode) error {
	var (
		err    error
		viewer = getViewer(c)
	)

	actorAccessCtx, err := h.getActorAccessContext(item, viewer)
	if err != nil {
		return err
	}

	// Setting Features
	item.Features, err = item.GetFeatures()
	if err != nil {
		log.Println("Error fetching features for actor:", err)
	}

	item.IsLeadingViewer = actorAccessCtx.ActorViewerRel.IsLeading
	item.Authorized.Edit, _ = h.Perms.Groups.CanEdit(actorAccessCtx)
	item.Authorized.Delete, _ = h.Perms.Groups.CanDelete(actorAccessCtx)
	item.Authorized.Administration, _ = h.Perms.Groups.CanAdministrate(actorAccessCtx)
	item.Administrators, _ = h.Srv.ActorAdminsGRPC.Browse(item.ID)
	item.IsAdminedByViewer = true

	return nil
}
