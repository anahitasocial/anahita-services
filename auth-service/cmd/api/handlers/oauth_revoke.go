package handlers

import (
	"log"
	"net/http"

	"auth/requests"
	"auth/responses"

	"github.com/labstack/echo/v4"
)

// AddRevoke handles POST /oauth/revoke
//
// RFC 7009 token revocation. Per spec, always returns 200 regardless of
// whether the token existed — to prevent information leakage about
// token validity to unauthorized clients.
//
// Anahita revokes refresh tokens. Access tokens are short-
// lived (15 minutes) and rely on natural expiry; this is documented in
// the service-layer RevokeToken comment.
func (h *oauthHandler) AddRevoke(c echo.Context) error {
	var (
		req = requests.NewOAuthTokenRequest().Revoke
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	client, err := h.OAuth.AuthenticateClientFromRequest(
		c.Request().Header.Get("Authorization"),
		req.ClientID,
		req.ClientSecret,
	)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusUnauthorized, &responses.OAuthErrorResponse{
			Error:       "invalid_client",
			Description: err.Error(),
		})
	}

	if err := h.OAuth.RevokeToken(req.Token, req.TokenTypeHint, client); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// RFC 7009 §2.2: always return 200 regardless of whether the token
	// existed.
	return c.NoContent(http.StatusOK)
}
