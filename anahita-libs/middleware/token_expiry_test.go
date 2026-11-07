package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

const expiryIssuer = "https://auth.anahita.test"

type expiryConfig struct{ jwksURL string }

func (expiryConfig) GetRedisAddr() string      { return "" }
func (expiryConfig) GetRedisPassword() string  { return "" }
func (expiryConfig) GetRedisDB() int           { return 0 }
func (expiryConfig) IsDev() bool               { return true }
func (expiryConfig) IsStaging() bool           { return false }
func (expiryConfig) IsProd() bool              { return false }
func (expiryConfig) GetOAuthIssuer() string    { return expiryIssuer }
func (c expiryConfig) GetOAuthJWKSURL() string { return c.jwksURL }

// A token the gateway is shown must say when it runs out. auth-service
// puts exp in every token it mints, so one without was not made there, and
// would never run out (roadmap row 119).
func TestATokenWithNoExpiryIsRefused(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"kid": "k1",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(keys))
	}))
	t.Cleanup(server.Close)
	jwks := NewJWKSClient(server.URL)

	sign := func(claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "k1"
		signed, err := token.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}
	shown := func(token string) error {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		_, err := authenticateOAuth(echo.New().NewContext(request, httptest.NewRecorder()), expiryConfig{server.URL}, jwks)
		return err
	}

	claims := jwt.MapClaims{"iss": expiryIssuer, "sub": "5", "iat": time.Now().Unix()}

	// Without exp: refused.
	if err := shown(sign(claims)); !errors.Is(err, jwt.ErrTokenRequiredClaimMissing) {
		t.Errorf("a token with no expiry: got %v, want it refused for that", err)
	}

	// With one still to come, the same token otherwise: taken, so it is
	// exp that was missing and nothing else. The viewer it names is not
	// looked at here.
	claims["exp"] = time.Now().Add(15 * time.Minute).Unix()
	if err := shown(sign(claims)); errors.Is(err, jwt.ErrTokenRequiredClaimMissing) {
		t.Errorf("a token with an expiry was refused for having none: %v", err)
	}
}
