package middleware

import (
	"testing"

	"anahita-libs/constants"

	"github.com/golang-jwt/jwt/v5"
)

// TestBuildViewerPopulatesPersonType is the regression test for a bug
// that made every role check in the fleet useless.
//
// The person_type claim was being hydrated into Viewer.Type (the node
// entity type) while IsAdmin/IsSuperAdmin/IsGuest all read
// Viewer.PersonType, which NewViewer pins to "guest". The result: every
// token-authenticated viewer was permanently a guest, so admin branches
// were dead and even ordinary `if !viewer.IsGuest()` gates failed.
func TestBuildViewerPopulatesPersonType(t *testing.T) {
	cases := []struct {
		name           string
		personType     string
		wantAdmin      bool
		wantSuperAdmin bool
		wantGuest      bool
	}{
		{"registered", constants.PersonTypeRegistered, false, false, false},
		{"administrator", constants.PersonTypeAdmin, true, false, false},
		{"super administrator", constants.PersonTypeSuperAdmin, true, true, false},
		{"guest", constants.PersonTypeGuest, false, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewer, err := BuildViewer(jwt.MapClaims{
				constants.JWTClaimSub:        "42",
				constants.JWTClaimPersonType: tc.personType,
				constants.JWTClaimScope:      "actors:read actors:write",
				constants.JWTClaimPersonPK:   float64(42),
			})
			if err != nil {
				t.Fatalf("BuildViewer: %v", err)
			}

			if viewer.PersonType != tc.personType {
				t.Errorf("PersonType = %q, want %q — the role claim must land on PersonType, not Type", viewer.PersonType, tc.personType)
			}
			if got := viewer.IsAdmin(); got != tc.wantAdmin {
				t.Errorf("IsAdmin() = %v, want %v", got, tc.wantAdmin)
			}
			if got := viewer.IsSuperAdmin(); got != tc.wantSuperAdmin {
				t.Errorf("IsSuperAdmin() = %v, want %v", got, tc.wantSuperAdmin)
			}
			if got := viewer.IsGuest(); got != tc.wantGuest {
				t.Errorf("IsGuest() = %v, want %v", got, tc.wantGuest)
			}

			// Type must keep the node entity type — the node helpers
			// depend on it, which is why the role cannot live there.
			if viewer.Type != constants.NodeActorPersonEntityType {
				t.Errorf("Type = %q, want the node entity type %q", viewer.Type, constants.NodeActorPersonEntityType)
			}
		})
	}
}

// TestBuildViewerPopulatesScopes confirms the scope claim survives into
// the slice HasScope reads, using constants rather than literals.
func TestBuildViewerPopulatesScopes(t *testing.T) {
	viewer, err := BuildViewer(jwt.MapClaims{
		constants.JWTClaimSub:        "7",
		constants.JWTClaimPersonType: constants.PersonTypeRegistered,
		constants.JWTClaimScope:      constants.OAuthScopeNotificationsRead + " " + constants.OAuthScopeMediaWrite,
		constants.JWTClaimPersonPK:   float64(7),
	})
	if err != nil {
		t.Fatalf("BuildViewer: %v", err)
	}

	if !viewer.HasScope(constants.OAuthScopeNotificationsRead) {
		t.Error("granted scope notifications:read is missing from the viewer")
	}
	if !viewer.HasScope(constants.OAuthScopeMediaWrite) {
		t.Error("granted scope media:write is missing from the viewer")
	}
	if viewer.HasScope(constants.OAuthScopeOAuthWrite) {
		t.Error("viewer reports a scope that was never granted")
	}
}
