package handlers

import (
	"log"
	"net/http"

	"auth/constants"
	"auth/requests"
	"auth/responses"

	"github.com/labstack/echo/v4"
)

type tokenHandler struct {
	*Env
}

func newTokenHandler(env *Env) *tokenHandler {
	return &tokenHandler{env}
}

func (h *tokenHandler) Read(c echo.Context) error {
	var (
		err error
		req = &requests.Token{}
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	token, err := h.Repos.Tokens.ReadByBody(req.Body)
	if err != nil {
		log.Println(err, "Token not found!")
		return c.NoContent(http.StatusNotFound)
	}

	if token.Status == constants.TokenStatusUsed {
		log.Println(err, "Token is already used!")
		return c.NoContent(http.StatusForbidden)
	}

	if token.Status == constants.TokenStatusExpired {
		log.Println(err, "Token is expired!")
		return c.NoContent(http.StatusNotAcceptable)
	}

	tokenResp := &responses.TokenResponse{}
	resp := tokenResp.MapToken(token)

	return c.JSON(http.StatusAccepted, resp)
}
