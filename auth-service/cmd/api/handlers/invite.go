package handlers

import (
	"log"
	"net/http"
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
		return c.NoContent(http.StatusPaymentRequired)
	}

	browseReq := libs_requests.NewBrowse()
	browseReq.Limit = libs_constants.MaxPageSize

	user, err := h.Repos.Users.ReadByEmail(req.Email)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if user != nil {
		return c.NoContent(http.StatusConflict)
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
