package permissions

import (
	"strings"
	"testing"
	"time"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
	"auth/data/entities"
)

// Permission checks decide who may read, change and destroy accounts, and
// until now none of them had a test. They are pure functions over a viewer and
// a target, which makes them the cheapest security-relevant thing in the
// service to pin down — and the most expensive to get quietly wrong, because a
// check that returns nil for the wrong person fails open and silently.
//
// Two properties recur and are asserted everywhere they apply:
//
//   - scope is checked BEFORE role, so a token without the scope is refused
//     even when the person behind it is an administrator;
//   - acting on somebody ELSE always needs more than acting on yourself.

const (
	viewerID = int64(42)
	otherID  = int64(99)
)

// newViewer builds an authenticated viewer with the given type and scopes.
func newViewer(id int64, userType string, scopes ...string) *libs_entities.Viewer {
	v := libs_entities.NewViewer()
	v.ID = id
	v.PersonType = userType
	v.Authenticated = true
	v.Scopes = scopes
	return v
}

func newUser(id int64) *entities.User {
	u := entities.NewUser()
	u.ID = id
	return u
}

// scopeless returns every scope EXCEPT the one named, so a test can prove a
// check fails on the absence of that specific scope rather than on having no
// scopes at all.
func scopeless(except string) []string {
	all := []string{
		libs_constants.OAuthScopeAccountRead,
		libs_constants.OAuthScopeAccountWrite,
		libs_constants.OAuthScopeActorsRead,
		libs_constants.OAuthScopeActorsWrite,
	}

	out := make([]string, 0, len(all))
	for _, s := range all {
		if s != except {
			out = append(out, s)
		}
	}
	return out
}

// ---------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------

func TestUsersCanEdit(t *testing.T) {
	p := NewUsersPermissions(&Env{})
	target := newUser(otherID)
	self := newUser(viewerID)

	cases := []struct {
		name   string
		viewer *libs_entities.Viewer
		target *entities.User
		want   bool
	}{
		{
			name:   "self with account:write",
			viewer: newViewer(viewerID, libs_constants.PersonTypeRegistered, libs_constants.OAuthScopeAccountWrite),
			target: self,
			want:   true,
		},
		{
			// Editing your own account is gated on account:write. Holding
			// actors:write instead is not a substitute — a token scoped for
			// managing OTHER people should not silently manage your own.
			name:   "self without account:write",
			viewer: newViewer(viewerID, libs_constants.PersonTypeRegistered, scopeless(libs_constants.OAuthScopeAccountWrite)...),
			target: self,
			want:   false,
		},
		{
			name:   "ordinary person editing somebody else",
			viewer: newViewer(viewerID, libs_constants.PersonTypeRegistered, libs_constants.OAuthScopeActorsWrite),
			target: target,
			want:   false,
		},
		{
			name:   "admin editing somebody else",
			viewer: newViewer(viewerID, libs_constants.PersonTypeAdmin, libs_constants.OAuthScopeActorsWrite),
			target: target,
			want:   true,
		},
		{
			name:   "super admin editing somebody else",
			viewer: newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, libs_constants.OAuthScopeActorsWrite),
			target: target,
			want:   true,
		},
		{
			// Scope before role: an administrator whose token was not granted
			// actors:write is still refused. Without this ordering the scope
			// system would be decorative for anyone holding a role.
			name:   "admin without actors:write",
			viewer: newViewer(viewerID, libs_constants.PersonTypeAdmin, scopeless(libs_constants.OAuthScopeActorsWrite)...),
			target: target,
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := p.CanEdit(tc.viewer, tc.target)
			assertPermission(t, ok, err, tc.want)
		})
	}
}

