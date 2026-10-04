package services

import (
	"encoding/json"
	"testing"
	"time"

	libs_constants "anahita-libs/constants"

	"auth/data/entities"
)

// userinfo is the ONE response the web app does not run through its
// camel-casing interceptor, so what this map is keyed on is what the client
// reads, letter for letter. That makes a rename here invisible on both sides:
// the server keeps answering 200 and the client keeps reading a field that is
// now undefined.
//
// It had already happened. The claims were `username` and `usertype` while
// every permission helper in the client read `personType` and every route read
// `alias` — so administration controls were off for everybody, super
// administrators included, and the "go to my profile" redirect went to
// /people/undefined. Nothing failed; things were just quietly absent.
//
// These tests pin the claim NAMES. They cannot tell you the client agrees —
// only the running stack can — but they stop the names moving silently.

func userinfoFor(user *entities.User, scopes string) map[string]any {
	return (&OAuthService{}).GetUserInfo(user, scopes)
}

func testViewer() *entities.User {
	user := entities.NewUser()
	user.ID = 42
	user.Name = "Ada Lovelace"
	user.Alias = "ada"
	user.Email = "ada@example.com"
	user.Type = libs_constants.NodeActorPersonEntityType
	user.PersonType = libs_constants.PersonTypeSuperAdmin
	user.Enabled = true

	return user
}

func TestUserInfoNamesActorFieldsLikeEveryOtherActor(t *testing.T) {
	info := userinfoFor(testViewer(), "openid profile email")

	// The actor half. Each of these is read by name in the web app.
	for claim, want := range map[string]any{
		"alias":       "ada",
		"person_type": libs_constants.PersonTypeSuperAdmin,
		"name":        "Ada Lovelace",
		"id":          int64(42),
		"email":       "ada@example.com",
	} {
		if got := info[claim]; got != want {
			t.Errorf("claim %q = %v, want %v", claim, got, want)
		}
	}

	// The retired spellings. They are not merely absent — emitting them
	// alongside the new ones would let the client keep reading the old name
	// and hide the very drift this is here to catch.
	for _, retired := range []string{"username", "usertype", "given_name", "family_name"} {
		if _, present := info[retired]; present {
			t.Errorf("claim %q is still emitted; the platform calls these "+
				"`alias` and `person_type`, and two names for one value is "+
				"how they drifted apart in the first place", retired)
		}
	}
}

// preferred_username is the exception, and the only one: OIDC names it, so it
// keeps protocol vocabulary. Removing it would break any conformant client.
func TestUserInfoKeepsTheStandardOIDCClaims(t *testing.T) {
	info := userinfoFor(testViewer(), "openid profile")

	for _, claim := range []string{"sub", "name", "preferred_username"} {
		if _, present := info[claim]; !present {
			t.Errorf("standard OIDC claim %q is missing", claim)
		}
	}

	if info["sub"] != "42" {
		t.Errorf("sub = %v, want the string \"42\" — OIDC requires a string", info["sub"])
	}
	if info["preferred_username"] != info["alias"] {
		t.Errorf("preferred_username (%v) and alias (%v) describe the same handle "+
			"and must not diverge", info["preferred_username"], info["alias"])
	}
}

// Scope gating, per OIDC §5.4. A client granted only `openid` learns the
// subject and nothing else.
func TestUserInfoWithholdsClaimsOutsideTheGrantedScopes(t *testing.T) {
	info := userinfoFor(testViewer(), "openid")

	if _, present := info["sub"]; !present {
		t.Fatal("sub is missing; it is returned for openid alone")
	}

	for _, gated := range []string{"name", "alias", "person_type", "email", "totp_enabled"} {
		if _, present := info[gated]; present {
			t.Errorf("claim %q was returned to a client granted only `openid`", gated)
		}
	}

	// email rides on its own scope, not on profile.
	withProfile := userinfoFor(testViewer(), "openid profile")
	if _, present := withProfile["email"]; present {
		t.Error("email was returned without the email scope")
	}
}

// A person who has never signed in has no last visit, and JSON null is how
// that is said. A zero date would read as a real visit in the year 1.
//
// Asserted on the MARSHALLED output rather than the map value, because the map
// holds a *time.Time — and a nil pointer boxed in an `any` is not equal to nil.
// Checking the map would pass on a value that serialises wrongly, or fail on
// one that serialises right, which is what it did.
func TestUserInfoReportsNeverVisitedAsNull(t *testing.T) {
	user := testViewer()
	user.LastVisitedAt = nil

	encoded, err := json.Marshal(userinfoFor(user, "openid profile"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	value, present := decoded["last_visited_at"]
	if !present {
		t.Fatal("last_visited_at is absent; the client distinguishes never-visited " +
			"from not-told, and an absent key says the second")
	}
	if value != nil {
		t.Errorf("last_visited_at = %v, want null for somebody who has never signed in", value)
	}
}

// And a real visit survives the round trip as a real date.
func TestUserInfoReportsARealVisit(t *testing.T) {
	user := testViewer()
	visited := time.Date(2026, 9, 13, 20, 15, 0, 0, time.UTC)
	user.LastVisitedAt = &visited

	encoded, err := json.Marshal(userinfoFor(user, "openid profile"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := decoded["last_visited_at"]; got != "2026-09-13T20:15:00Z" {
		t.Errorf("last_visited_at = %v, want the RFC 3339 timestamp", got)
	}
}

// Onboarding state. The web app's gate reads both by name, and like
// last_visited_at, "not yet" is null rather than absent: an absent key reads as
// an older server that does not know, not as a person who has not been through
// it.
func TestUserInfoCarriesOnboardingState(t *testing.T) {
	decode := func(user *entities.User) map[string]any {
		t.Helper()
		encoded, err := json.Marshal(userinfoFor(user, "openid profile"))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return decoded
	}

	fresh := decode(testViewer())
	value, present := fresh["onboarded_at"]
	if !present {
		t.Fatal("onboarded_at is absent; it must be null for somebody who has not been through onboarding")
	}
	if value != nil {
		t.Errorf("onboarded_at = %v, want null", value)
	}
	if fresh["has_bio"] != false {
		t.Errorf("has_bio = %v, want false", fresh["has_bio"])
	}

	user := testViewer()
	at := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	user.OnboardedAt = &at
	user.HasBio = true

	done := decode(user)
	if got := done["onboarded_at"]; got != "2026-09-27T09:00:00Z" {
		t.Errorf("onboarded_at = %v, want the RFC 3339 timestamp", got)
	}
	if done["has_bio"] != true {
		t.Errorf("has_bio = %v, want true", done["has_bio"])
	}

	// Profile-scoped, like the rest of the actor half.
	openid := userinfoFor(testViewer(), "openid")
	for _, gated := range []string{"onboarded_at", "has_bio"} {
		if _, present := openid[gated]; present {
			t.Errorf("claim %q was returned to a client granted only `openid`", gated)
		}
	}
}
