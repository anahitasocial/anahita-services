package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// acceptAll verifies every signature: these tests are about the clock.
type acceptAll struct{}

func (acceptAll) Verify(context.Context, string, string, []byte, []byte) error { return nil }

func tokenWith(t *testing.T, claims map[string]any) string {
	t.Helper()
	part := func(v any) string {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return part(map[string]any{"alg": "ES256", "kid": "test-kid", "typ": "at+jwt"}) + "." +
		part(claims) + "." + base64.RawURLEncoding.EncodeToString([]byte("signature"))
}

// A token is expired from its exp on, as the services count it, not one
// second later: counted in whole seconds, it was valid here through its last
// second and forwarded to services that refused it.
func TestATokenIsExpiredFromItsExpiryOn(t *testing.T) {
	svc := &OAuthService{verifier: acceptAll{}}
	now := time.Now()

	for _, each := range []struct {
		what  string
		exp   int64
		valid bool
	}{
		{"two minutes left", now.Add(2 * time.Minute).Unix(), true},
		{"its expiry this very second", now.Unix(), false},
		{"a second gone", now.Add(-time.Second).Unix(), false},
	} {
		_, err := svc.ValidateAccessToken(context.Background(), tokenWith(t, map[string]any{"exp": each.exp}))
		if each.valid && err != nil {
			t.Errorf("%s: refused: %v", each.what, err)
		}
		if !each.valid && err == nil {
			t.Errorf("%s: taken as valid", each.what)
		}
	}
}

// A token that is not valid yet is refused until its nbf.
func TestATokenIsNotValidBeforeItsStart(t *testing.T) {
	svc := &OAuthService{verifier: acceptAll{}}
	later := time.Now().Add(time.Minute).Unix()
	token := tokenWith(t, map[string]any{"exp": later + 600, "nbf": later})
	if _, err := svc.ValidateAccessToken(context.Background(), token); err == nil {
		t.Error("a token not valid for another minute was taken")
	}
}
