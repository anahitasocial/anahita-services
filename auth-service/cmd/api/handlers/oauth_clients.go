package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"strings"

	"auth/data/entities"
	"auth/requests"
	"auth/responses"

	"github.com/labstack/echo/v4"
)

// BrowseClients returns the full list of registered OAuth clients.
//
// Super-admin only. The list excludes secret_hash; secrets are never
// retrievable after creation/rotation.
func (h *oauthHandler) BrowseClients(c echo.Context) error {
	viewer := getViewer(c)

	if ok, err := h.Perms.OAuth.CanBrowseClients(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	clients, err := h.Repos.OAuthClients.Browse()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	type Response struct {
		Data []*responses.OAuthClientItem `json:"data"`
	}

	oauthResp := &responses.OAuthClientResponse{}
	return c.JSON(http.StatusOK, &Response{
		Data: oauthResp.MapClients(clients),
	})
}

// ReadClient returns a single OAuth client by UUID.
func (h *oauthHandler) ReadClient(c echo.Context) error {
	req := requests.NewOAuthClientRequest().Read
	viewer := getViewer(c)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	client, err := h.Repos.OAuthClients.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.OAuth.CanReadClient(viewer, client); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	oauthResp := &responses.OAuthClientResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapClient(client))
}

// AddClient registers a new OAuth client.
//
// For confidential clients, generates a 48-byte random secret, hashes
// it for storage, and returns the plaintext exactly once in the
// response. The plaintext is never recoverable afterwards — clients
// must store it immediately.
//
// For public clients (Confidential=false), no secret is generated;
// PKCE provides client identity proof at /token.
func (h *oauthHandler) AddClient(c echo.Context) error {
	req := requests.NewOAuthClientRequest().Add
	viewer := getViewer(c)

	if ok, err := h.Perms.OAuth.CanAddClient(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// Role first, then presence. A viewer who is not a super admin is
	// refused above rather than asked for a passkey that cannot help
	// them.
	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("oauth client add:", err)
		return stepUpRequired(c, err)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Reject unknown scopes here rather than letting them persist. An
	// unregistered scope on a client row is silent until authorize time,
	// where it surfaces as an opaque invalid_scope for real users — so
	// name the offending scope while the operator is still looking at it.
	if err := h.OAuth.ValidateScopes(strings.Join(req.Scopes, " ")); err != nil {
		return refuse(c, http.StatusBadRequest, "invalid_scope", err.Error())
	}

	client := entities.NewOAuthClient()
	client.ClientID = req.ClientID
	client.Name = req.Name
	client.RedirectURIs = req.RedirectURIs
	client.GrantTypes = req.GrantTypes
	client.Scopes = req.Scopes
	client.TokenExpiry = req.TokenExpiry
	client.Confidential = req.Confidential
	client.SkipConsent = req.SkipConsent
	client.FirstParty = true // First-party status is immutable and always true
	client.Active = req.Active

	var clientSecret string
	if client.Confidential {
		secret, err := generateRandomString(48)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		clientSecret = secret
		client.SecretHash = h.OAuth.HashToken(secret)
	}

	persisted, err := h.Repos.OAuthClients.Add(client)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One proof, one registration.
	h.clearStepUp(viewer.ID)

	oauthResp := &responses.OAuthClientResponse{}
	if clientSecret != "" {
		// Confidential client — return the plaintext secret once.
		return c.JSON(http.StatusCreated, oauthResp.MapClientWithSecret(persisted, clientSecret))
	}
	return c.JSON(http.StatusCreated, oauthResp.MapClient(persisted))
}

// EditClient updates an existing OAuth client. ClientID and SecretHash
// are immutable through this endpoint — for credential rotation use
// RotateClientSecret.
func (h *oauthHandler) EditClient(c echo.Context) error {
	req := requests.NewOAuthClientRequest().Edit
	viewer := getViewer(c)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	client, err := h.Repos.OAuthClients.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.OAuth.CanEditClient(viewer, client); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("oauth client edit:", err)
		return stepUpRequired(c, err)
	}

	if err := h.OAuth.ValidateScopes(strings.Join(req.Scopes, " ")); err != nil {
		return refuse(c, http.StatusBadRequest, "invalid_scope", err.Error())
	}

	client.Name = req.Name
	client.RedirectURIs = req.RedirectURIs
	client.GrantTypes = req.GrantTypes
	client.Scopes = req.Scopes
	client.TokenExpiry = req.TokenExpiry
	client.Confidential = req.Confidential
	client.SkipConsent = req.SkipConsent
	client.FirstParty = true // First-party status is immutable and always true
	client.Active = req.Active

	persisted, err := h.Repos.OAuthClients.Edit(client)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.clearStepUp(viewer.ID)

	oauthResp := &responses.OAuthClientResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapClient(persisted))
}

// DeleteClient removes a client and revokes all its outstanding state.
//
// The schema has no foreign keys, so nothing cascades: the client's refresh
// tokens are revoked and its authorization codes deleted here, before the
// client's row, so a failure leaves the client in place to be deleted again.
// Left behind, they would work again for a client later made under the same
// client_id.
func (h *oauthHandler) DeleteClient(c echo.Context) error {
	req := requests.NewOAuthClientRequest().Delete
	viewer := getViewer(c)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	client, err := h.Repos.OAuthClients.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.OAuth.CanDeleteClient(viewer, client); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("oauth client delete:", err)
		return stepUpRequired(c, err)
	}

	if err := h.Repos.OAuthRefreshTokens.RevokeByClientID(client.ClientID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.Repos.OAuthAuthorizations.DeleteByClientID(client.ClientID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.Repos.OAuthClients.Delete(client.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.clearStepUp(viewer.ID)

	oauthResp := &responses.OAuthClientResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapClient(client))
}

// RotateClientSecret generates a new secret for a confidential client.
//
// Public clients have no secret to rotate — returns 400. The new
// plaintext secret is returned exactly once. All outstanding refresh
// tokens for the client are revoked since the old secret is now
// considered compromised.
func (h *oauthHandler) RotateClientSecret(c echo.Context) error {
	req := requests.NewOAuthClientRequest().Read
	viewer := getViewer(c)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	client, err := h.Repos.OAuthClients.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.OAuth.CanEditClient(viewer, client); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("oauth client rotate secret:", err)
		return stepUpRequired(c, err)
	}

	if !client.Confidential {
		return refuse(c, http.StatusBadRequest, "public_client", "public clients have no secret to rotate")
	}

	newSecret, err := generateRandomString(48)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	client.SecretHash = h.OAuth.HashToken(newSecret)

	persisted, err := h.Repos.OAuthClients.Edit(client)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Revoke all outstanding refresh tokens — the old secret is now
	// considered compromised, so any tokens issued under it should not
	// be refreshable.
	if err := h.Repos.OAuthRefreshTokens.RevokeByClientID(client.ClientID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.clearStepUp(viewer.ID)

	oauthResp := &responses.OAuthClientResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapClientWithSecret(persisted, newSecret))
}

// generateRandomString generates a URL-safe random string of approximately
// `bytes * 4/3` characters (base64-RawURL encoding).
func generateRandomString(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
