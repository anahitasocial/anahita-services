package handlers

import (
	"net/http"

	"anahita-libs/cmd/api/permissions"

	"github.com/labstack/echo/v4"
)

// RefuseAudience answers a post whose audience is not one it may have where
// it is going: 422, with the ones it may.
//
// 422 rather than 400. The value is one of the seven the API knows, so the
// request is well formed; it does not fit this post on this profile. The list
// is sent so a client that offered the wrong choice can offer the right ones.
// RefuseAccess answers a profile whose access may not be what was asked for,
// the same way: 422, with the levels it may have. Today the only reason is an
// installation that allows nothing new to be public.
func RefuseAccess(c echo.Context, allowed []string) error {
	return c.JSON(http.StatusUnprocessableEntity, map[string]any{
		"error":   permissions.ErrAudienceNotAllowed,
		"allowed": allowed,
	})
}

func RefuseAudience(c echo.Context, ctx *permissions.MediumAccessContext) error {
	return c.JSON(http.StatusUnprocessableEntity, map[string]any{
		"error":   permissions.ErrAudienceNotAllowed,
		"allowed": permissions.MediumAudiences(ctx),
	})
}