func TestUsersCanDelete(t *testing.T) {
	p := NewUsersPermissions(&Env{})

	t.Run("self", func(t *testing.T) {
		v := newViewer(viewerID, libs_constants.PersonTypeRegistered, libs_constants.OAuthScopeAccountWrite)
		ok, err := p.CanDelete(v, newUser(viewerID))
		assertPermission(t, ok, err, true)
	})

	// The self-branch is guarded on viewer.ID > 0, which matters because an
	// unauthenticated viewer has ID 0. Without the guard, a zero-id viewer
	// would match a zero-id target and delete it as "their own".
	t.Run("zero id viewer does not match a zero id user", func(t *testing.T) {
		v := newViewer(0, libs_constants.PersonTypeGuest, libs_constants.OAuthScopeAccountWrite)
		ok, err := p.CanDelete(v, newUser(0))
		assertPermission(t, ok, err, false)
	})

	t.Run("ordinary person deleting somebody else", func(t *testing.T) {
		v := newViewer(viewerID, libs_constants.PersonTypeRegistered, libs_constants.OAuthScopeActorsWrite)
		ok, err := p.CanDelete(v, newUser(otherID))
		assertPermission(t, ok, err, false)
	})

	t.Run("admin deleting somebody else", func(t *testing.T) {
		v := newViewer(viewerID, libs_constants.PersonTypeAdmin, libs_constants.OAuthScopeActorsWrite)
		ok, err := p.CanDelete(v, newUser(otherID))
		assertPermission(t, ok, err, true)
	})
}

func TestUsersCanBrowseIsAdminOnly(t *testing.T) {
	p := NewUsersPermissions(&Env{})

	for _, tc := range []struct {
		userType string
		want     bool
	}{
		{libs_constants.PersonTypeRegistered, false},
		{libs_constants.PersonTypeAdmin, true},
		{libs_constants.PersonTypeSuperAdmin, true},
	} {
		t.Run(tc.userType, func(t *testing.T) {
			v := newViewer(viewerID, tc.userType, libs_constants.OAuthScopeActorsWrite)
			ok, err := p.CanBrowse(v)
			assertPermission(t, ok, err, tc.want)
		})
	}
}

// Reading your own account uses account:read; reading somebody else's uses
// actors:write. A viewer holding only the first must not reach the second.
func TestUsersCanReadSeparatesSelfFromOthers(t *testing.T) {
	p := NewUsersPermissions(&Env{})

	v := newViewer(viewerID, libs_constants.PersonTypeRegistered, libs_constants.OAuthScopeAccountRead)

	ok, err := p.CanRead(v, newUser(viewerID))
	assertPermission(t, ok, err, true)

	ok, err = p.CanRead(v, newUser(otherID))
	assertPermission(t, ok, err, false)
}

// ---------------------------------------------------------------------
// TOTP
// ---------------------------------------------------------------------

// Disabling 2FA weakens an account, and is the one place admins were
// deliberately widened: recovering somebody who lost their authenticator and
// their recovery codes should not queue behind a single super-administrator.
func TestTotpCanDelete(t *testing.T) {
	p := NewTotpPerm(&Env{})

	cases := []struct {
		name     string
		viewerID int64
		userType string
		scopes   []string
		targetID int64
		wantErr  bool
	}{
		{
			// Self-service needs no scope check at all — the branch returns
			// before reaching one.
			name:     "self",
			viewerID: viewerID,
			userType: libs_constants.PersonTypeRegistered,
			targetID: viewerID,
			wantErr:  false,
		},
		{
			name:     "admin disabling somebody else",
			viewerID: viewerID,
			userType: libs_constants.PersonTypeAdmin,
			scopes:   []string{libs_constants.OAuthScopeActorsWrite},
			targetID: otherID,
			wantErr:  false,
		},
		{
			name:     "super admin disabling somebody else",
			viewerID: viewerID,
			userType: libs_constants.PersonTypeSuperAdmin,
			scopes:   []string{libs_constants.OAuthScopeActorsWrite},
			targetID: otherID,
			wantErr:  false,
		},
		{
			name:     "ordinary person disabling somebody else",
			viewerID: viewerID,
			userType: libs_constants.PersonTypeRegistered,
			scopes:   []string{libs_constants.OAuthScopeActorsWrite},
			targetID: otherID,
			wantErr:  true,
		},
		{
			name:     "admin without actors:write",
			viewerID: viewerID,
			userType: libs_constants.PersonTypeAdmin,
			scopes:   []string{libs_constants.OAuthScopeAccountWrite},
			targetID: otherID,
			wantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := newViewer(tc.viewerID, tc.userType, tc.scopes...)
			err := p.CanDelete(v, newUser(tc.targetID))

			if tc.wantErr && err == nil {
				t.Error("expected refusal, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected permission, got %v", err)
			}
		})
	}
}

