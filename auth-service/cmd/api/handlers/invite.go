package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/requests"
	"auth/responses"

	libs_requests "anahita-libs/requests"
	libs_responses "anahita-libs/responses"

	libs_constants "anahita-libs/constants"

	"github.com/labstack/echo/v4"
)

type inviteHandler struct {
	*Env
}

func newInviteHandler(env *Env) *inviteHandler {
	return &inviteHandler{env}
}

func (h *inviteHandler) Browse(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = libs_requests.NewBrowse()
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.Perms.Invite.CanBrowse(viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	type Response struct {
		Data       []*responses.InviteItem   `json:"data"`
		Pagination libs_responses.Pagination `json:"pagination"`
	}

	resp := &Response{
		Data: []*responses.InviteItem{},
		Pagination: libs_responses.Pagination{
			Limit:  req.Limit,
			Offset: req.Offset,
		},
	}

	resp.Pagination.Total, err = h.Repos.Tokens.Count(constants.TokenTypeInvite, req)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	invites, err := h.Repos.Tokens.Browse(constants.TokenTypeInvite, req)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	invitesResp := &responses.InviteResponse{}
	resp.Data = invitesResp.MapInviteItems(invites)

	return c.JSON(http.StatusOK, resp)
}

func (h *inviteHandler) Add(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = &requests.InviteAdd{}
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.Perms.Invite.CanAdd(viewer); err != nil {
		log.Println(err)
		// 403. This said StatusPaymentRequired — 402 — which is not a
		// permission failure and is not something any client knows how to
		// act on.
		return c.NoContent(http.StatusForbidden)
	}

	browseReq := libs_requests.NewBrowse()
	browseReq.Limit = libs_constants.MaxPageSize

	// sql.ErrNoRows is the ANSWER, not a failure: nobody holds this address,
	// which is the whole point of inviting them.
	//
	// Treating every error as a fault meant this returned 500 for every
	// legitimate invitation — the only address that did NOT 500 was one that
	// already had an account, which is the case that is supposed to be
	// refused. Sending an invite has never worked. Same bug, same shape, as
	// the existing-account check in invite_accept.go.
	user, err := h.Repos.Users.ReadByEmail(req.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("invite add: existing-account check failed:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if user != nil {
		return c.NoContent(http.StatusConflict)
	}

	// Metered here, after the cheap refusals and before anything is written
	// or sent.
	//
	// Two counters, because they answer different questions. The per-person
	// one bounds how fast one account can turn the mail service into an
	// outbound relay. The per-recipient one bounds how often ONE address can
	// be mailed by this site — a thing no single inviter's budget can see,
	// since a person who does not want to be invited is not made happier by
	// the invitations coming from different members.
	//
	// Both count, so they belong on the path where the attempt happens. See
	// the note on rateLimitExceeded.
	if h.rateLimitExceeded(rateLimitScopeInvitePerson, strconv.FormatInt(viewer.ID, 10)) {
		log.Printf("invite add: rate limited (person=%d)", viewer.ID)
		return c.NoContent(http.StatusTooManyRequests)
	}

	if h.rateLimitExceeded(rateLimitScopeInviteEmail, strings.ToLower(req.Email)) {
		// Deliberately the same status as the counter above, and nothing in
		// the body. Saying "that address has been invited too often" would
		// tell an inviter something about somebody else's mail — including
		// that another member invited them.
		log.Println("invite add: recipient rate limited")
		return c.NoContent(http.StatusTooManyRequests)
	}

	token := entities.NewToken()
	token.ExpiredAfter = constants.TokenExpiryTime
	token.Type = constants.TokenTypeInvite
	token.IsUsed = false
	token.CreatedAt = time.Now()
	token.CreatedBy = viewer.ID
	token.UpdatedAt = time.Now()
	token.UpdatedBy = viewer.ID
	token.RecipientEmail = req.Email
	token.Status = constants.TokenStatusPending
	if err := token.GenerateToken(); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = token.Validate(); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	token, err = h.Repos.Tokens.Add(token)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// The invitation itself. Until this was restored the token existed only
	// in the JSON response, so an invitee was never sent their link and could
	// not accept — a broken feature rather than a missing notification.
	//
	// The inviter is re-read rather than taken from the viewer: the message
	// names them ("Ada Lovelace invited you to join"), and a Viewer carries a
	// single Name off the JWT rather than the given/family pair. Same posture
	// as the password handler, which re-reads for the same reason.
	inviter, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || inviter == nil {
		// Not fatal. The invitation exists and the token is valid; failing the
		// request here would leave a usable invite that the caller believes
		// failed. Logged so a silent stop is visible.
		log.Println("invite: could not read inviter for notification:", err)
	} else {
		notifyInviter := *inviter
		recipient := token.RecipientEmail
		go func() {
			if err := h.Notif.Invite.EmailInvite(&notifyInviter, recipient, token); err != nil {
				log.Printf(
					"failed to send invite email for token %d to %s: %v",
					token.ID, recipient, err,
				)
			}
		}()
	}

	invitesResp := &responses.InviteResponse{}
	return c.JSON(http.StatusOK, invitesResp.MapInviteItem(token))
}

func (h *inviteHandler) Delete(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = &requests.InviteDelete{}
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.Perms.Invite.CanDelete(viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	token, err := h.Repos.Tokens.ReadByID(req.TokenID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Repos.Tokens.Delete(token.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Tell the invitee their link is dead. Without this the invitation simply
	// stops working, which reads as a broken site rather than a decision —
	// and they have no account to check, so there is nowhere else to find out.
	revoker, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || revoker == nil {
		log.Println("invite: could not read revoker for notification:", err)
	} else {
		notifyRevoker := *revoker
		recipient := token.RecipientEmail
		go func() {
			if err := h.Notif.Invite.EmailRevoked(&notifyRevoker, recipient); err != nil {
				log.Println("invite: revocation notification failed:", err)
			}
		}()
	}

	return c.JSON(http.StatusOK, token)
}
