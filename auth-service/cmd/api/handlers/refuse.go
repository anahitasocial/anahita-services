package handlers

import (
	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// refuse answers a refusal that says why: a code a client can tell apart,
// and the same in words.
//
// Not for the OAuth protocol routes (/oauth/token, /oauth/revoke,
// /oauth/introspect, /oauth/userinfo), which answer in the shape RFC 6749
// gives them: responses.OAuthErrorResponse.
func refuse(c echo.Context, status int, code, description string) error {
	return c.JSON(status, libs_responses.Refusal{Error: code, Description: description})
}