// Enrolling, editing and generating recovery codes are self-only. There is no
// admin path: an administrator who could enrol 2FA on somebody else's account
// would hold that account's second factor.
func TestTotpMutationsAreSelfOnly(t *testing.T) {
	p := NewTotpPerm(&Env{})
	admin := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin,
		libs_constants.OAuthScopeActorsWrite, libs_constants.OAuthScopeAccountWrite)

	checks := map[string]func(*libs_entities.Viewer, *entities.User) error{
		"CanAdd":              p.CanAdd,
		"CanEdit":             p.CanEdit,
		"CanAddRecoveryCodes": p.CanAddRecoveryCodes,
	}

	for name, check := range checks {
		t.Run(name+"/self", func(t *testing.T) {
			if err := check(admin, newUser(viewerID)); err != nil {
				t.Errorf("self was refused: %v", err)
			}
		})
		t.Run(name+"/somebody else, even as super admin", func(t *testing.T) {
			if err := check(admin, newUser(otherID)); err == nil {
				t.Error("a super admin was allowed to act on another account's second factor")
			}
		})
	}
}

// ---------------------------------------------------------------------
// Auth logs
// ---------------------------------------------------------------------

func TestAuthLogCanBrowse(t *testing.T) {
	p := NewAuthLogPerm(&Env{})

	t.Run("own logs", func(t *testing.T) {
		v := newViewer(viewerID, libs_constants.PersonTypeRegistered, libs_constants.OAuthScopeAccountRead)
		if err := p.CanBrowse(v, newUser(viewerID)); err != nil {
			t.Errorf("refused own logs: %v", err)
		}
	})

	// Auth logs hold IP addresses, cities and device fingerprints — a record of
	// where somebody has been. Reading another person's is admin-only.
	t.Run("somebody else's logs as an ordinary person", func(t *testing.T) {
		v := newViewer(viewerID, libs_constants.PersonTypeRegistered, libs_constants.OAuthScopeAccountRead)
		if err := p.CanBrowse(v, newUser(otherID)); err == nil {
			t.Error("an ordinary person read another person's location history")
		}
	})

	t.Run("somebody else's logs as an admin", func(t *testing.T) {
		v := newViewer(viewerID, libs_constants.PersonTypeAdmin, libs_constants.OAuthScopeAccountRead)
		if err := p.CanBrowse(v, newUser(otherID)); err != nil {
			t.Errorf("admin was refused: %v", err)
		}
	})

	t.Run("without account:read", func(t *testing.T) {
		v := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, scopeless(libs_constants.OAuthScopeAccountRead)...)
		if err := p.CanBrowse(v, newUser(viewerID)); err == nil {
			t.Error("scope was not checked")
		}
	})
}

// Deleting an auth log force-logs-out that session, so it is gated on write
// rather than read — and an admin may do it to somebody else.
func TestAuthLogCanDeleteRequiresWriteScope(t *testing.T) {
	p := NewAuthLogPerm(&Env{})

	readOnly := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, libs_constants.OAuthScopeAccountRead)
	if err := p.CanDelete(readOnly, newUser(viewerID)); err == nil {
		t.Error("a read-only token ended a session")
	}

	writer := newViewer(viewerID, libs_constants.PersonTypeAdmin, libs_constants.OAuthScopeAccountWrite)
	if err := p.CanDelete(writer, newUser(otherID)); err != nil {
		t.Errorf("admin was refused: %v", err)
	}
}

// An unsaved user has no id, and an auth log belonging to nobody is a row that
// can never be browsed or revoked.
func TestAuthLogCanAddRejectsAnUnsavedUser(t *testing.T) {
	p := NewAuthLogPerm(&Env{})

	if err := p.CanAdd(newUser(0)); err == nil {
		t.Error("accepted an auth log for a user with no id")
	}
	if err := p.CanAdd(newUser(viewerID)); err != nil {
		t.Errorf("refused a saved user: %v", err)
	}
}

// ---------------------------------------------------------------------
// Passkeys
// ---------------------------------------------------------------------

