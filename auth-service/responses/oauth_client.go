package responses

import (
	"time"

	"auth/data/entities"
)

type OAuthClientItem struct {
	ID           int64     `json:"id"`
	ClientID     string    `json:"client_id"`
	Name         string    `json:"name"`
	RedirectURIs []string  `json:"redirect_uris"`
	GrantTypes   []string  `json:"grant_types"`
	Scopes       []string  `json:"scopes"`
	TokenExpiry  int       `json:"token_expiry"`
	Confidential bool      `json:"confidential"`
	SkipConsent  bool      `json:"skip_consent"`
	FirstParty   bool      `json:"first_party"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

// OAuthClientCreatedItem is the response shape used only on Add and
// RotateSecret — the plaintext client_secret is exposed once and never
// returned by any other endpoint.
type OAuthClientCreatedItem struct {
	OAuthClientItem
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
