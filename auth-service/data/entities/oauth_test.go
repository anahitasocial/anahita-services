package entities

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"
)

// The OAuth entities carry the decisions the token endpoint makes: whether a
// redirect is allowed back to, whether a grant is permitted, whether a refresh
// token is still usable. Each is a small function, and each fails open in an
// interesting way if it stops being right.

// ---------------------------------------------------------------------
// Redirect URIs
// ---------------------------------------------------------------------

// The redirect URI is where an authorization code is delivered. A scheme
// filter that lets javascript: or data: through turns the OAuth flow into an
// XSS vector against whoever clicks the link.
func TestRedirectURISchemeRules(t *testing.T) {
	cases := []struct {
		uri  string
		want bool
		why  string
	}{
		{"https://app.example.com/callback", true, "ordinary https"},
		{"https://app.example.com/callback?tenant=acme", true, "https with a query"},

		// Loopback over http is a development convenience and is limited to
		// loopback hostnames.
		{"http://localhost:3000/oauth/callback", true, "localhost"},
		{"http://127.0.0.1:3000/callback", true, "IPv4 loopback"},
		{"http://[::1]:3000/callback", true, "IPv6 loopback"},
		{"http://app.example.com/callback", false, "plain http off-loopback"},
		{"http://evil.com/callback", false, "plain http"},
		// Not loopback despite the prefix — a lookalike host must not pass.
		{"http://localhost.evil.com/callback", false, "a host that merely starts with localhost"},

		// Native apps register a reverse-domain custom scheme.
		{"com.example.app:/oauth/callback", true, "reverse-domain custom scheme"},

		// The dot requirement is what keeps these out.
		{"javascript:alert(1)", false, "javascript"},
		{"data:text/html,<script>alert(1)</script>", false, "data"},
		{"file:///etc/passwd", false, "file"},
		{"vbscript:msgbox", false, "vbscript"},
		{"about:blank", false, "about"},

		{"", false, "empty"},
	}

	for _, tc := range cases {
		t.Run(tc.why, func(t *testing.T) {
			c := NewOAuthClient()
			c.RedirectURIs = StringSlice{tc.uri}
			c.ClientID = "test-client"
			c.Name = "Test"
			// NewOAuthClient defaults to confidential, which requires one.
			c.SecretHash = "$2a$10$hash"
			c.GrantTypes = StringSlice{GrantTypeAuthorizationCode}
			c.TokenExpiry = 3600

			err := c.Validate()
			got := err == nil

			if got != tc.want {
				t.Errorf("Validate() accepted=%v for %q, want %v (err: %v)",
					got, tc.uri, tc.want, err)
			}
		})
	}
}

// Exact match only, per OAuth 2.1: prefix matching lets a registered
// https://app.example.com/ accept https://app.example.com.evil.com/.
func TestAllowsRedirectURIIsExactMatch(t *testing.T) {
	c := NewOAuthClient()
	c.RedirectURIs = StringSlice{"https://app.example.com/callback"}

	if !c.AllowsRedirectURI("https://app.example.com/callback") {
		t.Error("the registered URI was refused")
	}

	for _, near := range []string{
		"https://app.example.com/callback/extra",
		"https://app.example.com/callback?x=1",
		"https://app.example.com.evil.com/callback",
		"https://app.example.com/CALLBACK",
		"http://app.example.com/callback",
	} {
		if c.AllowsRedirectURI(near) {
			t.Errorf("accepted %q, which is not the registered URI", near)
		}
	}
}

