package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/requests"
	an_responses "anahita-libs/responses"
	"anahita-libs/validators"
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(person.ID, constants.NodeActorPersonEntityType, viewer)
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

	// Normalise before validating: people type "example.com", and http_url
	// requires a scheme. Doing it afterwards would mean the scheme rule
	// never saw the value it exists to check.
	req.WebsiteURL = validators.NormalizeWebsiteURL(req.WebsiteURL)

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
	person.WebsiteURL = req.WebsiteURL
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
			Name:             routing_key,
			SubjectID:        viewer.ID,
			ObjectID:         person.ID,
			TargetID:         person.ID,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
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

	// Prove it, don't just be signed in.
	//
	// Deletion was the last credential-grade action gated on a session alone,
	// while password, email, username and 2FA-disable all went through
	// /reauth/*. A stolen laptop with an unlocked browser deleted a profile.
	//
	// Keyed on the VIEWER, not the person being deleted: the proof answers
	// "is the person making this request who they claim", and for an
	// administrator deleting somebody else it is the administrator who has to
	// prove it. The person being removed cannot be asked.
	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("person delete:", err)
		return stepUpRefusal(c, err)
	}

	// The floor: never zero super administrators.
	//
	// The failure this prevents is the last super administrator deleting
	// themselves, thirty days passing, the sweeper purging, and the site
	// having nobody who can administer it and no route to appoint one. Short
	// of writing to the database by hand, that is unrecoverable.
	//
	// Not expressed as "a super administrator cannot delete themselves". That
	// rule deadlocks an installation with exactly one — they could never
	// delete their account and nobody else could either — and it is heavier
	// than the risk anyway: with three, one leaving should not have to ask a
	// colleague.
	//
	// 409 rather than 403: the request is well formed and the caller is
	// permitted, it conflicts with the state of the site. The body names the
	// way out, because a refusal that does not is a support ticket.
	if person.PersonType == constants.PersonTypeSuperAdmin {
		others, err := h.Repos.People.CountOtherActiveSuperAdmins(person.ID)
		if err != nil {
			log.Println("person delete: super administrator count failed:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if others == 0 {
			log.Printf("person delete: refused, %d is the last super administrator",
				person.ID)
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "last_super_administrator",
				"message": "This is the only super administrator. Promote " +
					"someone else, or change this account to administrator, " +
					"and then delete it.",
			})
		}
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

	// One proof authorises one deletion, matching how the password and email
	// flows consume theirs. Deleting two profiles means proving twice.
	h.clearStepUp(viewer.ID)

	// No publish here. graph-grpc emits node.actor.person.person.deleted.v1
	// from ActorsGRPC.Delete above, which is where the state actually changes.
	//
	// This handler used to publish its own copy. Group deletion published
	// nothing, so the same action announced itself for one actor type and not
	// the other — and once both are emitted from the one place, a second
	// publish here would deliver every person deletion to consumers twice.

	return c.NoContent(http.StatusOK)
}

