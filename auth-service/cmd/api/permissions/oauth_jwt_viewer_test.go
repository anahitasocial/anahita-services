package permissions

import (
	"testing"

	libs_constants "anahita-libs/constants"
	libs_middleware "anahita-libs/middleware"

	"auth/data/entities"

	"github.com/golang-jwt/jwt/v5"
)

// These tests build the viewer the way the authorize middleware really
// does — from JWT claims, through BuildViewer — rather than by filling
// a struct in the test. That is the whole point of them.
//
// Every OAuth write permission once ended with `if !viewer.TotpEnabled`.
// A struct-built viewer in a test can have that field set to anything,
// so a test written the usual way would have passed while all five
// endpoints refused every caller in production: there is no TOTP claim,
// so the field is false on every real request no matter what the
// account has enrolled. hydrateViewer's doc comment names this trap,
// and it had already been hit once in the WebAuthn handlers before it
// was hit again here.
//
// So the invariant these pin down is not "a super admin may add a
// client". It is: THE OAUTH PERMISSIONS READ NOTHING THE JWT CANNOT
// CARRY. Scopes and person_type are claims; TOTP enrolment, passkey
// count and MFA freshness are not. Anything in the second group belongs
// in the handler, after hydrateViewer or behind requireStepUp — not
// here, where it silently reads as absent.

// claimsFor builds the claim set the token endpoint mints for a person.
// Deliberately complete: everything a real access token carries, and
// nothing more.
func claimsFor(personType string, scopes string) jwt.MapClaims {
	return jwt.MapClaims{
		"sub":         "42",
		"person_pk":   float64(42),
		"person_type": personType,
		"scope":       scopes,
		"client_id":   "anahita-web",
	}
}

func TestOAuthWritesReachableByJWTSuperAdmin(t *testing.T) {
	claims := claimsFor(
		libs_constants.PersonTypeSuperAdmin,
		"oauth:read oauth:write",
	)

	viewer, err := libs_middleware.BuildViewer(claims)
	if err != nil {
		t.Fatalf("BuildViewer: %v", err)
	}

	// The field that caused the outage. Asserted rather than assumed,
	// so that if a TOTP claim is ever added this test says so instead
	// of quietly starting to prove something else.
	if viewer.TotpEnabled {
		t.Fatal("a JWT-built viewer now reports TotpEnabled — if a claim " +
			"was added, revisit every gate that reads person state")
	}

	perm := NewOAuthPerm(&Env{})
	keyPerm := NewOAuthSigningKeyPerm(&Env{})
	client := &entities.OAuthClient{}

	checks := []struct {
		name string
		run  func() (bool, error)
	}{
		{"CanAddClient", func() (bool, error) {
			return perm.CanAddClient(viewer)
		}},
		{"CanEditClient", func() (bool, error) {
			return perm.CanEditClient(viewer, client)
		}},
		{"CanDeleteClient", func() (bool, error) {
			return perm.CanDeleteClient(viewer, client)
		}},
		{"CanEditSigningKey", func() (bool, error) {
			return keyPerm.CanEditSigningKey(viewer, nil)
		}},
	}

	for _, check := range checks {
		ok, permErr := check.run()
		if !ok {
			t.Errorf("%s denied a super admin holding oauth:write: %v",
				check.name, permErr)
		}
	}
}

// The permissions must still refuse the people they always refused.
// Removing the TOTP branch was meant to unbreak the gate, not open it.
func TestOAuthWritesStillRefuseTheWrongPeople(t *testing.T) {
	perm := NewOAuthPerm(&Env{})
	keyPerm := NewOAuthSigningKeyPerm(&Env{})
	client := &entities.OAuthClient{}

	cases := []struct {
		name       string
		personType string
		scopes     string
	}{
		{
			name:       "administrator with the scope",
			personType: libs_constants.PersonTypeAdmin,
			scopes:     "oauth:read oauth:write",
		},
		{
			name:       "super admin without the write scope",
			personType: libs_constants.PersonTypeSuperAdmin,
			scopes:     "oauth:read",
		},
		{
			name:       "registered person",
			personType: libs_constants.PersonTypeRegistered,
			scopes:     "actors:write",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewer, err := libs_middleware.BuildViewer(
				claimsFor(tc.personType, tc.scopes),
			)
			if err != nil {
				t.Fatalf("BuildViewer: %v", err)
			}

			if ok, _ := perm.CanAddClient(viewer); ok {
				t.Error("CanAddClient allowed them")
			}
			if ok, _ := perm.CanEditClient(viewer, client); ok {
				t.Error("CanEditClient allowed them")
			}
			if ok, _ := perm.CanDeleteClient(viewer, client); ok {
				t.Error("CanDeleteClient allowed them")
			}
			if ok, _ := keyPerm.CanEditSigningKey(viewer, nil); ok {
				t.Error("CanEditSigningKey allowed them")
			}
		})
	}
}
