package permissions

import (
	"errors"
	"strings"
	"testing"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"

	"auth/config"
	auth_constants "auth/constants"
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
// Revoking another person's passkeys is PEOPLE administration, so it
// admits administrators — authority here divides by domain rather than
// seniority, with super admins owning system configuration and site
// admins owning people and actors in full.
//
// Freshness is no longer asserted here because it is no longer decided
// here. It moved to the handler as requireStepUp, which accepts a
// passkey, a passcode or a password within five minutes. The check this
// replaced demanded TOTP specifically, so an administrator carrying
// only a passkey could not revoke anybody's — the same trap the OAuth
// endpoints had.
func TestWebAuthnCanDeleteAll(t *testing.T) {
	p := NewWebAuthnPerm(&Env{})
	v := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, libs_constants.OAuthScopeAccountWrite)

	actorOfType := func(userType string) *entities.User {
		u := newUser(viewerID)
		u.PersonType = userType
		return u
	}

	t.Run("own account needs no elevation", func(t *testing.T) {
		self := newUser(viewerID)
		if ok, err := p.CanDeleteAll(v, self, self); !ok {
			t.Errorf("refused bulk revocation on own account: %v", err)
		}
	})

	t.Run("administrator", func(t *testing.T) {
		admin := actorOfType(libs_constants.PersonTypeAdmin)
		if ok, err := p.CanDeleteAll(v, admin, newUser(otherID)); !ok {
			t.Errorf("an administrator was refused: %v", err)
		}
	})

	t.Run("super administrator", func(t *testing.T) {
		super := actorOfType(libs_constants.PersonTypeSuperAdmin)
		if ok, err := p.CanDeleteAll(v, super, newUser(otherID)); !ok {
			t.Errorf("a super administrator was refused: %v", err)
		}
	})

	t.Run("no passkey, no TOTP, still allowed here", func(t *testing.T) {
		// The permission answers authority alone. Presence is the
		// handler's question, and asserting it here would pin the
		// wrong function.
		admin := actorOfType(libs_constants.PersonTypeAdmin)
		admin.TotpEnabled = false

		if ok, err := p.CanDeleteAll(v, admin, newUser(otherID)); !ok {
			t.Errorf("authority was refused for want of a second factor: %v", err)
		}
	})

	t.Run("an ordinary person is refused", func(t *testing.T) {
		registered := actorOfType(libs_constants.PersonTypeRegistered)

		ok, err := p.CanDeleteAll(v, registered, newUser(otherID))
		if ok {
			t.Error("an ordinary person stripped somebody else's passkeys")
		}
		if err == nil || !strings.Contains(err.Error(), "administrator") {
			t.Errorf("error %v does not explain the requirement", err)
		}
	})

	t.Run("an unhydrated viewer is refused rather than defaulted", func(t *testing.T) {
		super := actorOfType(libs_constants.PersonTypeSuperAdmin)

		if ok, _ := p.CanDeleteAll(v, nil, newUser(otherID)); ok {
			t.Error("accepted a nil actor")
		}
		if ok, _ := p.CanDeleteAll(v, super, nil); ok {
			t.Error("accepted a nil target")
		}
	})
}

// Reading a count is deliberately NOT gated on TOTP freshness: it populates a
// card rather than destroying anything, and a count is not a list.
// Reading a count is a read that populates a card, so it carries no
// step-up — an administrator with a long-idle session should still be
// able to render the person page.
//
// Administrator, matching CanDeleteAll. A card an administrator can see
// but not act on, or act on but not see, would be worse than either.
func TestWebAuthnCanReadCount(t *testing.T) {
	p := NewWebAuthnPerm(&Env{})
	v := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, libs_constants.OAuthScopeAccountWrite)

	actorOfType := func(userType string) *entities.User {
		u := newUser(viewerID)
		u.PersonType = userType
		return u
	}

	for _, userType := range []string{
		libs_constants.PersonTypeAdmin,
		libs_constants.PersonTypeSuperAdmin,
	} {
		t.Run(userType, func(t *testing.T) {
			// No TOTP and no recent verification: authority only.
			if ok, err := p.CanReadCount(v, actorOfType(userType), newUser(otherID)); !ok {
				t.Errorf("refused a count to %s: %v", userType, err)
			}
		})
	}

	t.Run("an ordinary person is refused", func(t *testing.T) {
		registered := actorOfType(libs_constants.PersonTypeRegistered)
		if ok, _ := p.CanReadCount(v, registered, newUser(otherID)); ok {
			t.Error("an ordinary person read another person's passkey count")
		}
	})
}

// ---------------------------------------------------------------------
// Invites and signup
// ---------------------------------------------------------------------