// Ownership is checked against the stored row, not against the id in the path,
// which is person-supplied.
func TestWebAuthnMutationChecksOwnership(t *testing.T) {
	p := NewWebAuthnPerm(&Env{})
	v := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, libs_constants.OAuthScopeAccountWrite)

	mine := &entities.WebAuthnCredential{UserID: viewerID}
	theirs := &entities.WebAuthnCredential{UserID: otherID}

	if ok, err := p.CanDelete(v, mine); !ok {
		t.Errorf("refused my own passkey: %v", err)
	}

	// Even a super administrator: the single-credential endpoints are
	// self-only, and bulk revocation is the separate, heavier path.
	if ok, _ := p.CanDelete(v, theirs); ok {
		t.Error("a super admin deleted another person's passkey through the self endpoint")
	}

	// A missing row and somebody else's row must be indistinguishable, or the
	// endpoint becomes an oracle for whether a credential id exists.
	okMissing, errMissing := p.CanDelete(v, nil)
	okTheirs, errTheirs := p.CanDelete(v, theirs)
	if okMissing || okTheirs {
		t.Fatal("expected both to be refused")
	}
	if errMissing.Error() != errTheirs.Error() {
		t.Errorf("a missing credential (%v) is distinguishable from somebody else's (%v)",
			errMissing, errTheirs)
	}
}

func TestWebAuthnRejectsUnauthenticatedViewers(t *testing.T) {
	p := NewWebAuthnPerm(&Env{})

	guest := libs_entities.NewViewer()
	guest.Authenticated = false
	guest.Scopes = []string{libs_constants.OAuthScopeAccountWrite, libs_constants.OAuthScopeAccountRead}

	if ok, _ := p.CanBrowse(guest); ok {
		t.Error("an unauthenticated viewer listed passkeys")
	}
	if ok, _ := p.CanAdd(guest); ok {
		t.Error("an unauthenticated viewer enrolled a passkey")
	}
	if ok, _ := p.CanBrowse(nil); ok {
		t.Error("a nil viewer was accepted")
	}
}

// Bulk revocation against somebody else is super-admin only AND requires a
// recent TOTP verification — a global admin session that has been sitting open
// is not by itself authority to strip credentials.
func TestWebAuthnCanDeleteAll(t *testing.T) {
	p := NewWebAuthnPerm(&Env{})
	v := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, libs_constants.OAuthScopeAccountWrite)

	freshSuper := newUser(viewerID)
	freshSuper.PersonType = libs_constants.PersonTypeSuperAdmin
	freshSuper.TotpEnabled = true
	freshSuper.TotpLastVerifiedAt = time.Now()

	t.Run("own account needs no elevation", func(t *testing.T) {
		self := newUser(viewerID)
		if ok, err := p.CanDeleteAll(v, self, self); !ok {
			t.Errorf("refused bulk revocation on own account: %v", err)
		}
	})

	t.Run("super admin with fresh totp", func(t *testing.T) {
		if ok, err := p.CanDeleteAll(v, freshSuper, newUser(otherID)); !ok {
			t.Errorf("refused: %v", err)
		}
	})

	t.Run("ordinary admin is not enough", func(t *testing.T) {
		admin := newUser(viewerID)
		admin.PersonType = libs_constants.PersonTypeAdmin
		admin.TotpEnabled = true
		admin.TotpLastVerifiedAt = time.Now()

		ok, err := p.CanDeleteAll(v, admin, newUser(otherID))
		if ok {
			t.Error("an ordinary admin stripped another person's passkeys")
		}
		if !strings.Contains(err.Error(), "super admin") {
			t.Errorf("error %q does not explain the requirement", err)
		}
	})

	t.Run("super admin without totp", func(t *testing.T) {
		noTotp := newUser(viewerID)
		noTotp.PersonType = libs_constants.PersonTypeSuperAdmin

		if ok, _ := p.CanDeleteAll(v, noTotp, newUser(otherID)); ok {
			t.Error("a super admin with no second factor stripped somebody's passkeys")
		}
	})

	t.Run("super admin with a stale totp verification", func(t *testing.T) {
		stale := newUser(viewerID)
		stale.PersonType = libs_constants.PersonTypeSuperAdmin
		stale.TotpEnabled = true
		stale.TotpLastVerifiedAt = time.Now().Add(-365 * 24 * time.Hour)

		// Freshness, not enrolment: "has TOTP configured" is a claim about the
		// account, "verified recently" is a claim about who is at the keyboard.
		if ok, _ := p.CanDeleteAll(v, stale, newUser(otherID)); ok {
			t.Error("a long-idle super admin session stripped somebody's passkeys")
		}
	})

	t.Run("an unhydrated viewer is refused rather than defaulted", func(t *testing.T) {
		if ok, _ := p.CanDeleteAll(v, nil, newUser(otherID)); ok {
			t.Error("accepted a nil actor")
		}
		if ok, _ := p.CanDeleteAll(v, freshSuper, nil); ok {
			t.Error("accepted a nil target")
		}
	})
}

