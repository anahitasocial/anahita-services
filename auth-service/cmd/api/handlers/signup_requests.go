package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"
	libs_responses "anahita-libs/responses"

	auth_constants "auth/constants"
	"auth/data/entities"

	"github.com/labstack/echo/v4"
)

// The approval queue's administration surface.
//
// JSON, and administrator-only. The SCREEN that consumes it belongs with the
// other settings work and is not here; these endpoints are what it will call,
// and what makes `REGISTRATION_MODE=approval` usable in the meantime.

type signupRequestsHandler struct {
	*Env
}

func newSignupRequestsHandler(env *Env) *signupRequestsHandler {
	return &signupRequestsHandler{env}
}

// signupRequestResponse is what an administrator sees.
//
// NO PASSWORD HASH, and no token. The hash is in the row because approval
// needs it, and it leaves the building for nobody — a queue that serialises
// credentials is a far worse thing to leak than a list of people who would
// like to join.
type signupRequestResponse struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	WebsiteURL string `json:"website_url"`
	Status     string `json:"status"`
	InvitedBy  int64  `json:"invited_by,omitempty"`
	CreatedIP  string `json:"created_ip"`
	CreatedAt  string `json:"created_at"`

	// UsernameTaken says whether the name they asked for is still free.
	//
	// Computed at read time rather than stored, because it changes underneath
	// the queue: somebody else may take it while a request waits. An
	// administrator deciding without this finds out by having the approval
	// fail.
	UsernameTaken bool `json:"username_taken"`
}

func (h *signupRequestsHandler) mapSignupRequest(
	sr *entities.SignupRequest,
) signupRequestResponse {
	taken, err := h.Repos.Users.IsUsername(sr.Username)
	if err != nil {
		log.Println("signup requests: username check failed:", err)
	}

	return signupRequestResponse{
		ID:            sr.ID,
		Username:      sr.Username,
		Email:         sr.Email,
		Name:          sr.Name,
		Body:          sr.Body,
		WebsiteURL:    sr.WebsiteURL,
		Status:        sr.Status,
		InvitedBy:     sr.InvitedBy,
		CreatedIP:     sr.CreatedIP,
		CreatedAt:     sr.CreatedAt.Format(time.RFC3339),
		UsernameTaken: taken,
	}
}