// Issuing and revoking somebody else's stay with administrators under
// the default setting.
//
// CanBrowse is NOT in this list any more, and its absence is the point.
// It used to be, because browsing returned every invitation on the
// site — so reading the list was reading who else was being brought in,
// and by whom. The handler now scopes the query to created_by, which
// makes browsing a read of your own outbox rather than of the site's.
// See TestInviteBrowseIsSelfService.
func TestInvitePermissionsAreAdminOnly(t *testing.T) {
	p := NewInvitePerm(&Env{
		Config: &config.Config{
			InvitesFrom: auth_constants.InvitesFromAdministrators,
		},
	})

	registered := newViewer(viewerID, libs_constants.PersonTypeRegistered,
		libs_constants.OAuthScopeActorsRead, libs_constants.OAuthScopeActorsWrite)
	admin := newViewer(viewerID, libs_constants.PersonTypeAdmin,
		libs_constants.OAuthScopeActorsRead, libs_constants.OAuthScopeActorsWrite)

	for name, check := range map[string]func(*libs_entities.Viewer) error{
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

// Browsing is self-service, whatever the invite level is.
//
// The list is the viewer's own — scoped on created_by by the handler —
// so reading it is the same kind of act as reading your own passkeys.
// Gating it on administrator would mean that on a site where ordinary
// people may invite, they could send an invitation and revoke it by id
// but never see it listed.
//
// The safety of this rests ENTIRELY on the scoping. If an unscoped
// browse ever comes back, this test should fail and this permission
// should go back to administrators in the same commit.
func TestInviteBrowseIsSelfService(t *testing.T) {
	registered := newViewer(viewerID, libs_constants.PersonTypeRegistered,
		libs_constants.OAuthScopeActorsRead)
	guest := newViewer(viewerID, libs_constants.PersonTypeGuest,
		libs_constants.OAuthScopeActorsRead)

	for _, level := range []string{
		auth_constants.InvitesFromRegistered,
		auth_constants.InvitesFromAdministrators,
		auth_constants.InvitesFromSuperAdministrators,
	} {
		t.Run(level, func(t *testing.T) {
			p := NewInvitePerm(&Env{
				Config: &config.Config{InvitesFrom: level},
			})

			if err := p.CanBrowse(registered); err != nil {
				t.Errorf("a registered person could not read their own invitations: %v", err)
			}
			if err := p.CanBrowse(guest); err == nil {
				t.Error("a guest was allowed to browse invitations")
			}
		})
	}
}

// The level is a MINIMUM ROLE, so each one admits everybody above it.
//
// The bug this pins down: CanAdd used to open with an unconditional
// `if viewer.IsAdminOrSuperAdmin() { return nil }` before it consulted
// the setting. Under super-administrators that let every ordinary
// administrator through, so the tightest level was the one that did
// nothing.
func TestInviteLevelIsAMinimumRole(t *testing.T) {
	scopes := []string{
		libs_constants.OAuthScopeActorsRead,
		libs_constants.OAuthScopeActorsWrite,
	}

	guest := newViewer(viewerID, libs_constants.PersonTypeGuest, scopes...)
	registered := newViewer(viewerID, libs_constants.PersonTypeRegistered, scopes...)
	admin := newViewer(viewerID, libs_constants.PersonTypeAdmin, scopes...)
	superAdmin := newViewer(viewerID, libs_constants.PersonTypeSuperAdmin, scopes...)

	cases := []struct {
		level   string
		allowed []*libs_entities.Viewer
		refused []*libs_entities.Viewer
	}{
		{
			level:   auth_constants.InvitesFromRegistered,
			allowed: []*libs_entities.Viewer{registered, admin, superAdmin},
			refused: []*libs_entities.Viewer{guest},
		},
		{
			level:   auth_constants.InvitesFromAdministrators,
			allowed: []*libs_entities.Viewer{admin, superAdmin},
			refused: []*libs_entities.Viewer{guest, registered},
		},
		{
			level:   auth_constants.InvitesFromSuperAdministrators,
			allowed: []*libs_entities.Viewer{superAdmin},
			refused: []*libs_entities.Viewer{guest, registered, admin},
		},
		{
			// A mistyped setting admits nobody rather than everybody.
			//
			// The singular of a real level, because that is the typo an
			// operator actually makes — the levels are plural and the
			// person types are singular, so "administrator" is one
			// keystroke from correct and reads correct. A nonsense word
			// would prove the same branch while suggesting Anahita has
			// a role it does not.
			level:   libs_constants.PersonTypeAdmin,
			allowed: nil,
			refused: []*libs_entities.Viewer{guest, registered, admin, superAdmin},
		},
	}

	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			p := NewInvitePerm(&Env{
				Config: &config.Config{InvitesFrom: tc.level},
			})

			for _, viewer := range tc.allowed {
				if err := p.CanAdd(viewer); err != nil {
					t.Errorf("%s was refused: %v", viewer.PersonType, err)
				}
			}
			for _, viewer := range tc.refused {
				if err := p.CanAdd(viewer); err == nil {
					t.Errorf("%s was allowed to invite", viewer.PersonType)
				}
			}
		})
	}
}

