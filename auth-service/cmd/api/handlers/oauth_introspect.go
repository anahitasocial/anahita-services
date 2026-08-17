package handlers

import (
	"log"
	"net/http"

	"auth/requests"
	"auth/responses"

	"github.com/labstack/echo/v4"
)

// AddIntrospect handles POST /oauth/introspect
//
// RFC 7662 token introspection. Authenticated clients submit a token
// and receive metadata about it, including whether it's currently
// active. Returns active=false (with HTTP 200) for invalid, expired,
// revoked, or unknown tokens — never an error response, per RFC 7662 §2.2.
func (h *oauthHandler) AddIntrospect(c echo.Context) error {
	var (
		req = requests.NewOAuthTokenRequest().Introspect
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

	tokenInfo, err := h.OAuth.IntrospectToken(
		c.Request().Context(),
		req.Token,
		req.TokenTypeHint,
		client,
	)
	if err != nil {
		// Per RFC 7662: invalid tokens return active=false, not an error.
		oauthResp := &responses.OAuthIntrospectResponse{}
		return c.JSON(http.StatusOK, oauthResp.MapInactive())
	}

	oauthResp := &responses.OAuthIntrospectResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapTokenInfo(tokenInfo))
}
