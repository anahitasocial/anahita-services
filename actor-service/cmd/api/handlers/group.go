package handlers

import (
	"actor/constants"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"anahita-libs/validators"
	"log"
	"net/http"
	"time"

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
		Data       []any                `json:"data"`
		Pagination responses.Pagination `json:"pagination"`
	}
	resp := &Response{Data: []any{}}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.Types = []string{constants.NodeActorGroupType}
	gReq.Offset = req.Offset
	gReq.Limit = req.Limit
	gReq.Search = req.Search
	gReq.Ordering = req.Ordering
	gReq.Direction = req.Direction
	gReq.Disabled = req.Disabled
	gReq.Featured = req.Featured
	gReq.IncludeAskable = listsAskable(viewer, req.Featured, req.Disabled)
	gReq.Ordering = req.GraphOrdering(c.QueryParam("ordering") != "")
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
		if node.Restricted {
			continue
		}
		if err = h.onAfterFetched(c, node); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	batchPresignPaths(h.Env, nodes)

	// Each a group, or what is shown of one the viewer may only ask to
	// follow (LimitedActor, marked restricted).
	actorResp := &responses.ActorResponse{}
	for _, node := range nodes {
		if node.Restricted {
			resp.Data = append(resp.Data, responses.NewLimitedActor(node, node.IsFollowRequestedByViewer))
			continue
		}
		resp.Data = append(resp.Data, actorResp.MapNodeItem(node))
	}

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
		// As for a person: a group that lets people ask shows its name
		// and picture to somebody it would otherwise refuse.
		if limited := limitedProfile(h.Env, actor, actorAccessCtx); limited != nil {
			return c.JSON(http.StatusOK, limited)
		}
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
// Archive marks a group archived. Permanent, and destroys nothing.
//
//	POST /groups/:id/archive
//
// For a group whose project has ended. Its posts stay readable at their URLs
// forever; what stops is the group being live — it leaves feeds, suggestions
// AND search, which a person's archive does not. That difference is the whole
// reason the search rule is type-aware: a finished group should stop competing
// with live spaces, while a memorial must stay findable.
//
// A group administrator may do this, unlike the hard delete. It destroys
// nothing, so the group's own administrators are the right people to decide it
// is finished — and unlike deletion there is no thirty-day window to change
// their mind in, which is what the step-up and the confirmation are for.
func (h *groupHandler) Archive(c echo.Context) error {
	var (
		req    = requests.NewActorRequest().Delete
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
	if ok, err := h.Perms.Groups.CanDelete(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("group archive:", err)
		return stepUpRefusal(c, err)
	}

	archived, err := h.Srv.ActorsGRPC.Archive(actor.ID, viewer)
	if err != nil {
		log.Println("group archive:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.clearStepUp(viewer.ID)

	if !archived {
		return c.NoContent(http.StatusNoContent)
	}

	return c.NoContent(http.StatusOK)
}

// SetEnabled turns a group on or off. Reversible.
//
//	PATCH /groups/:id/enabled
//
// Administrators only — site administrators, not the group's. Disabling is a
// moderation action taken against a group; archiving is what its own
// administrators reach for when it is finished.
func (h *groupHandler) SetEnabled(c echo.Context) error {
	// ONE bind, into one struct carrying both the path parameter and the body.
	//
	// echo's Bind consumes the request body, so calling it twice leaves the
	// second call reading nothing — the path id arrived and `enabled` was
	// always nil, which this handler then reported as a malformed request. It
	// refused every call, including its own correct ones.
	var req struct {
		ID      int64 `param:"id"      validate:"required,numeric"`
		Enabled *bool `json:"enabled"  validate:"required"`
	}

	viewer := getViewer(c)

	if err := c.Bind(&req); err != nil {
		log.Println("group set-enabled bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if req.ID == 0 || req.Enabled == nil {
		log.Println("group set-enabled: missing id or enabled field")
		return c.NoContent(http.StatusBadRequest)
	}

	if viewer == nil || !viewer.IsAdmin() {
		log.Println("group set-enabled: refused, viewer is not an administrator")
		return c.NoContent(http.StatusForbidden)
	}

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	changed, err := h.Srv.ActorsGRPC.SetEnabled(actor.ID, *req.Enabled, viewer)
	if err != nil {
		log.Println("group set-enabled:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !changed {
		return c.NoContent(http.StatusNoContent)
	}

	return c.NoContent(http.StatusOK)
}

// SetFeatured features or unfeatures a group: the accounts onboarding offers
// a new member to follow.
//
//	PATCH /groups/:id/featured   { "featured": true | false }
//
// SUPER ADMINISTRATORS ONLY. Featuring is a statement by the installation, not
// by one group's administrators — who would otherwise all feature their own.
//
// 200: changed
// 204: already in that state
// 409: featuring a group that is disabled, archived or deleted
func (h *groupHandler) SetFeatured(c echo.Context) error {
	// One bind, for the reason SetEnabled gives.
	var req struct {
		ID       int64 `param:"id"       validate:"required,numeric"`
		Featured *bool `json:"featured"  validate:"required"`
	}

	viewer := getViewer(c)

	if err := c.Bind(&req); err != nil {
		log.Println("group set-featured bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if req.ID == 0 || req.Featured == nil {
		log.Println("group set-featured: missing id or featured field")
		return c.NoContent(http.StatusBadRequest)
	}

	if viewer == nil || !viewer.IsSuperAdmin() {
		log.Println("group set-featured: refused, viewer is not a super administrator")
		return c.NoContent(http.StatusForbidden)
	}

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	changed, err := h.Srv.ActorsGRPC.SetFeatured(actor.ID, *req.Featured, viewer)
	if err != nil {
		if services.IsNotFeaturable(err) {
			return refuseNotFeaturable(c)
		}
		log.Println("group set-featured:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !changed {
		return c.NoContent(http.StatusNoContent)
	}

	return c.NoContent(http.StatusOK)
}

// DeletionCounts reports what deleting this group would cost.
//
//	GET /groups/:id/deletion-counts
//
// Shown in the Danger zone before the confirmation. A group holds other
// people's content — that is what a group IS — so the figures matter more here
// than on a personal profile: the person clicking is spending posts and
// discussions somebody else wrote.
//
// Gated on the same permission as the deletion, not a weaker read permission.
// If you may not delete the group, you may not enumerate what is in it.
//
// No step-up. This is a read that changes nothing, and requiring a proof to
// see the consequences before deciding would put the friction in the wrong
// place — somebody would authenticate to find out they did not want to.
func (h *groupHandler) DeletionCounts(c echo.Context) error {
	var (
		req    = requests.NewActorRequest().Delete
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
	if ok, err := h.Perms.Groups.CanDelete(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	counts, err := h.Srv.ActorsGRPC.ReadDeletionCounts(actor.ID, viewer)
	if err != nil || counts == nil {
		log.Println("deletion counts:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	type Response struct {
		Posts       int64  `json:"posts"`
		Replies     int64  `json:"replies"`
		Followers   int64  `json:"followers"`
		Admins      int64  `json:"admins"`
		MemberSince string `json:"member_since,omitempty"`
	}

	// "Following" and "groups administered" are omitted rather than sent as
	// zero: a group follows nobody and administers nothing, so the fields would
	// be permanently zero and the dialog would have to know to hide them.
	// Better that the shape says what a group has.
	resp := &Response{
		Posts:     counts.Posts,
		Replies:   counts.Replies,
		Followers: counts.Followers,
		Admins:    counts.Admins,
	}

	if counts.MemberSinceUnix > 0 {
		resp.MemberSince = time.Unix(counts.MemberSinceUnix, 0).UTC().Format(time.RFC3339)
	}

	return c.JSON(http.StatusOK, resp)
}

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

	rel := actorAccessCtx.ActorViewerRel
	item.IsLeadingViewer = rel.IsLeading
	item.IsFollowingViewer = rel.IsFollowing
	// Whether the viewer administers the group. This was set to true for
	// everybody, so the web app treated every viewer as an administrator of
	// every group it showed.
	item.IsAdminedByViewer = rel.HasAdmin
	item.Authorized.Edit, _ = h.Perms.Groups.CanEdit(actorAccessCtx)
	item.Authorized.Delete, _ = h.Perms.Groups.CanDelete(actorAccessCtx)
	item.Authorized.Administration, _ = h.Perms.Groups.CanAdministrate(actorAccessCtx)
	item.Administrators, _ = h.Srv.ActorAdminsGRPC.Browse(item.ID)

	// What the viewer may add, from the checks the services enforce, so the
	// web app offers only the composers and the add-follower control that
	// will work.
	item.Authorized.Composers = permissions.Composers(viewer, item, rel)
	item.Authorized.Audiences = permissions.PostAudiences(viewer, item, rel)
	addFollower := permissions.CanAddOthersAsFollowers(viewer, item, rel)
	item.Authorized.AddFollower = &addFollower
	invite := permissions.CanInvite(viewer, item, rel)
	item.Authorized.Invite = &invite
	// And whether the viewer has themselves been invited, to answer here.
	item.IsInvitedViewer = rel.HasInvite

	return nil
}
