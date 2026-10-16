package responses

import (
	"time"

	"auth/data/entities"
)

type OAuthClientItem struct {
	// Its number here: what the routes name it by.
	ID int64 `json:"id"`
	// What it calls itself when it signs people in: a slug.
	ClientID string `json:"client_id"`
	// What people are shown it is called.
	Name string `json:"name"`
	// Where people may be sent back to after signing in. Exactly these.
	RedirectURIs []string `json:"redirect_uris"`
	// How it may get tokens: `authorization_code`, `refresh_token`, `client_credentials`.
	GrantTypes []string `json:"grant_types"`
	// What it may ask to do (oauth.md).
	Scopes []string `json:"scopes"`
	// How long its access tokens last, in seconds.
	TokenExpiry int `json:"token_expiry"`
	// Whether it can keep a secret: an app's own server can, an app in a browser or on a phone cannot.
	Confidential bool `json:"confidential"`
	// Whether people are not asked to allow it.
	SkipConsent bool `json:"skip_consent"`
	// Whether it is one of the installation's own apps.
	FirstParty bool `json:"first_party"`
	// Whether it may sign people in.
	Active bool `json:"active"`
	// When it was registered.
	CreatedAt time.Time `json:"created_at"`
}

// OAuthClientCreatedItem is the response shape used only on Add and
// RotateSecret — the plaintext client_secret is exposed once and never
// returned by any other endpoint.
type OAuthClientCreatedItem struct {
	OAuthClientItem
	// Its secret, shown this once and never again. Empty for a public app.
	ClientSecret string `json:"client_secret"`
}

type OAuthClientResponse struct{}

func (r *OAuthClientResponse) MapClient(client *entities.OAuthClient) *OAuthClientItem {
	if client == nil {
		return nil
	}

	return &OAuthClientItem{
		ID:           client.ID,
		ClientID:     client.ClientID,
		Name:         client.Name,
		RedirectURIs: client.RedirectURIs,
		GrantTypes:   client.GrantTypes,
		Scopes:       client.Scopes,
		TokenExpiry:  client.TokenExpiry,
		Confidential: client.Confidential,
		SkipConsent:  client.SkipConsent,
		FirstParty:   client.FirstParty,
		Active:       client.Active,
		CreatedAt:    client.CreatedAt,
	}
}

func (r *OAuthClientResponse) MapClients(clients []*entities.OAuthClient) []*OAuthClientItem {
	if clients == nil {
		return nil
	}

	results := make([]*OAuthClientItem, 0, len(clients))
	for _, client := range clients {
		results = append(results, r.MapClient(client))
	}
	return results
}

func (r *OAuthClientResponse) MapClientWithSecret(client *entities.OAuthClient, secret string) *OAuthClientCreatedItem {
	if client == nil {
		return nil
	}

	return &OAuthClientCreatedItem{
		OAuthClientItem: *r.MapClient(client),
		ClientSecret:    secret,
	}
}
