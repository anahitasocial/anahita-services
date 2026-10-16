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
	// Its id: what renaming or removing it names.
	ID int64 `json:"id"`
	// What it is called: the device, unless its owner renamed it.
	Nickname string `json:"nickname"`
	// How the browser reaches it: `internal`, `usb`, `nfc`, `ble`, `hybrid`.
	Transports []string `json:"transports"`
	// Whether it is kept in sync across devices, so that losing one loses nothing.
	Synced bool `json:"synced"`
	// Whether it looked copied when last used: worth removing.
	CloneWarning bool `json:"clone_warning"`
	// When it was last used. Null when never.
	LastUsedAt *time.Time `json:"last_used_at"`
	// When it was added.
	CreatedAt time.Time `json:"created_at"`
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
