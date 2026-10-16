package handlers

import (
	"log"
	"net/http"
	"strings"

	"auth/data/entities"
	"auth/responses"

	libs_constants "anahita-libs/constants"

	"github.com/labstack/echo/v4"
)

// ReadUserInfo handles GET /oauth/userinfo
//
// Returns OIDC user profile claims for the authenticated viewer,
// filtered by the scopes granted in the access token.
//
// The middleware-built viewer is intentionally thin (just IDs and
// scopes from the JWT). Profile data isn't in the access token —
// fetching it here on demand keeps the JWT small and avoids serving
// stale profile claims after a name/email update.
//
// Hydration: the person is read straight from the database by id, so
// a single lookup covers every claim this endpoint returns.
//
// Per OIDC Core §5.3.1, the openid scope is required. Without it, the
// request is rejected with insufficient_scope.
func (h *oauthHandler) ReadUserInfo(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || !viewer.Authenticated {
		return c.JSON(http.StatusUnauthorized, &responses.OAuthErrorResponse{
			Error:       "invalid_token",
			Description: "no authenticated viewer",
		})
	}

	if !viewer.HasScope(libs_constants.OAuthScopeOpenID) {
		return c.JSON(http.StatusForbidden, &responses.OAuthErrorResponse{
			Error:       "insufficient_scope",
			Description: "openid scope is required",
		})
	}

	// Hydrate the viewer's Person from gRPC — the JWT only carried
	// IDs, not the full profile.
	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println("userinfo: person lookup failed:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	scopes := strings.Join(viewer.Scopes, " ")
	info := h.OAuth.GetUserInfo(user, scopes)

	// What the person may do, and how much, where the installation sets a
	// limit: the account's fields, so with `profile` as the rest are.
	// Here and not in GetUserInfo, which has no settings to read.
	if viewer.HasScope(libs_constants.OAuthScopeProfile) {
		info["entitlements"] = h.entitlements(user)
	}

	return c.JSON(http.StatusOK, info)
}

// Entitlements is what a person may do, and how much, where the
// installation sets a limit. A client offers no more than this; the
// server refuses more regardless.
type Entitlements struct {
	// Sending invitations. Null when they may not invite at all.
	Invites *InviteEntitlement `json:"invites"`
}

// InviteEntitlement is how many invitations a person may have waiting.
type InviteEntitlement struct {
	// The most that may wait at once: sent, and not yet accepted, expired
	// or taken back. How many are waiting now comes with the list of them
	// (`GET /invites`, `pending`).
	MaxPending int `json:"max_pending"`
}

func (h *oauthHandler) entitlements(user *entities.User) Entitlements {
	var out Entitlements
	if h.Config.AllowsInvitations() && h.Config.MayInvite(user.PersonType) {
		admin := user.PersonType == libs_constants.PersonTypeAdmin || user.PersonType == libs_constants.PersonTypeSuperAdmin
		out.Invites = &InviteEntitlement{MaxPending: h.Config.GetInvitePendingMax(admin)}
	}
	return out
}