// Archive marks a profile archived. Permanent, and destroys nothing.
//
//	POST /people/:id/archive
//
// For somebody who has left the network, or died. The posts stay readable at
// their URLs forever; what stops is the account being live — it leaves feeds
// and suggestions, and it can no longer be signed in to.
//
// Two entry points reach this, and they are different situations wearing one
// state. A person archiving their own profile is leaving. A super
// administrator archiving somebody else's is memorialising an account whose
// owner cannot ask — which is the case that makes the administrative path
// necessary at all, since a dead person cannot click anything.
//
// Step-up gated, and permanent, so it is deliberately as heavy as deletion.
// There is no unarchive: see the RPC.
func (h *personHandler) Archive(c echo.Context) error {
	var (
		req    = requests.NewPersonRequest().Delete
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

	person, err := h.Repos.People.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// Self, or a super administrator. NOT CanDelete, which an ordinary
	// administrator satisfies: archiving cannot be undone, so widening it to
	// everyone who may schedule a recoverable deletion would be strictly
	// worse than the thing it sits beside.
	isSelf := viewer != nil && viewer.ID == person.ID
	isSuperAdmin := viewer != nil && viewer.PersonType == constants.PersonTypeSuperAdmin
	if !isSelf && !isSuperAdmin {
		log.Println("person archive: refused, neither the owner nor a super administrator")
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("person archive:", err)
		return stepUpRefusal(c, err)
	}

	// The same floor deletion has. An archived super administrator cannot sign
	// in, so archiving the last one locks the site out exactly as deleting
	// them would.
	if person.PersonType == constants.PersonTypeSuperAdmin {
		others, err := h.Repos.People.CountOtherActiveSuperAdmins(person.ID)
		if err != nil {
			log.Println("person archive: super administrator count failed:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if others == 0 {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "last_super_administrator",
				"message": "This is the only super administrator. Promote " +
					"someone else before archiving this account.",
			})
		}
	}

	archived, err := h.Srv.ActorsGRPC.Archive(person.ID, viewer)
	if err != nil {
		log.Println("person archive:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.clearStepUp(viewer.ID)

	if !archived {
		// Already archived, or scheduled for deletion. Nothing happened, and
		// the caller should not be told it did.
		return c.NoContent(http.StatusNoContent)
	}

	return c.NoContent(http.StatusOK)
}

// SetEnabled turns a profile on or off. Reversible.
//
//	PATCH /people/:id/enabled
//
// The counterpart to archiving, and the reason `enabled` now means one thing:
// suspension used to arrive only as a side effect of half-deleting somebody.
//
// Administrators, not the owner. Disabling your own profile is not a feature —
// it is a way to lock yourself out — and the archive path above is what
// somebody leaving actually wants.
func (h *personHandler) SetEnabled(c echo.Context) error {
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
		log.Println("person set-enabled bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if req.ID == 0 || req.Enabled == nil {
		log.Println("person set-enabled: missing id or enabled field")
		return c.NoContent(http.StatusBadRequest)
	}

	if viewer == nil || !viewer.IsAdmin() {
		log.Println("person set-enabled: refused, viewer is not an administrator")
		return c.NoContent(http.StatusForbidden)
	}

	person, err := h.Repos.People.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// Disabling the last super administrator locks the site out the same way
	// deleting or archiving them would — they cannot sign in to undo it.
	if !*req.Enabled && person.PersonType == constants.PersonTypeSuperAdmin {
		others, err := h.Repos.People.CountOtherActiveSuperAdmins(person.ID)
		if err != nil {
			log.Println("person set-enabled: super administrator count failed:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if others == 0 {
			return c.JSON(http.StatusConflict, map[string]string{
				"error":   "last_super_administrator",
				"message": "This is the only super administrator and cannot be disabled.",
			})
		}
	}

	changed, err := h.Srv.ActorsGRPC.SetEnabled(person.ID, *req.Enabled, viewer)
	if err != nil {
		log.Println("person set-enabled:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !changed {
		return c.NoContent(http.StatusNoContent)
	}

	return c.NoContent(http.StatusOK)
}

// Purge destroys a profile immediately, skipping the grace period.
//
//	DELETE /people/:id/purge
//
// The hard delete, for site cleanup — clearing out stale and abandoned
// accounts. Super administrators only, and unlike every other permission in
// this service that check is written here rather than in CanDelete: CanDelete
// answers "may this caller remove this profile", which an ordinary
// administrator and the owner both may. This asks something narrower, and
// conflating them would let either of them reach it.
//
// A separate route from DELETE /people/:id for the same reason the RPC is
// separate from Delete — the difference between "recoverable for thirty days"
// and "gone" should not be a parameter.
//
// Notifies nobody. See docs/plans: a super administrator clearing out an
// account is not a conversation, and the grace-period notification is for
// people who might still act on it.
func (h *personHandler) Purge(c echo.Context) error {
	var (
		req    = requests.NewPersonRequest().Delete
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

	// Super administrators only. Not IsAdmin — an ordinary administrator may
	// schedule a deletion, which is recoverable; this is not.
	if viewer == nil || viewer.PersonType != constants.PersonTypeSuperAdmin {
		log.Println("person purge: refused, viewer is not a super administrator")
		return c.NoContent(http.StatusForbidden)
	}

	// Prove it. The heaviest action in the system gets the same gate as the
	// reversible one, which is the floor rather than the ceiling.
	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("person purge:", err)
		return stepUpRefusal(c, err)
	}

	person, err := h.Repos.People.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// The same floor deletion has, and for the same reason — more so, since
	// there is no thirty-day window to notice the mistake in.
	if person.PersonType == constants.PersonTypeSuperAdmin {
		others, err := h.Repos.People.CountOtherActiveSuperAdmins(person.ID)
		if err != nil {
			log.Println("person purge: super administrator count failed:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if others == 0 {
			log.Printf("person purge: refused, %d is the last super administrator",
				person.ID)
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "last_super_administrator",
				"message": "This is the only super administrator. Promote " +
					"someone else before destroying this account.",
			})
		}
	}

	purged, err := h.Srv.ActorsGRPC.Purge(person.ID, viewer)
	if err != nil {
		log.Println("person purge:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.clearStepUp(viewer.ID)

	if !purged {
		// Nothing there to destroy. The caller wanted it gone and it is.
		return c.NoContent(http.StatusNoContent)
	}

	return c.NoContent(http.StatusOK)
}

// DeletionCounts reports what deleting this profile would cost.
//
//	GET /people/:id/deletion-counts
//
// Shown in the Danger zone before the confirmation, because "your posts will
// be removed" reads as boilerplate while "4,826 posts" does not. This is the
// gate that makes somebody stop, and it only works if it arrives before the
// decision rather than after it.
//
// Gated on the same permission as the deletion itself, not on a weaker read
// permission. The figures describe somebody's footprint on the site — how much
// they have written, who follows them, how long they have been here — and that
// is not a summary to hand to anyone who asks for it. If you may not delete
// this profile, you may not enumerate it either.
//
// No step-up, though. This is a read, it changes nothing, and requiring a
// proof to see the consequences before deciding would put the friction in the
// wrong place: somebody would have to authenticate to find out they did not
// want to do it.
func (h *personHandler) DeletionCounts(c echo.Context) error {
	var (
		req    = requests.NewPersonRequest().Delete
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

	counts, err := h.Srv.ActorsGRPC.ReadDeletionCounts(person.ID, viewer)
	if err != nil || counts == nil {
		log.Println("deletion counts:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	type Response struct {
		Posts              int64  `json:"posts"`
		Comments           int64  `json:"comments"`
		Followers          int64  `json:"followers"`
		Following          int64  `json:"following"`
		GroupsAdministered int64  `json:"groups_administered"`
		MemberSince        string `json:"member_since,omitempty"`
	}

	resp := &Response{
		Posts:              counts.Posts,
		Comments:           counts.Comments,
		Followers:          counts.Followers,
		Following:          counts.Following,
		GroupsAdministered: counts.GroupsAdministered,
	}

	// Omitted rather than sent as an epoch date when unknown. The dialog can
	// leave the line out; it cannot un-say "member since 1 January 1970".
	if counts.MemberSinceUnix > 0 {
		resp.MemberSince = time.Unix(counts.MemberSinceUnix, 0).UTC().Format(time.RFC3339)
	}

	return c.JSON(http.StatusOK, resp)
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(person.ID, constants.NodeActorPersonEntityType, viewer)
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
