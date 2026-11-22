package settings

import (
	"strings"
	"testing"
)

// The defaults, the names Valkey's settings had before, and the keys'
// address made from the issuer's: once here, for every service, where each
// service's tests said it again.
func TestTheCommonSettingsAreReadAlike(t *testing.T) {
	for _, key := range []string{"ENV", "SECURE_COOKIE_KEY", "VALKEY_ADDR", "REDIS_ADDR", "VALKEY_PASSWORD", "REDIS_PASSWORD",
		"VALKEY_DB", "REDIS_DB", "FILE_SERVICE_ADDR", "GRAPH_SERVICE_ADDR", "AMQP_ADDR", "DSN", "OAUTH_ISSUER", "OAUTH_JWKS_URL"} {
		t.Setenv(key, "")
	}
	common := ReadCommon()
	if common.Env != "dev" || common.ValkeyAddr != "localhost:6379" || common.ValkeyDB != 0 || common.OAuthJWKSURL != "" {
		t.Errorf("defaults: %+v", common)
	}

	t.Setenv("REDIS_ADDR", "valkey:6379")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("OAUTH_ISSUER", "https://auth.example.com/")
	t.Setenv("DSN", "user@tcp(mysql:3306)/anahita")
	common = ReadCommon()
	if common.ValkeyAddr != "valkey:6379" || common.ValkeyDB != 2 {
		t.Errorf("the names before: %q %d", common.ValkeyAddr, common.ValkeyDB)
	}
	if common.GetOAuthJWKSURL() != "https://auth.example.com/.well-known/jwks.json" {
		t.Errorf("keys at %q", common.GetOAuthJWKSURL())
	}
	if common.GetDSN() != "user@tcp(mysql:3306)/anahita" {
		t.Errorf("DSN %q", common.GetDSN())
	}
	t.Setenv("OAUTH_JWKS_URL", "https://keys.example.com/jwks.json")
	if got := ReadCommon().GetOAuthJWKSURL(); got != "https://keys.example.com/jwks.json" {
		t.Errorf("keys given: %q", got)
	}

	for env, want := range map[string][3]bool{"dev": {true, false, false}, "staging": {false, true, false}, "prod": {false, false, true}} {
		c := Common{Env: env}
		if got := [3]bool{c.IsDev(), c.IsStaging(), c.IsProd()}; got != want {
			t.Errorf("%s: %v", env, got)
		}
	}
}

// What a service needs and has not is said, in the order it asks, once.
func TestWhatIsMissingIsSaidInOrder(t *testing.T) {
	all := []Need{CookieKey, Valkey, FileService, GraphService, Amqp, Database, OAuthIssuer, OAuthJWKS}
	got := Common{}.Missing(all...)
	want := []string{ErrMissingSecureCookieKey, ErrMissingValkeyAddr, ErrMissingFileAddr, ErrMissingGraphAddr,
		ErrMissingAmqpAddr, ErrMissingDSN, ErrMissingOAuthIssuer, ErrMissingOAuthJWKSURL}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("missing: %v", got)
	}
	if got := (Common{SecureCookieKey: "short"}).Missing(CookieKey); len(got) != 1 || got[0] != ErrWrongSizeSecureCookieKey {
		t.Errorf("a short key: %v", got)
	}
	full := Common{SecureCookieKey: strings.Repeat("k", 32), ValkeyAddr: "v", FileServiceAddr: "f", GraphServiceAddr: "g",
		AmqpAddr: "a", DSN: "d", OAuthIssuer: "i", OAuthJWKSURL: "j"}
	if got := full.Missing(all...); len(got) != 0 {
		t.Errorf("nothing is missing: %v", got)
	}
}
