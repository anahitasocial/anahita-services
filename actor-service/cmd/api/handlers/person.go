package handlers

import (
	"actor/responses"
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
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
	gReq.Featured = req.Featured
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

	actors, err := h.Srv.ActorsGRPC.Browse(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if len(actors) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	batchPresignPaths(h.Env, actors)

	// The actors ARE the people. This used to read people_people for the same
	// ids and merge the two lists by id, which also carried a nil dereference:
	// a person whose node did not come back from the browse indexed a missing
	// map entry.
	people := make([]*entities.Person, 0, len(actors))
	for _, actor := range actors {
		person := entities.NewPerson()
		person.AnahitaNode = *actor
		people = append(people, person)
	}

	for _, person := range people {
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

	person, err := h.readPersonByAlias(c, req.Alias)
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

	// One name, and one write.
	//
	// given_name and family_name are gone: nodes.name already holds the
	// correct value, while those two were truncated at the first non-ASCII
	// byte — "Jos" for "José". The gender enum is gone with them, replaced by
	// free-text pronouns, so there is no longer a second table to update and
	// no enum to validate against a list the handler and the validator
	// disagreed about.
	person.Name = req.Name
	person.PersonPronouns = req.PersonPronouns
	person.Body = req.Body

	// The language this person usually posts in. Left out, it stays as it
	// was.
	if language, valid := an_handlers.RequestLanguage(req.Language); !valid {
		return an_handlers.RefuseLanguage(c)
	} else if language != "" {
		person.Language = language
	}
	person.WebsiteURL = req.WebsiteURL
	person.ModifiedBy = viewer.ID
	person.UpdatedAt = time.Now()

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

	if err := h.Srv.Mentions.Update(person, person.Mentions, viewer); err != nil {
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
		if isLastSuperAdmin(err) {
			return refuseLastSuperAdmin(c, "delete")
		}
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

	person, err := h.fetchPerson(c, req.ID)
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

	archived, err := h.Srv.ActorsGRPC.Archive(person.ID, viewer)
	if err != nil {
		if isLastSuperAdmin(err) {
			return refuseLastSuperAdmin(c, "archive")
		}
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

	person, err := h.fetchPerson(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	changed, err := h.Srv.ActorsGRPC.SetEnabled(person.ID, *req.Enabled, viewer)
	if err != nil {
		if isLastSuperAdmin(err) {
			return refuseLastSuperAdmin(c, "disable")
		}
		log.Println("person set-enabled:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !changed {
		return c.NoContent(http.StatusNoContent)
	}

	return c.NoContent(http.StatusOK)
}

// SetFeatured features or unfeatures a profile: the accounts onboarding
// offers a new member to follow.
//
//	PATCH /people/:id/featured   { "featured": true | false }
//
// SUPER ADMINISTRATORS ONLY. Featuring is a statement by the installation —
// "these are worth following here" — not a moderation action, so an ordinary
// administrator may not make it, and nobody may feature themselves.
//
// 200: changed
// 204: already in that state
// 409: featuring a profile that is disabled, archived or deleted
func (h *personHandler) SetFeatured(c echo.Context) error {
	// One bind, for the reason SetEnabled gives.
	var req struct {
		ID       int64 `param:"id"       validate:"required,numeric"`
		Featured *bool `json:"featured"  validate:"required"`
	}

	viewer := getViewer(c)

	if err := c.Bind(&req); err != nil {
		log.Println("person set-featured bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if req.ID == 0 || req.Featured == nil {
		log.Println("person set-featured: missing id or featured field")
		return c.NoContent(http.StatusBadRequest)
	}

	if viewer == nil || !viewer.IsSuperAdmin() {
		log.Println("person set-featured: refused, viewer is not a super administrator")
		return c.NoContent(http.StatusForbidden)
	}

	person, err := h.fetchPerson(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	changed, err := h.Srv.ActorsGRPC.SetFeatured(person.ID, *req.Featured, viewer)
	if err != nil {
		if services.IsNotFeaturable(err) {
			return refuseNotFeaturable(c)
		}
		log.Println("person set-featured:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !changed {
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

	// The composers the viewer may use on this profile, from the check the
	// media services enforce, so the web app offers only ones that will work.
	node.Authorized.Composers = permissions.Composers(viewer, &node.AnahitaNode, actorAccessCtx.ActorViewerRel)
	// And who a post written here may be shown to, from the rule the media
	// services enforce.
	node.Authorized.Audiences = permissions.PostAudiences(viewer, &node.AnahitaNode, actorAccessCtx.ActorViewerRel)

	return nil
}

// fetchPerson fetches a person by ID and then assigns the actor node
// readPersonByAlias reads a person by their handle.
//
// The handle is nodes.alias now; it used to be people_people.username, kept in
// step with the alias by whoever remembered to write both. One column, one
// read, and the graph applies the viewer's access rules to it — which the
// direct SELECT did not.
func (h *personHandler) readPersonByAlias(c echo.Context, alias string) (*entities.Person, error) {
	req := services.NewGraphNodeRequest().ReadByAlias
	req.Alias = alias
	req.NodeType = constants.NodeActorPersonEntityType

	node, err := h.Srv.GraphNodeGRPC.ReadByAlias(req)
	if err != nil {
		return nil, err
	}

	// Resolved to an id, then read through the access-checked path rather than
	// returned straight from the alias lookup. NodeService.ReadByAlias takes no
	// viewer, so it answers the same for everybody; fetchPerson applies the
	// viewer's rules and presigns the avatar, which every other read of a
	// person already does.
	return h.fetchPerson(c, node.ID)
}

func (h *personHandler) fetchPerson(c echo.Context, id int64) (*entities.Person, error) {
	viewer := getViewer(c)

	// One read, not two stitched together.
	//
	// This used to read people_people for the person half, read the actor node
	// for the other half, and assign one over the other. Everything the first
	// read returned now lives on the node — name instead of given/family,
	// alias instead of username, person_type instead of usertype — so the
	// actor IS the person.
	//
	// The stitch also had a failure mode of its own: a people_people row whose
	// node did not come back left the caller holding a Person with a zero
	// AnahitaNode rather than an error.
	actor, err := h.Srv.ActorsGRPC.ReadOfType(id, constants.NodeActorPersonEntityType, viewer)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	person := entities.NewPerson()
	person.AnahitaNode = *actor

	if err := h.onAfterFetched(c, person); err != nil {
		return nil, err
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{&person.AnahitaNode})

	return person, nil
}
