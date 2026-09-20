package requests

import (
	"strings"
	"testing"

	"auth/data/entities"

	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
)

// Request structs are validation rules expressed as struct tags, and a tag is
// the kind of thing that gets edited without anyone noticing what it was
// protecting. These run the SAME validator main.go installs — including the
// custom `username` rule — so a tag that stops matching reality fails here.
//
// The rules worth holding are the ones with a reason behind the number:
// a token length that must match what the generator emits, a PKCE verifier
// length fixed by RFC 7636, a password floor that is a security decision.

func newValidator() *validators.RequestValidator {
	return validators.NewRequestValidator(validator.New())
}

func valid(t *testing.T, v *validators.RequestValidator, req any) {
	t.Helper()
	if err := v.Validate(req); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func invalid(t *testing.T, v *validators.RequestValidator, req any, why string) {
	t.Helper()
	if err := v.Validate(req); err == nil {
		t.Errorf("expected invalid (%s), got nil", why)
	}
}

// ---------------------------------------------------------------------
// Token lengths, which are coupled to the generator
// ---------------------------------------------------------------------

// Every token field validates len=128 because Token.GenerateToken emits 64
// random bytes hex-encoded. If the generator ever changes, this is the test
// that says so — otherwise every reset link would fail validation at the door
// with a message about the password.
func TestTokenLengthMatchesWhatTheGeneratorProduces(t *testing.T) {
	token := entities.NewToken()
	if err := token.GenerateToken(); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if len(token.Body) != 128 {
		t.Fatalf("generated body is %d chars; the len=128 tags across requests/ "+
			"are now wrong and every tokenised link will be rejected", len(token.Body))
	}

	v := newValidator()

	valid(t, v, &PasswordReset{Token: token.Body, Password: "a perfectly good password"})
	invalid(t, v, &PasswordReset{Token: token.Body[:64], Password: "a perfectly good password"},
		"a half-length token")
	invalid(t, v, &PasswordReset{Token: token.Body + "a", Password: "a perfectly good password"},
		"an over-length token")
}

// ---------------------------------------------------------------------
// Passwords
// ---------------------------------------------------------------------

// 15 characters is a deliberate floor — long enough that a passphrase is the
// natural way to meet it, rather than a short string with a digit bolted on.
func TestPasswordLengthBounds(t *testing.T) {
	v := newValidator()
	token := strings.Repeat("a", 128)

	cases := []struct {
		name     string
		password string
		ok       bool
	}{
		{"at the floor", strings.Repeat("a", 15), true},
		{"one below the floor", strings.Repeat("a", 14), false},
		{"empty", "", false},
		{"at the ceiling", strings.Repeat("a", 150), true},
		{"over the ceiling", strings.Repeat("a", 151), false},
		// A long passphrase is the point of the floor, so it must pass.
		{"a passphrase", "correct horse battery staple and then some", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &PasswordReset{Token: token, Password: tc.password}
			if tc.ok {
				valid(t, v, req)
			} else {
				invalid(t, v, req, tc.name)
			}
		})
	}
}

// The authenticated change carries no current-password field: re-authentication
// moved to /reauth/*. A field reappearing here would mean the step-up gate had
// been quietly bypassed.
func TestPasswordEditCarriesOnlyANewPassword(t *testing.T) {
	v := newValidator()

	valid(t, v, &PasswordEdit{Password: strings.Repeat("a", 15)})
	invalid(t, v, &PasswordEdit{Password: "short"}, "a short password")
	invalid(t, v, &PasswordEdit{}, "an empty password")
}

// ---------------------------------------------------------------------
// Usernames
// ---------------------------------------------------------------------

// The custom rule allows letters, digits, hyphen and underscore, and nothing
// else — no dots, no spaces, no unicode. A username is also a profile slug, so
// what it permits ends up in a URL.
func TestUsernameCharacterSet(t *testing.T) {
	v := newValidator()

	cases := []struct {
		username string
		ok       bool
	}{
		{"ada", true},
		{"ada_lovelace", true},
		{"john-doe", true},
		{"Ada99", true},
		{"ab", false},                    // below min=3
		{strings.Repeat("a", 32), true},  // at max
		{strings.Repeat("a", 33), false}, // over max
		{"ada lovelace", false},          // space
		{"ada.lovelace", false},          // dot
		{"ada@example.com", false},       // an email is not a username
		{"adá", false},                   // unicode
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.username, func(t *testing.T) {
			req := &UsernameChangeEdit{Username: tc.username}
			if tc.ok {
				valid(t, v, req)
			} else {
				invalid(t, v, req, tc.username)
			}
		})
	}
}