// `members` is not a value any more, and must not quietly become one.
//
// It was accepted as an alias for `registered` until this: an earlier
// pass at the setting had introduced it, and Anahita has no person type
// by that name. An unrecognised level ranks above every role, so the
// level admits nobody rather than silently meaning "everybody" — which
// is what an alias quietly reinstated by a future edit would do.
func TestInvitesFromMembersIsNotAValue(t *testing.T) {
	p := NewInvitePerm(&Env{
		Config: &config.Config{InvitesFrom: "members"},
	})

	for _, viewer := range []*libs_entities.Viewer{
		newViewer(viewerID, libs_constants.PersonTypeRegistered,
			libs_constants.OAuthScopeActorsWrite),
		newViewer(viewerID, libs_constants.PersonTypeSuperAdmin,
			libs_constants.OAuthScopeActorsWrite),
	} {
		if err := p.CanAdd(viewer); err == nil {
			t.Errorf("`members` was honoured as a level for %s", viewer.PersonType)
		}
	}
}

// Opening issuing to ordinary people opens issuing, and not moderation.
//
// Revoking SOMEBODY ELSE'S invitation stays with administrators whatever
// the level is: withdrawing your own is your own business, taking back
// another person's is moderation. CanDeleteOwn is the one that admits
// the issuer, and only for the row they created.
//
// The role check is the ONLY gate on any of this. A comment claimed the
// scope catalog gated these as AdminOnly at token-mint time and the role
// check was defence in depth; actors:write is an ordinary scope every
// registered person holds, so that second layer never existed.
func TestOpenIssuingDoesNotOpenModeration(t *testing.T) {
	p := NewInvitePerm(&Env{
		Config: &config.Config{InvitesFrom: auth_constants.InvitesFromRegistered},
	})

	registered := newViewer(viewerID, libs_constants.PersonTypeRegistered,
		libs_constants.OAuthScopeActorsRead, libs_constants.OAuthScopeActorsWrite)

	if err := p.CanAdd(registered); err != nil {
		t.Errorf("a registered person was refused under `registered`: %v", err)
	}

	if err := p.CanDelete(registered); err == nil {
		t.Error("a registered person could revoke somebody else's invitation")
	}

	if err := p.CanDeleteOwn(registered, otherID); err == nil {
		t.Error("a registered person revoked an invitation issued by somebody else")
	}
	if err := p.CanDeleteOwn(registered, viewerID); err != nil {
		t.Errorf("a registered person could not revoke their own invitation: %v", err)
	}
}

// A guest is never admitted, whatever the setting says.
//
// This is the case that caught a bad first cut of the level ranking.
// Person types and configured levels were ranked on one scale whose
// `default` branch sat above every role — right for an unrecognised
// SETTING, which must admit nobody, and exactly backwards for an
// unrecognised ROLE. Guests ranked at the top and every one of them
// could invite.
func TestInviteLevelStillExcludesGuests(t *testing.T) {
	guest := newViewer(viewerID, libs_constants.PersonTypeGuest,
		libs_constants.OAuthScopeActorsWrite)

	for _, level := range []string{
		auth_constants.InvitesFromRegistered,
		auth_constants.InvitesFromAdministrators,
		auth_constants.InvitesFromSuperAdministrators,
	} {
		t.Run(level, func(t *testing.T) {
			p := NewInvitePerm(&Env{
				Config: &config.Config{InvitesFrom: level},
			})

			if err := p.CanAdd(guest); err == nil {
				t.Error("a guest issued an invitation")
			}
		})
	}
}

// Browse reads; add and delete write. A read-only token must not create or
// revoke an invitation.
func TestInviteSeparatesReadFromWrite(t *testing.T) {
	p := NewInvitePerm(&Env{Config: &config.Config{}})
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

// Signup answers to two things now: whether the caller is already somebody,
// and whether this installation takes self-serve registrations at all.
//
// The second is why REACT_APP_SIGNUP_CLOSED was not enough. It hid a tab while
// POST /signups stayed open, so "signups are closed" was true of the interface
// and false of the service.
func TestSignupIsForGuestsOnly(t *testing.T) {
	p := NewSignupPermissions(&Env{
		Config: &config.Config{
			RegistrationMode: auth_constants.RegistrationModeOpen,
		},
	})

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

func TestSignupFollowsTheRegistrationMode(t *testing.T) {
	guest := libs_entities.NewViewer()
	guest.PersonType = libs_constants.PersonTypeGuest

	for _, tc := range []struct {
		mode    string
		want    bool
		because string
	}{
		{auth_constants.RegistrationModeOpen, true, "open takes anybody"},
		{
			auth_constants.RegistrationModeApproval, true,
			"they start a signup request, not an account — refusing " +
				"them here makes the approval queue unreachable",
		},
		{auth_constants.RegistrationModeInvite, false, "invitations only"},
		{auth_constants.RegistrationModeClosed, false, "nothing gets in"},
		{"", false, "unset reads as closed"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			p := NewSignupPermissions(&Env{
				Config: &config.Config{RegistrationMode: tc.mode},
			})

			ok, err := p.CanAdd(guest, nil)
			if ok != tc.want {
				t.Errorf("CanAdd = %v, want %v — %s (err: %v)",
					ok, tc.want, tc.because, err)
			}

			// A refusal has to say which refusal it is. "Forbidden" alone
			// cannot tell a client to show an invitation notice rather than
			// a sign-in link.
			if !tc.want && !errors.Is(err, ErrRegistrationClosed) {
				t.Errorf("refusal was %v, want ErrRegistrationClosed", err)
			}
		})
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