func TestClientValidateRequiresTheEssentials(t *testing.T) {
	complete := func() *OAuthClient {
		c := NewOAuthClient()
		c.ClientID = "anahita-web"
		c.Name = "Anahita Web"
		c.SecretHash = "$2a$10$hash"
		c.RedirectURIs = StringSlice{"https://app.example.com/callback"}
		c.GrantTypes = StringSlice{GrantTypeAuthorizationCode}
		c.TokenExpiry = 3600
		return c
	}

	if err := complete().Validate(); err != nil {
		t.Fatalf("a complete client was rejected: %v", err)
	}

	cases := map[string]func(*OAuthClient){
		"no client id":       func(c *OAuthClient) { c.ClientID = "" },
		"no name":            func(c *OAuthClient) { c.Name = "" },
		"whitespace name":    func(c *OAuthClient) { c.Name = "   " },
		"no redirect uris":   func(c *OAuthClient) { c.RedirectURIs = nil },
		"no grant types":     func(c *OAuthClient) { c.GrantTypes = nil },
		"unknown grant type": func(c *OAuthClient) { c.GrantTypes = StringSlice{"password"} },
		"zero expiry":        func(c *OAuthClient) { c.TokenExpiry = 0 },
		"negative expiry":    func(c *OAuthClient) { c.TokenExpiry = -1 },
		// A confidential client with no secret can never authenticate, so the
		// row would be unusable rather than merely incomplete.
		"confidential without a secret": func(c *OAuthClient) { c.SecretHash = "" },
	}

	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			c := complete()
			break_(c)
			if err := c.Validate(); err == nil {
				t.Errorf("accepted a client with %s", name)
			}
		})
	}
}

