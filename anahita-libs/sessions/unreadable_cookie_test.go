package sessions

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type testConfig struct{}

func (testConfig) GetSecureCookieKey() string { return "12345678901234567890123456789012" }
func (testConfig) GetRedisAddr() string       { return "" }
func (testConfig) GetRedisPassword() string   { return "" }
func (testConfig) GetRedisDB() int            { return 0 }
func (testConfig) IsStaging() bool            { return false }
func (testConfig) IsDev() bool                { return true }
func (testConfig) IsProd() bool               { return false }
func (testConfig) GetDomainName() string      { return "anahita.test" }

// A session cookie that cannot be opened, changed or sealed with the cipher
// before v1.0.17, is no session: read as if there were no cookie, so its
// holder signs in again, and nothing answers with an error (roadmap row 119).
func TestAnUnreadableCookieIsNoSession(t *testing.T) {
	session := NewCookieSession(testConfig{})

	// Sealed here, then read back: the session as it was given.
	given := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	if _, err := session.Add(given, "session", `{"user_id":5}`); err != nil {
		t.Fatal(err)
	}
	sealed := given.Response().Header().Get("Set-Cookie")
	if sealed == "" {
		t.Fatal("no cookie was set")
	}
	cookie, err := http.ParseSetCookie(sealed)
	if err != nil {
		t.Fatal(err)
	}

	read := func(value string) (string, error) {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.AddCookie(&http.Cookie{Name: "session", Value: value})
		return session.Read(echo.New().NewContext(request, httptest.NewRecorder()), "session")
	}

	if got, err := read(cookie.Value); err != nil || got != `{"user_id":5}` {
		t.Fatalf("the cookie as given: %q, %v", got, err)
	}

	// One letter of it changed.
	changed := []byte(cookie.Value)
	if changed[10] == 'A' {
		changed[10] = 'B'
	} else {
		changed[10] = 'A'
	}
	if got, err := read(string(changed)); err != nil || got != "" {
		t.Errorf("a changed cookie: %q, %v; want no session and no error", got, err)
	}
}
