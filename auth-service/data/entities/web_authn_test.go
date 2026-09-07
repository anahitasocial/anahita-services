package entities

import (
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// The WebAuthn user handle is stored ON the authenticator at registration and
// read back at login. It is the one encoding in the system that cannot be
// changed after the fact: alter it and every enrolled passkey stops resolving
// to a person, with no error anybody can act on.

func TestUserHandleRoundTrip(t *testing.T) {
	for _, id := range []int64{1, 42, 1 << 20, 1<<63 - 1} {
		handle := WebAuthnUserHandle(id)

		if len(handle) != 8 {
			t.Errorf("id %d produced a %d-byte handle, want 8", id, len(handle))
		}

		got, err := UserIDFromWebAuthnHandle(handle)
		if err != nil {
			t.Fatalf("id %d: decode: %v", id, err)
		}
		if got != id {
			t.Errorf("round trip: got %d, want %d", got, id)
		}
	}
}

// Big-endian, fixed 8 bytes. Pinned explicitly because the format is a
// compatibility contract with hardware already in people's pockets, not an
// implementation detail free to change.
func TestUserHandleEncodingIsBigEndianEightBytes(t *testing.T) {
	handle := WebAuthnUserHandle(1)

	want := []byte{0, 0, 0, 0, 0, 0, 0, 1}
	if len(handle) != len(want) {
		t.Fatalf("got %d bytes, want %d", len(handle), len(want))
	}
	for i := range want {
		if handle[i] != want[i] {
			t.Fatalf("handle = %v, want %v — changing this orphans every enrolled passkey",
				handle, want)
		}
	}
}

// A handle of the wrong length is from another relying party, or from an older
// encoding. It must fail loudly rather than decode to somebody else's id.
func TestUserHandleRejectsWrongLengths(t *testing.T) {
	for _, handle := range [][]byte{
		nil,
		{},
		{1, 2, 3, 4},
		{1, 2, 3, 4, 5, 6, 7, 8, 9},
	} {
		if _, err := UserIDFromWebAuthnHandle(handle); err == nil {
			t.Errorf("accepted a %d-byte handle", len(handle))
		}
	}
}

func TestCredentialUserHandleMatchesTheHelper(t *testing.T) {
	c := NewWebAuthnCredential()
	c.UserID = 42

	got, err := UserIDFromWebAuthnHandle(c.UserHandle())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
}

// ---------------------------------------------------------------------
// Sync state
// ---------------------------------------------------------------------

// Synced means the passkey survives losing the device. Both flags are needed:
// eligible-but-not-backed-up is a passkey that COULD sync and has not, which
// is still device-bound.
func TestIsSyncedRequiresBothFlags(t *testing.T) {
	cases := []struct {
		eligible bool
		state    bool
		want     bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}

	for _, tc := range cases {
		c := NewWebAuthnCredential()
		c.BackupEligible = tc.eligible
		c.BackupState = tc.state

		if got := c.IsSynced(); got != tc.want {
			t.Errorf("eligible=%v state=%v: IsSynced()=%v, want %v",
				tc.eligible, tc.state, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------
// Post-login authenticator updates
// ---------------------------------------------------------------------

// The return value decides whether a login costs a database write, so it has
// to be false when genuinely nothing moved.
func TestApplyAuthenticatorUpdateReportsRealChanges(t *testing.T) {
	c := NewWebAuthnCredential()
	c.SignCount = 5
	c.BackupEligible = true
	c.BackupState = true

	unchanged := &webauthn.Credential{}
	unchanged.Authenticator.SignCount = 5
	unchanged.Flags.BackupEligible = true
	unchanged.Flags.BackupState = true

	if c.ApplyAuthenticatorUpdate(unchanged) {
		t.Error("reported a change when nothing moved; every login would write")
	}

	moved := &webauthn.Credential{}
	moved.Authenticator.SignCount = 6
	moved.Flags.BackupEligible = true
	moved.Flags.BackupState = true

	if !c.ApplyAuthenticatorUpdate(moved) {
		t.Error("a counter increment was not reported as a change")
	}
	if c.SignCount != 6 {
		t.Errorf("SignCount = %d, want 6", c.SignCount)
	}
}

// A passkey created on one device can later sync to a cloud keychain, which
// flips BackupState. The row has to follow, or the UI keeps calling a synced
// passkey device-bound.
func TestApplyAuthenticatorUpdateTracksBackupState(t *testing.T) {
	c := NewWebAuthnCredential()
	c.BackupEligible = true
	c.BackupState = false

	cred := &webauthn.Credential{}
	cred.Flags.BackupEligible = true
	cred.Flags.BackupState = true

	if !c.ApplyAuthenticatorUpdate(cred) {
		t.Error("a backup-state flip was not reported")
	}
	if !c.IsSynced() {
		t.Error("the credential did not become synced")
	}
}

// The clone warning latches. Once the counter has gone backwards, the evidence
// stays visible until the person revokes the credential — a later clean login
// must not quietly clear it.
func TestCloneWarningLatchesOn(t *testing.T) {
	c := NewWebAuthnCredential()

	warned := &webauthn.Credential{}
	warned.Authenticator.CloneWarning = true

	if !c.ApplyAuthenticatorUpdate(warned) {
		t.Error("the first clone warning was not recorded")
	}
	if !c.CloneWarning {
		t.Fatal("CloneWarning was not set")
	}

	clean := &webauthn.Credential{}
	clean.Authenticator.CloneWarning = false

	c.ApplyAuthenticatorUpdate(clean)
	if !c.CloneWarning {
		t.Error("a subsequent clean login cleared the clone warning")
	}
}

// ---------------------------------------------------------------------
// Nickname derivation
// ---------------------------------------------------------------------

// The default name is derived at registration so it matches what the browser's
// own picker showed. A person who renames afterwards has said what they want,
// which is why this must never run again on a later path.
func TestDeriveNicknameFromAAGUID(t *testing.T) {
	// iCloud Keychain, from the vendor map.
	aaguid := []byte{
		0xfb, 0xfc, 0x30, 0x07, 0x15, 0x4e, 0x4e, 0xcc,
		0x8c, 0x0b, 0x6e, 0x02, 0x05, 0x57, 0xd7, 0xbd,
	}

	c := NewWebAuthnCredential()
	c.AAGUID = aaguid

	if got := c.DeriveNickname(); got != "iCloud Keychain" {
		t.Errorf("DeriveNickname() = %q, want iCloud Keychain", got)
	}
}

// An all-zero AAGUID is common and deliberate — many authenticators report it
// to avoid being fingerprinted — so it must fall through to the heuristics
// rather than being treated as a lookup miss worth surfacing.
func TestDeriveNicknameFallsBackToTransports(t *testing.T) {
	cases := []struct {
		name       string
		transports []string
		want       string
	}{
		{"hybrid means a phone", []string{"hybrid"}, "Phone or tablet"},
		{"usb means a security key", []string{"usb"}, "Security key"},
		{"nfc means a security key", []string{"nfc"}, "Security key"},
		{"ble means a security key", []string{"ble"}, "Security key"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewWebAuthnCredential()
			c.AAGUID = make([]byte, 16) // all zero
			c.Transports = tc.transports

			if got := c.DeriveNickname(); got != tc.want {
				t.Errorf("DeriveNickname() = %q, want %q", got, tc.want)
			}
		})
	}
}

// With nothing else to go on, synced vs device-bound is still the distinction
// that matters most to the person.
func TestDeriveNicknameFallsBackToSyncState(t *testing.T) {
	synced := NewWebAuthnCredential()
	synced.AAGUID = make([]byte, 16)
	synced.BackupEligible = true
	synced.BackupState = true

	if got := synced.DeriveNickname(); got != "Synced passkey" {
		t.Errorf("synced: got %q, want Synced passkey", got)
	}

	bound := NewWebAuthnCredential()
	bound.AAGUID = make([]byte, 16)

	if got := bound.DeriveNickname(); got != "This device" {
		t.Errorf("device-bound: got %q, want This device", got)
	}
}

// A malformed AAGUID must not panic — it arrives from an authenticator, which
// is to say from outside.
func TestDeriveNicknameSurvivesAMalformedAAGUID(t *testing.T) {
	for _, aaguid := range [][]byte{nil, {}, {1, 2, 3}, make([]byte, 32)} {
		c := NewWebAuthnCredential()
		c.AAGUID = aaguid

		if got := c.DeriveNickname(); got == "" {
			t.Errorf("a %d-byte AAGUID produced an empty nickname", len(aaguid))
		}
	}
}

// ---------------------------------------------------------------------
// Auth logs
// ---------------------------------------------------------------------

// An auth log with no owner can never be browsed or revoked, and the other
// fields are what make a row recognisable in the sessions list.
func TestAuthLogValidate(t *testing.T) {
	complete := func() *AuthLog {
		a := NewAuthLog()
		a.CreatedBy = 42
		a.Device = "laptop"
		a.OS = "Linux"
		a.Name = "Firefox"
		a.IP = "203.0.113.9"
		a.City = "Vancouver"
		a.Country = "Canada"
		a.OAuthRefreshTokenID = 7
		a.ClientID = "anahita-web"
		a.CreatedOn = time.Now()
		return a
	}

	if err := complete().Validate(); err != nil {
		t.Fatalf("a complete auth log was rejected: %v", err)
	}

	cases := map[string]func(*AuthLog){
		"no owner":   func(a *AuthLog) { a.CreatedBy = 0 },
		"no device":  func(a *AuthLog) { a.Device = "" },
		"no os":      func(a *AuthLog) { a.OS = "" },
		"no name":    func(a *AuthLog) { a.Name = "" },
		"no ip":      func(a *AuthLog) { a.IP = "" },
		"no city":    func(a *AuthLog) { a.City = "" },
		"no country": func(a *AuthLog) { a.Country = "" },
		// A log with no refresh token points at no session, so nothing can
		// ever revoke it from the sessions list.
		"no refresh token": func(a *AuthLog) { a.OAuthRefreshTokenID = 0 },
		"no client":        func(a *AuthLog) { a.ClientID = "" },
	}

	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			a := complete()
			break_(a)
			if err := a.Validate(); err == nil {
				t.Errorf("accepted an auth log with %s", name)
			}
		})
	}
}
