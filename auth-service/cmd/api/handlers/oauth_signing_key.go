package handlers

import (
	"errors"
	"log"
	"net/http"

	"auth/cmd/api/permissions"
	"auth/data/entities"
	"auth/data/repositories"
	"auth/requests"
	"auth/responses"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// oauthSigningKeyHandler holds dependencies for the
// /oauth-signing-keys endpoints.
type oauthSigningKeyHandler struct {
	*Env
}

func newOAuthSigningKeyHandler(env *Env) *oauthSigningKeyHandler {
	return &oauthSigningKeyHandler{Env: env}
}

// Browse handles GET /oauth-signing-keys
//
// Returns ALL signing keys (active, inactive, expired) for the admin
// UI. Excludes private key material. Sorted newest-first.
//
// 200: { "data": [...] }
// 403: viewer lacks oauth:read scope or super-admin role
// 500: database error
func (h *oauthHandler) BrowseSigningKeys(c echo.Context) error {
	viewer := h.viewerFrom(c)

	if allowed, permErr := h.Perms.OAuthSigningKey.CanBrowseSigningKeys(viewer); !allowed {
		return permForbidden(c, permErr)
	}

	keys, err := h.Repos.OAuthSigningKeys.Browse()
	if err != nil {
		log.Println("oauth_signing_keys browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// nil → [] so the frontend always sees an array.
	if keys == nil {
		keys = []*entities.OAuthSigningKey{}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": keys,
	})
}

// Read handles GET /oauth-signing-keys/:kid
//
// Returns one signing key by its kid. Lookup happens before the
// permission check because the per-entity permission method takes
// the key — matches the OAuthPerm.CanReadClient pattern.
//
// PrivateKey is zeroed before the response is written, defensively
// guarding against a missing `json:"-"` tag on the entity field.
//
// 200: { "data": <signing key> }
// 400: missing kid
// 403: viewer lacks oauth:read scope or super-admin role
// 404: no key with that kid
// 500: database error
func (h *oauthHandler) ReadSigningKey(c echo.Context) error {
	viewer := h.viewerFrom(c)

	req := requests.NewOAuthSigningKeyRequest().Read

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	key, err := h.Repos.OAuthSigningKeys.ReadByKeyID(req.Kid)
	if err != nil {
		if isNotFoundLikeErr(err) {
			return c.NoContent(http.StatusNotFound)
		}
		log.Println("oauth_signing_keys read:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if allowed, permErr := h.Perms.OAuthSigningKey.CanReadSigningKey(viewer, key); !allowed {
		return permForbidden(c, permErr)
	}

	key.PrivateKey = ""
	return c.JSON(http.StatusOK, map[string]any{
		"data": key,
	})
}

// Edit handles PATCH /oauth-signing-keys/:kid
//
// The only mutable field is `active`. Atomic last-active guard in the
// repo prevents the operation from leaving zero active keys (which
// would block all token minting).
//
// Body:
//
//	{ "active": true }   // enable
//	{ "active": false }  // disable (with last-active guard)
//
// 200: { "data": <updated signing key> }
// 400: missing kid or malformed body
// 403: viewer lacks oauth:write scope or super-admin role
// 404: no key with that kid
// 409: would leave zero active keys
// 500: database error
func (h *oauthHandler) EditSigningKey(c echo.Context) error {
	viewer := h.viewerFrom(c)

	req := requests.NewOAuthSigningKeyRequest().Edit

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	existing, err := h.Repos.OAuthSigningKeys.ReadByKeyID(req.Kid)
	if err != nil {
		if isNotFoundLikeErr(err) {
			return c.NoContent(http.StatusNotFound)
		}
		log.Println("oauth_signing_keys edit lookup:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if allowed, permErr := h.Perms.OAuthSigningKey.CanEditSigningKey(viewer, existing); !allowed {
		return permForbidden(c, permErr)
	}

	if err := h.Repos.OAuthSigningKeys.SetActive(existing.ID, *req.Active); err != nil {
		if errors.Is(err, repositories.ErrCannotDeactivateLastKey) {
			return c.JSON(http.StatusConflict, map[string]string{
				"error":   "last_active_key",
				"message": "cannot deactivate the last active signing key",
			})
		}
		log.Println("oauth_signing_keys edit:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	updated, err := h.Repos.OAuthSigningKeys.ReadByKeyID(req.Kid)
	if err != nil {
		log.Println("oauth_signing_keys edit reread:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	updated.PrivateKey = ""
	return c.JSON(http.StatusOK, map[string]any{
		"data": updated,
	})
}

// viewerFrom pulls the authenticated viewer that setViewer middleware
// stuffed into the request context.
func (h *oauthHandler) viewerFrom(c echo.Context) *libs_entities.Viewer {
	v, _ := c.Get(libs_constants.ViewerContextKey).(*libs_entities.Viewer)
	return v
}

// permForbidden translates a permission error into an OAuth-shaped 403
// response. Insufficient-scope errors get the RFC 6750 error code so
// callers can distinguish "your token is missing a scope" from
// "you're the wrong person."
func permForbidden(c echo.Context, err error) error {
	if err == nil {
		return c.NoContent(http.StatusForbidden)
	}
	switch err.Error() {
	case permissions.ErrOAuthSigningKeyInsufficientScope:
		return c.JSON(http.StatusForbidden, &responses.OAuthErrorResponse{
			Error:       "insufficient_scope",
			Description: err.Error(),
		})
	case permissions.ErrOAuthSigningKeyTotpRequired,
		permissions.ErrOAuthClientTotpRequired:
		return c.JSON(http.StatusForbidden, &responses.OAuthErrorResponse{
			Error:       "totp_required",
			Description: err.Error(),
		})
	}
	return c.JSON(http.StatusForbidden, &responses.OAuthErrorResponse{
		Error:       "forbidden",
		Description: err.Error(),
	})
}

// isNotFoundLikeErr — same defensive pattern as the seeders. The repo
// currently leaks sql.ErrNoRows directly for the not-found case;
// once it translates that to a domain sentinel at the boundary, this
// can be replaced with a single errors.Is check.
func isNotFoundLikeErr(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "sql: no rows in result set"
}