// Reading a count is deliberately NOT gated on TOTP freshness: it populates a
// card rather than destroying anything, and a count is not a list.
func TestWebAuthnCanReadCountDoesNotRequireFreshTotp(t *testing.T) {
	p := NewWebAuthnPerm(&Env{})
	v := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, libs_constants.OAuthScopeAccountWrite)

	stale := newUser(viewerID)
	stale.PersonType = libs_constants.PersonTypeSuperAdmin

	if ok, err := p.CanReadCount(v, stale, newUser(otherID)); !ok {
		t.Errorf("a super admin with a stale session could not read a count: %v", err)
	}

	// Still super-admin only, though.
	admin := newUser(viewerID)
	admin.PersonType = libs_constants.PersonTypeAdmin
	if ok, _ := p.CanReadCount(v, admin, newUser(otherID)); ok {
		t.Error("an ordinary admin read another person's passkey count")
	}
}

// ---------------------------------------------------------------------
// Invites and signup
// ---------------------------------------------------------------------

func TestInvitePermissionsAreAdminOnly(t *testing.T) {
	p := NewInvitePerm(&Env{})

	registered := newViewer(viewerID, libs_constants.PersonTypeRegistered,
		libs_constants.OAuthScopeActorsRead, libs_constants.OAuthScopeActorsWrite)
	admin := newViewer(viewerID, libs_constants.PersonTypeAdmin,
		libs_constants.OAuthScopeActorsRead, libs_constants.OAuthScopeActorsWrite)

	for name, check := range map[string]func(*libs_entities.Viewer) error{
		"CanBrowse": p.CanBrowse,
		"CanAdd":    p.CanAdd,
		"CanDelete": p.CanDelete,
	} {
		t.Run(name, func(t *testing.T) {
			if err := check(registered); err == nil {
				t.Error("an ordinary person was allowed")
			}
			if err := check(admin); err != nil {
				t.Errorf("an admin was refused: %v", err)
			}
		})
	}
}

// Browse reads; add and delete write. A read-only token must not create or
// revoke an invitation.
func TestInviteSeparatesReadFromWrite(t *testing.T) {
	p := NewInvitePerm(&Env{})
	readOnly := newViewer(viewerID, libs_constants.PersonTypeAdmin, libs_constants.OAuthScopeActorsRead)

	if err := p.CanBrowse(readOnly); err != nil {
		t.Errorf("browse was refused to a read scope: %v", err)
	}
	if err := p.CanAdd(readOnly); err == nil {
		t.Error("a read-only token created an invitation")
	}
	if err := p.CanDelete(readOnly); err == nil {
		t.Error("a read-only token revoked an invitation")
	}
}

func TestSignupIsForGuestsOnly(t *testing.T) {
	p := NewSignupPermissions(&Env{})

	guest := libs_entities.NewViewer()
	guest.PersonType = libs_constants.PersonTypeGuest

	if ok, err := p.CanAdd(guest, nil); !ok {
		t.Errorf("a guest could not sign up: %v", err)
	}
	if ok, err := p.CanAdd(nil, nil); !ok {
		t.Errorf("a nil viewer could not sign up: %v", err)
	}

	signedIn := newViewer(viewerID, libs_constants.PersonTypeRegistered)
	if ok, _ := p.CanAdd(signedIn, nil); ok {
		t.Error("a signed-in person created a second account")
	}
}

// ---------------------------------------------------------------------

func assertPermission(t *testing.T, ok bool, err error, want bool) {
	t.Helper()

	if ok != want {
		t.Errorf("permission = %v, want %v (err: %v)", ok, want, err)
	}
	// A refusal must say why: these errors reach the handler, which maps them
	// onto a status code.
	if !ok && err == nil {
		t.Error("refused with no error to explain it")
	}
	if ok && err != nil {
		t.Errorf("allowed but returned an error: %v", err)
	}
}
