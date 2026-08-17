package responses

import (
	"time"

	"auth/data/entities"
)

// WebAuthnCredentialItem is the person-facing shape of an enrolled
// passkey.
//
// No credential ID, no public key, no AAGUID. None of it is secret,
// but none of it helps the person decide which passkey to revoke
// either, and shipping key material to a browser is a habit worth not
// forming.
type WebAuthnCredentialItem struct {
	ID           int64      `json:"id"`
	Nickname     string     `json:"nickname"`
	Transports   []string   `json:"transports"`
	Synced       bool       `json:"synced"`
	CloneWarning bool       `json:"clone_warning"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type WebAuthnCredentialResponse struct{}

func (r *WebAuthnCredentialResponse) MapCredential(cred *entities.WebAuthnCredential) *WebAuthnCredentialItem {
	if cred == nil {
		return nil
	}

	transports := cred.Transports
	if transports == nil {
		transports = []string{}
	}

	return &WebAuthnCredentialItem{
		ID:       cred.ID,
		Nickname: cred.Nickname,

		Transports: transports,

		// Synced tells the person whether losing this device loses
		// the passkey. It is the one piece of authenticator metadata
		// that changes what they should do, so it is the one we
		// surface.
		Synced: cred.IsSynced(),

		// Surfaced so the person can act on a possible cloned
		// authenticator. We record it rather than blocking the login,
		// so if we don't show it here nobody ever learns about it.
		CloneWarning: cred.CloneWarning,

		LastUsedAt: cred.LastUsedAt,
		CreatedAt:  cred.CreatedAt,
	}
}

func (r *WebAuthnCredentialResponse) MapCredentials(creds []*entities.WebAuthnCredential) []*WebAuthnCredentialItem {
	// Always an array, never null — the settings UI maps over this
	// directly.
	items := make([]*WebAuthnCredentialItem, 0, len(creds))
	for _, c := range creds {
		items = append(items, r.MapCredential(c))
	}
	return items
}
