package services

import (
	"fmt"
	"strings"

	"auth/data/entities"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnRPDisplayName is what the OS shows in the biometric prompt
// ("Sign in to Anahita using Touch ID"). Changing it is cosmetic and
// safe. Changing the RP ID is not — see NewWebAuthn.
const WebAuthnRPDisplayName = "Anahita"

// NewWebAuthn builds the relying-party instance used by both
// ceremonies.
//
// RP ID IS LOAD-BEARING AND EFFECTIVELY PERMANENT.
//
// Every credential a person enrols is cryptographically scoped to the
// RP ID in force when they enrolled it. Change the RP ID and every
// existing passkey stops working, silently — the browser simply won't
// offer them. There is no migration path; people have to re-enrol.
//
// The RP ID must also be a registrable suffix of the origin running
// the ceremony. That constraint decides the value here: registration
// runs in the SPA (app.<domain>, via the BFF) and login runs on the
// auth host (auth.<domain>), so the RP ID has to be the shared parent
// — <domain> itself, not auth.<domain>. Setting it to the auth host
// would make registration from the SPA impossible.
//
// origins is the exact-match allowlist of origins permitted to run
// ceremonies. Unlike RP ID this is a plain string comparison, so it
// needs every scheme+host that can host the login or settings UI.
func NewWebAuthn(rpID string, origins []string) (*webauthn.WebAuthn, error) {
	if rpID == "" {
		return nil, fmt.Errorf("webauthn: RP ID is not configured")
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("webauthn: no RP origins configured")
	}

	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: WebAuthnRPDisplayName,
		RPOrigins:     origins,

		AuthenticatorSelection: protocol.AuthenticatorSelection{
			// Discoverable credentials. Costs a little authenticator
			// storage and buys usernameless sign-in: tap the sensor,
			// no email typed first. Without this the login page would
			// have to ask who you are before it could ask the
			// authenticator anything.
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),

			// Required, not preferred. "Preferred" lets an
			// authenticator skip the biometric/PIN gesture and prove
			// possession only — which would make a passkey login
			// single-factor, and would undercut the decision to skip
			// TOTP on this path. Required keeps the gesture
			// mandatory and the path genuinely two-factor.
			UserVerification: protocol.VerificationRequired,
		},
	})
}

// WebAuthnUser adapts a Person plus their stored credentials to the
// interface go-webauthn expects.
//
// Built per-ceremony rather than held anywhere: the credential set has
// to be current at the moment of verification, not whatever was cached
// when the request started.
type WebAuthnUser struct {
	user        *entities.User
	credentials []webauthn.Credential
}

func NewWebAuthnUser(
	user *entities.User,
	stored []*entities.WebAuthnCredential,
) *WebAuthnUser {
	creds := make([]webauthn.Credential, 0, len(stored))
	for _, c := range stored {
		creds = append(creds, c.ToLibrary())
	}

	return &WebAuthnUser{
		user:        user,
		credentials: creds,
	}
}

// WebAuthnID is the user handle stored on the authenticator and
// returned to us during discoverable login.
//
// The person's numeric ID, not their email — the spec forbids PII
// here, because the handle is readable off the authenticator. The ID
// is also stable across email changes, which matters: this value is
// baked into the credential at enrolment and cannot be updated
// afterwards.
//
// The encoding lives in entities.WebAuthnUserHandle, which the login
// path decodes with — see the warning there before changing it.
func (u *WebAuthnUser) WebAuthnID() []byte {
	return entities.WebAuthnUserHandle(u.user.ID)
}

// WebAuthnName is what the OS shows when the person picks between
// several accounts on the same device.
func (u *WebAuthnUser) WebAuthnName() string {
	return u.user.Email
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	name := strings.TrimSpace(u.user.Name)
	if name == "" {
		return u.user.Email
	}
	return name
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}