// ---------------------------------------------------------------------
// OAuth
// ---------------------------------------------------------------------

// grant_type is a closed set. An unknown value must be refused by validation
// rather than reaching the handler's switch and falling through to a default.
func TestOAuthTokenGrantTypeIsAClosedSet(t *testing.T) {
	v := newValidator()

	for _, grant := range []string{"authorization_code", "refresh_token", "client_credentials"} {
		t.Run(grant, func(t *testing.T) {
			valid(t, v, &OAuthTokenReq{GrantType: grant})
		})
	}

	for _, grant := range []string{"", "password", "implicit", "AUTHORIZATION_CODE"} {
		t.Run("rejects "+grant, func(t *testing.T) {
			invalid(t, v, &OAuthTokenReq{GrantType: grant}, "grant type "+grant)
		})
	}
}

// PKCE verifier bounds are fixed by RFC 7636 §4.1 — 43 to 128 characters.
// Loosening them would accept verifiers no compliant client sends and weaken
// the exchange for one that does.
func TestPKCEVerifierBounds(t *testing.T) {
	v := newValidator()

	cases := []struct {
		name     string
		verifier string
		ok       bool
	}{
		{"at the RFC minimum", strings.Repeat("a", 43), true},
		{"one below", strings.Repeat("a", 42), false},
		{"at the RFC maximum", strings.Repeat("a", 128), true},
		{"one above", strings.Repeat("a", 129), false},
		// Optional: the flow also serves clients that do not use PKCE.
		{"absent", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &OAuthTokenReq{GrantType: "authorization_code", CodeVerifier: tc.verifier}
			if tc.ok {
				valid(t, v, req)
			} else {
				invalid(t, v, req, tc.name)
			}
		})
	}
}

// plain is accepted for compatibility, but nothing outside the pair is.
func TestPKCEChallengeMethodIsAClosedSet(t *testing.T) {
	v := newValidator()

	base := func(method string) *OAuthAuthorizeReq {
		return &OAuthAuthorizeReq{
			ResponseType:        "code",
			ClientID:            "anahita-web",
			RedirectURI:         "http://localhost:3000/oauth/callback",
			State:               "xyz",
			Scope:               "openid profile",
			CodeChallenge:       strings.Repeat("a", 43),
			CodeChallengeMethod: method,
		}
	}

	valid(t, v, base("S256"))
	valid(t, v, base("plain"))
	valid(t, v, base(""))
	invalid(t, v, base("s256"), "lowercase s256")
	invalid(t, v, base("none"), "none")
}

// ---------------------------------------------------------------------
// Email
// ---------------------------------------------------------------------

func TestEmailFieldsRejectMalformedAddresses(t *testing.T) {
	v := newValidator()

	valid(t, v, &PasswordForgot{Email: "ada@example.com"})

	for _, bad := range []string{"", "ada", "ada@", "@example.com", "ada example.com", "ada@@example.com"} {
		t.Run("rejects "+bad, func(t *testing.T) {
			invalid(t, v, &PasswordForgot{Email: bad}, bad)
		})
	}
}

// ---------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------

// The New* helpers exist so handlers do not bind onto a nil inner struct. A
// constructor that stops initialising a member is a nil dereference at the
// first bind, on a route nobody may exercise until production.
func TestConstructorsInitialiseTheirMembers(t *testing.T) {
	if r := NewOAuthTokenRequest(); r == nil || r.Token == nil {
		t.Error("NewOAuthTokenRequest left Token nil")
	}
	if r := NewOAuthAuthorizeRequest(); r == nil || r.Authorize == nil {
		t.Error("NewOAuthAuthorizeRequest left Authorize nil")
	}
	if r := NewAuthLogRequest(); r == nil || r.Browse == nil {
		t.Error("NewAuthLogRequest left Browse nil")
	}
	if r := NewTotpRequest(); r == nil {
		t.Error("NewTotpRequest returned nil")
	}
}