// A third-party client always sees the consent screen, whatever its
// skip_consent flag says — the flag exists for first-party clients, where
// consent is implied by the person already trusting the installation.
func TestRequiresConsentIgnoresSkipForThirdParties(t *testing.T) {
	cases := []struct {
		firstParty  bool
		skipConsent bool
		want        bool
	}{
		{firstParty: true, skipConsent: true, want: false},
		{firstParty: true, skipConsent: false, want: true},
		// Both of these must still prompt.
		{firstParty: false, skipConsent: true, want: true},
		{firstParty: false, skipConsent: false, want: true},
	}

	for _, tc := range cases {
		c := NewOAuthClient()
		c.FirstParty = tc.firstParty
		c.SkipConsent = tc.skipConsent

		if got := c.RequiresConsent(); got != tc.want {
			t.Errorf("firstParty=%v skipConsent=%v: RequiresConsent()=%v, want %v",
				tc.firstParty, tc.skipConsent, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------
// Refresh tokens and authorization codes
// ---------------------------------------------------------------------

func TestRefreshTokenValidity(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	cases := []struct {
		name      string
		revoked   bool
		expiresAt time.Time
		valid     bool
	}{
		{"live", false, future, true},
		{"revoked", true, future, false},
		{"expired", false, past, false},
		{"revoked and expired", true, past, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := NewOAuthRefreshToken()
			token.Revoked = tc.revoked
			token.ExpiresAt = tc.expiresAt

			if got := token.IsValid(); got != tc.valid {
				t.Errorf("IsValid() = %v, want %v", got, tc.valid)
			}
		})
	}
}

// A zero expiry is in the past, so an uninitialised token must not read as
// usable — that would make a row with a missing column a live credential.
func TestRefreshTokenWithNoExpiryIsNotValid(t *testing.T) {
	if NewOAuthRefreshToken().IsValid() {
		t.Error("a token with a zero expiry is usable")
	}
}

func TestScopeListSplitting(t *testing.T) {
	cases := []struct {
		scopes string
		want   int
	}{
		{"", 0},
		{"openid", 1},
		{"openid profile email", 3},
		// Fields collapses runs of whitespace, so a doubled space does not
		// produce an empty scope that would never match anything.
		{"openid  profile", 2},
		{"  openid profile  ", 2},
	}

	for _, tc := range cases {
		t.Run(tc.scopes, func(t *testing.T) {
			rt := NewOAuthRefreshToken()
			rt.Scopes = tc.scopes
			if got := len(rt.ScopeList()); got != tc.want {
				t.Errorf("refresh token: got %d scopes, want %d", got, tc.want)
			}

			auth := NewOAuthAuthorization()
			auth.Scopes = tc.scopes
			if got := len(auth.ScopeList()); got != tc.want {
				t.Errorf("authorization: got %d scopes, want %d", got, tc.want)
			}
		})
	}
}

// PKCE is required for the exchange only when the authorization was started
// with a challenge. Reporting it required when it is not would break clients
// that never sent one; reporting it optional when it was sent would let an
// attacker who stole a code redeem it without the verifier.
func TestAuthorizationRequiresPKCE(t *testing.T) {
	a := NewOAuthAuthorization()
	if a.RequiresPKCE() {
		t.Error("PKCE reported required with no challenge")
	}

	a.CodeChallenge = "a-challenge"
	if !a.RequiresPKCE() {
		t.Error("PKCE reported optional despite a challenge being present")
	}
}

func TestAuthorizationExpiry(t *testing.T) {
	a := NewOAuthAuthorization()
	a.ExpiresAt = time.Now().Add(-time.Second)
	if !a.IsExpired() {
		t.Error("a code one second past expiry reads as live")
	}

	a.ExpiresAt = time.Now().Add(time.Minute)
	if a.IsExpired() {
		t.Error("a code inside its window reads as expired")
	}

	// A zero expiry must read as expired, so an unset column cannot mint a
	// permanently redeemable code.
	if !NewOAuthAuthorization().IsExpired() {
		t.Error("a code with no expiry reads as live")
	}
}

// ---------------------------------------------------------------------
// StringSlice
// ---------------------------------------------------------------------

// MySQL has no array type, so these columns are JSON. The round trip has to
// survive both encodings the driver may hand back.
func TestStringSliceRoundTrip(t *testing.T) {
	original := StringSlice{"https://a.example.com/cb", "com.example.app:/cb"}

	value, err := original.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}

	for name, src := range map[string]any{
		"bytes":  value,
		"string": string(value.([]byte)),
	} {
		t.Run(name, func(t *testing.T) {
			var out StringSlice
			if err := out.Scan(src); err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(out) != len(original) {
				t.Fatalf("got %d elements, want %d", len(out), len(original))
			}
			for i := range original {
				if out[i] != original[i] {
					t.Errorf("element %d = %q, want %q", i, out[i], original[i])
				}
			}
		})
	}
}

// A nil slice must store as an empty JSON array rather than SQL NULL: the
// column is NOT NULL, and a nil would fail the insert.
func TestStringSliceNilStoresAsEmptyArray(t *testing.T) {
	var s StringSlice

	value, err := s.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}

	b, ok := value.([]byte)
	if !ok {
		t.Fatalf("Value returned %T, want []byte", value)
	}
	if string(b) != "[]" {
		t.Errorf("nil stored as %q, want []", b)
	}

	// And it must still be valid JSON an array-typed column accepts.
	var decoded []string
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Errorf("stored value is not valid JSON: %v", err)
	}
}

func TestStringSliceScanHandlesNullAndGarbage(t *testing.T) {
	var s StringSlice

	if err := s.Scan(nil); err != nil {
		t.Errorf("scanning NULL failed: %v", err)
	}
	if s != nil {
		t.Errorf("NULL scanned to %v, want nil", s)
	}

	if err := s.Scan([]byte{}); err != nil {
		t.Errorf("scanning an empty column failed: %v", err)
	}

	// A type the driver should never produce must error rather than silently
	// leaving the previous value in place.
	if err := s.Scan(42); err == nil {
		t.Error("scanned an int into a StringSlice")
	}
}

func TestStringSliceContains(t *testing.T) {
	s := StringSlice{"openid", "profile"}

	if !s.Contains("openid") {
		t.Error("Contains missed a present value")
	}
	if s.Contains("email") {
		t.Error("Contains found an absent value")
	}
	if s.Contains("") {
		t.Error("Contains matched the empty string")
	}
	// Case-sensitive: scopes and grant types are exact identifiers.
	if s.Contains("OpenID") {
		t.Error("Contains matched case-insensitively")
	}
}

var _ driver.Valuer = StringSlice{}
