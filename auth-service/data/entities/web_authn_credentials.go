package entities

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnCredential is one enrolled authenticator — a laptop's Touch
// ID or Windows Hello, a phone, a hardware key.
//
// The private key lives in the device's secure enclave and never
// reaches us. What we hold is the public key, which is only useful
// for verifying signatures, plus enough metadata to show the person a
// list they can reason about.
//
// JSON tags: everything cryptographic is `-`. The credentials list is
// mapped through responses.WebAuthnCredentialResponse, but tagging the
// entity too means a stray c.JSON(entity) can't leak key material.
type WebAuthnCredential struct {
	ID              int64      `json:"-"`
	UserID          int64      `json:"-"`
	CredentialID    []byte     `json:"-"`
	PublicKey       []byte     `json:"-"`
	AttestationType string     `json:"-"`
	AAGUID          []byte     `json:"-"`
	SignCount       uint32     `json:"-"`
	CloneWarning    bool       `json:"clone_warning"`
	Transports      []string   `json:"transports"`
	BackupEligible  bool       `json:"backup_eligible"`
	BackupState     bool       `json:"backup_state"`
	Nickname        string     `json:"nickname"`
	LastUsedAt      *time.Time `json:"last_used_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func NewWebAuthnCredential() *WebAuthnCredential {
	return &WebAuthnCredential{
		Transports: []string{},
	}
}

// NewWebAuthnCredentialFromLibrary converts the credential the
// go-webauthn library hands back at the end of a registration
// ceremony into the row we persist.
//
// userID is stored because discoverable login returns a user handle,
// and the handle we set at registration time is derived from it.
// Keeping it on the row lets the login path verify handle and
// credential agree without a second lookup.
//
// The nickname is derived, not supplied. It has to be: the AAGUID and
// transports it is derived from only exist once the ceremony has
// finished, which is also the first moment the person could have known
// what they were naming.
func NewWebAuthnCredentialFromLibrary(
	cred *webauthn.Credential,
	userID int64,
) *WebAuthnCredential {
	c := NewWebAuthnCredential()

	c.UserID = userID

	c.CredentialID = cred.ID
	c.PublicKey = cred.PublicKey
	c.AttestationType = cred.AttestationType
	c.AAGUID = cred.Authenticator.AAGUID
	c.SignCount = cred.Authenticator.SignCount
	c.CloneWarning = cred.Authenticator.CloneWarning
	c.BackupEligible = cred.Flags.BackupEligible
	c.BackupState = cred.Flags.BackupState

	c.Transports = make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		c.Transports = append(c.Transports, string(t))
	}

	// Last, because DeriveNickname reads AAGUID, Transports, and the
	// backup flags — all of which are set above.
	c.Nickname = c.DeriveNickname()

	return c
}

// ToLibrary converts the stored row back into the shape go-webauthn
// expects when verifying an assertion.
func (c *WebAuthnCredential) ToLibrary() webauthn.Credential {
	return webauthn.Credential{
		ID:              c.CredentialID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Transport:       c.libraryTransports(),
		Flags: webauthn.CredentialFlags{
			BackupEligible: c.BackupEligible,
			BackupState:    c.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       c.AAGUID,
			SignCount:    c.SignCount,
			CloneWarning: c.CloneWarning,
		},
	}
}

// Descriptor is what gets passed to the browser as an exclusion at
// registration time, so the same authenticator can't be enrolled
// twice and end up as two indistinguishable rows in the person's
// credentials list.
func (c *WebAuthnCredential) Descriptor() protocol.CredentialDescriptor {
	return protocol.CredentialDescriptor{
		Type:         protocol.PublicKeyCredentialType,
		CredentialID: c.CredentialID,
		Transport:    c.libraryTransports(),
	}
}

// libraryTransports converts the stored strings to the library's
// transport type. Shared by ToLibrary and Descriptor, which had
// identical copies of this loop.
func (c *WebAuthnCredential) libraryTransports() []protocol.AuthenticatorTransport {
	transports := make([]protocol.AuthenticatorTransport, 0, len(c.Transports))
	for _, t := range c.Transports {
		transports = append(transports, protocol.AuthenticatorTransport(t))
	}
	return transports
}

// WebAuthnUserHandle encodes a person's ID as the user handle stored
// on the authenticator and returned to us during discoverable login.
//
// THE ENCODING HERE IS PERMANENT.
//
// Eight bytes, big-endian. The spec allows up to 64 and the decimal
// text form would fit too, but the choice is written into every
// credential at enrolment and cannot be changed afterwards — switch it
// later and every existing passkey stops resolving to a person, with
// no error the person can act on.
//
// Encode and decode live here as a pair, and both the registration
// path (services.WebAuthnUser.WebAuthnID) and the login path go
// through them, so the two cannot drift apart on the format.
func WebAuthnUserHandle(userID int64) []byte {
	handle := make([]byte, 8)
	binary.BigEndian.PutUint64(handle, uint64(userID))
	return handle
}

// UserIDFromWebAuthnHandle reverses WebAuthnUserHandle.
//
// Length is checked rather than assumed so a handle from some other
// relying party — or from a credential enrolled under an older
// encoding — fails with a reason, instead of silently decoding to some
// other person's ID.
func UserIDFromWebAuthnHandle(handle []byte) (int64, error) {
	if len(handle) != 8 {
		return 0, fmt.Errorf("webauthn: user handle is %d bytes, want 8", len(handle))
	}
	return int64(binary.BigEndian.Uint64(handle)), nil
}

// UserHandle returns the bytes stored on the authenticator as this
// credential's user handle.
func (c *WebAuthnCredential) UserHandle() []byte {
	return WebAuthnUserHandle(c.UserID)
}

// ApplyAuthenticatorUpdate folds post-login authenticator state back
// onto the row.
//
// Two things change on a successful assertion: the signature counter,
// and the backup state (a passkey created on one device can later
// sync to a cloud keychain, flipping BS from false to true). Reports
// the caller whether anything actually moved, so a login against an
// unchanged synced credential doesn't cost a write.
func (c *WebAuthnCredential) ApplyAuthenticatorUpdate(cred *webauthn.Credential) bool {
	changed := false

	if cred.Authenticator.SignCount != c.SignCount {
		c.SignCount = cred.Authenticator.SignCount
		changed = true
	}

	// The library sets CloneWarning when the counter went backwards.
	// Latch it on: once we've seen evidence of a clone we keep showing
	// it until the person revokes the credential.
	if cred.Authenticator.CloneWarning && !c.CloneWarning {
		c.CloneWarning = true
		changed = true
	}

	if cred.Flags.BackupState != c.BackupState {
		c.BackupState = cred.Flags.BackupState
		changed = true
	}

	if cred.Flags.BackupEligible != c.BackupEligible {
		c.BackupEligible = cred.Flags.BackupEligible
		changed = true
	}

	return changed
}

// IsSynced reports whether this passkey is backed up to a cloud
// keychain, and therefore survives the person losing the device it
// was created on. Drives the "Synced" / "This device only" label in
// the credentials list.
func (c *WebAuthnCredential) IsSynced() bool {
	return c.BackupEligible && c.BackupState
}
