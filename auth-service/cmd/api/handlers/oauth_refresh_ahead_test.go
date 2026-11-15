package handlers

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// A session's token is renewed in its last minute, before it can run out on
// its way through the gateway, and not before.
func TestRefreshDue(t *testing.T) {
	now := time.Now()
	token := func(exp time.Time) *jwt.Token {
		// As a parsed token holds it: JSON numbers are float64.
		return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": float64(exp.Unix())})
	}
	for _, each := range []struct {
		what  string
		token *jwt.Token
		want  bool
	}{
		{"ten minutes left", token(now.Add(10 * time.Minute)), false},
		{"two minutes left", token(now.Add(2 * time.Minute)), false},
		{"thirty seconds left", token(now.Add(30 * time.Second)), true},
		{"no expiry", jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{}), false},
		{"no token", nil, false},
	} {
		if got := refreshDue(each.token, now); got != each.want {
			t.Errorf("%s: refreshDue = %v, want %v", each.what, got, each.want)
		}
	}
}

// When the early renewal fails, the current token, still valid, goes on.
func TestValidateRefreshAheadKeepsTheCurrentTokenWhenRenewalFails(t *testing.T) {
	te := newTestEnv()
	h := &oauthHandler{Env: te.Env}
	soon := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": float64(time.Now().Add(20 * time.Second).Unix())})

	// No refresh token: the renewal fails at once.
	got := h.validateRefreshAhead(renewalContext(), &storedOAuthTokens{AccessToken: "current"}, soon)
	if got != "current" {
		t.Errorf("forwarded %q, want the current token", got)
	}
}
