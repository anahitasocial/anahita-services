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

	// go func() {
	// 	if err := h.Notif.Invite.EmailInvite(viewer, token.RecipientEmail, token); err != nil {
	// 		log.Printf(
	// 			"failed to send invite email for token %s to %s: %v",
	// 			token.ID, token.RecipientEmail, err,
	// 		)
	// 	}
	// }()

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

	return c.JSON(http.StatusOK, token)
}