// Browse lists the queue, oldest first.
//
//	GET /signup-requests
//
// Answers { "data": [...], "pagination": { offset, limit, total } },
// the same envelope as /invites and every other browse in Anahita.
// This used to return { "signup_requests": [...], "total": n }, which
// no client's browse path knows how to read — actions/create.js in the
// React client unwraps data.data and takes limit/offset/total off
// pagination, so the queue would have arrived empty with a total of
// zero and no way to page through it.
func (h *signupRequestsHandler) Browse(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.SignupRequest.CanBrowse(viewer); err != nil {
		log.Println("signup requests browse:", err)
		return c.NoContent(http.StatusForbidden)
	}

	limit, _ := strconv.ParseUint(c.QueryParam("limit"), 10, 64)
	offset, _ := strconv.ParseUint(c.QueryParam("offset"), 10, 64)

	requests, err := h.Repos.SignupRequests.BrowsePending(limit, offset)
	if err != nil {
		log.Println("signup requests browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	total, err := h.Repos.SignupRequests.CountPending()
	if err != nil {
		log.Println("signup requests browse: count failed:", err)
	}

	items := make([]signupRequestResponse, 0, len(requests))
	for _, sr := range requests {
		items = append(items, h.mapSignupRequest(sr))
	}

	type Response struct {
		Data       []signupRequestResponse   `json:"data"`
		Pagination libs_responses.Pagination `json:"pagination"`
	}

	return c.JSON(http.StatusOK, &Response{
		Data: items,
		Pagination: libs_responses.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  uint64(total),
		},
	})
}

type decisionBody struct {
	Note string `json:"note"`
}

// Approve creates the account somebody asked for.
//
//	POST /signup-requests/:id/approve
func (h *signupRequestsHandler) Approve(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.SignupRequest.CanApprove(viewer); err != nil {
		log.Println("signup requests approve:", err)
		return c.NoContent(http.StatusForbidden)
	}

	sr, err := h.readPendingSignupRequest(c)
	if err != nil {
		return err
	}

	body := &decisionBody{}
	_ = c.Bind(body)

	// The username is checked again HERE, not only when they asked.
	//
	// A request can wait for days, and somebody else can take the name in the
	// meantime. Without this the approval fails on a duplicate-key error from
	// the graph, which tells the administrator nothing and leaves the request
	// in the queue looking untouched.
	if taken, err := h.Repos.Users.IsUsername(sr.Username); err != nil {
		log.Println("signup requests approve: username check failed:", err)
		return c.NoContent(http.StatusInternalServerError)
	} else if taken {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "username_taken",
			"message": "The username on this request has been taken since " +
				"it was submitted. Ask them to choose another, or " +
				"reject it.",
		})
	}

	// The account, through the same path signup uses: the actor first, then
	// the credential row, with a compensating purge if the second fails.
	actor := lib_entities.NewAnahitaNode()
	actor.Type = constants.NodeActorPersonEntityType
	actor.Service = constants.PersonService
	actor.Alias = sr.Username
	actor.Name = sr.Name
	actor.Body = sr.Body
	actor.WebsiteURL = sr.WebsiteURL
	actor.Access = constants.AnahitaAccessPublic
	actor.PersonType = constants.PersonTypeRegistered
	actor.Enabled = true
	actor.CreationTime = time.Now()
	actor.UpdateTime = time.Now()

	created, err := h.Srv.ActorsGRPC.Add(actor, signupViewer())
	if err != nil {
		log.Println("signup requests approve: could not create the actor:", err)
		if isDuplicateHandle(err) {
			return c.JSON(http.StatusConflict, map[string]string{
				"error":   "username_taken",
				"message": "That username was taken moments ago. Please try again.",
			})
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	user := entities.NewUser()
	user.ID = created.ID
	user.Alias = created.Alias
	user.Name = created.Name
	user.Email = sr.Email
	user.Password = sr.Password
	user.PersonType = constants.PersonTypeRegistered
	user.Enabled = true
	user.CreationTime = time.Now()
	user.UpdateTime = time.Now()
	// Verified already: confirming the address is what put this request in
	// front of an administrator in the first place.
	user.EmailVerifiedAt = sr.EmailVerifiedAt
	// The member who vouched, when a member's invitation is how this request
	// arrived; 0, written as NULL, for a cold application. Onboarding shows
	// the inviter first, and the request row is not somewhere it should have
	// to look once the account exists.
	user.InvitedBy = sr.InvitedBy

	if _, err := h.Repos.Users.Add(user); err != nil {
		log.Println("signup requests approve: could not create the account:", err)
		if purgeErr := h.purgeOrphanedActor(created.ID); purgeErr != nil {
			log.Printf("signup requests approve: orphaned actor %d: %v (purge also failed: %v)",
				created.ID, err, purgeErr)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	// Recorded LAST. If this fails the account exists and the request still
	// says pending, which a second approval attempt turns into a
	// duplicate-username refusal — visible, and recoverable by rejecting the
	// stale entry. The other order would mark somebody approved and leave them
	// with no account at all.
	if err := h.Repos.SignupRequests.Decide(
		sr.ID, entities.SignupRequestStatusApproved, viewer.ID, body.Note,
	); err != nil {
		log.Printf("signup requests approve: account %d created but the "+
			"request was not marked approved: %v", created.ID, err)
	}

	// Settle any invitations still outstanding to this address.
	//
	// The same sweep the invite-accept path runs, and needed here for
	// the case it cannot cover: under `approval` an invited person
	// lands in this queue rather than in an account, so their
	// invitation is consumed while several others may still be
	// pending — and they are only really spent once somebody says yes,
	// which is here. Doing it at queue time would spend them on a
	// request that might be rejected.
	//
	// Also covers a cold applicant who happened to be invited by
	// somebody else while they were waiting.
	if _, err := h.Repos.Tokens.InvalidateOutstanding(
		auth_constants.TokenTypeInvite, sr.Email,
	); err != nil {
		log.Printf("signup requests approve: could not settle invitations for %s: %v",
			sr.Email, err)
	}

	notifyRequest := *sr
	go func() {
		if err := h.Notif.Signup.EmailSignupRequestApproved(&notifyRequest); err != nil {
			log.Println("signup requests approve: notification failed:", err)
		}
	}()

	return c.JSON(http.StatusOK, map[string]any{
		"status":    entities.SignupRequestStatusApproved,
		"person_id": created.ID,
	})
}

// Reject refuses a request.
//
//	POST /signup-requests/:id/reject
func (h *signupRequestsHandler) Reject(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.SignupRequest.CanReject(viewer); err != nil {
		log.Println("signup requests reject:", err)
		return c.NoContent(http.StatusForbidden)
	}

	sr, err := h.readPendingSignupRequest(c)
	if err != nil {
		return err
	}

	body := &decisionBody{}
	_ = c.Bind(body)

	if err := h.Repos.SignupRequests.Decide(
		sr.ID, entities.SignupRequestStatusRejected, viewer.ID, body.Note,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Somebody else decided it between the read and the write.
			return c.JSON(http.StatusConflict, map[string]string{
				"error":   "already_decided",
				"message": "Another administrator has already answered this request.",
			})
		}
		log.Println("signup requests reject:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	notifyRequest := *sr
	go func() {
		if err := h.Notif.Signup.EmailSignupRequestRejected(&notifyRequest); err != nil {
			log.Println("signup requests reject: notification failed:", err)
		}
	}()

	return c.JSON(http.StatusOK, map[string]any{
		"status": entities.SignupRequestStatusRejected,
	})
}

// readPendingSignupRequest loads the request named in the path, or writes the
// refusal and returns an error for the caller to return.
func (h *signupRequestsHandler) readPendingSignupRequest(
	c echo.Context,
) (*entities.SignupRequest, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, c.NoContent(http.StatusBadRequest)
	}

	sr, err := h.Repos.SignupRequests.ReadByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, c.NoContent(http.StatusNotFound)
		}
		log.Println("signup requests: read failed:", err)
		return nil, c.NoContent(http.StatusInternalServerError)
	}

	if sr.IsDecided() {
		return nil, c.JSON(http.StatusConflict, map[string]string{
			"error":   "already_decided",
			"message": "This request has already been answered.",
		})
	}

	// An unverified request is not in the queue and must not be decidable from
	// it — approving one would create an account on an address nobody has
	// shown they can receive mail at, which is the whole thing verification is
	// for.
	if !sr.IsEmailVerified() {
		return nil, c.JSON(http.StatusConflict, map[string]string{
			"error":   "not_verified",
			"message": "This person has not confirmed their email address yet.",
		})
	}

	return sr, nil
}
