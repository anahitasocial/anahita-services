package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"auth/data/entities"
	"auth/data/services"

	"github.com/labstack/echo/v4"
)

// A page asks for several things at once. When its access token has expired,
// one request renews the session and the others wait for it; none of them may
// end the session for that. Ending it on a race, and a reuse cascade set off
// by a session such a race had ended, signed people out every fifteen minutes
// on banu.io (2026-11-02).

func sessionWithTokens(t *testing.T, sess *fakeSession, access, refresh string) {
	t.Helper()
	raw, err := json.Marshal(storedOAuthTokens{AccessToken: access, RefreshToken: refresh})
	if err != nil {
		t.Fatal(err)
	}
	sess.values[sessionKeyOAuthTokens] = string(raw)
}

func renewalContext() echo.Context {
	return echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/oauth/validate", nil), httptest.NewRecorder())
}

func TestAwaitRenewalTakesUpTheTokenAnotherRequestLeft(t *testing.T) {
	te := newTestEnv()
	h := &oauthHandler{Env: te.Env}
	sessionWithTokens(t, te.Sess, "old", "refresh-1")

	// The request renewing the session finishes a little later.
	go func() {
		time.Sleep(3 * oauthRefreshWaitStep)
		te.Sess.mu.Lock()
		defer te.Sess.mu.Unlock()
		raw, _ := json.Marshal(storedOAuthTokens{AccessToken: "new", RefreshToken: "refresh-2"})
		te.Sess.values[sessionKeyOAuthTokens] = string(raw)
	}()

	token, err := h.awaitRenewal(renewalContext(), func(token string) bool { return token == "new" })
	if err != nil {
		t.Fatalf("awaitRenewal: %v", err)
	}
	if token != "new" {
		t.Errorf("token = %q, want the one the other request left", token)
	}
}

func TestAwaitRenewalSaysSoWhenTheSessionEnded(t *testing.T) {
	te := newTestEnv()
	h := &oauthHandler{Env: te.Env}
	// Nothing in the session: another request ended it.

	_, err := h.awaitRenewal(renewalContext(), func(string) bool { return true })
	if !errors.Is(err, errSessionGone) {
		t.Errorf("err = %v, want errSessionGone", err)
	}
	if endsSession(err) {
		t.Error("a session already ended is not ended again by this request")
	}
}

func TestAwaitRenewalGivesUpWithoutEndingTheSession(t *testing.T) {
	te := newTestEnv()
	h := &oauthHandler{Env: te.Env}
	sessionWithTokens(t, te.Sess, "old", "refresh-1")

	started := time.Now()
	_, err := h.awaitRenewal(renewalContext(), func(string) bool { return false })
	if !errors.Is(err, errRenewingElsewhere) {
		t.Errorf("err = %v, want errRenewingElsewhere", err)
	}
	if endsSession(err) {
		t.Error("waiting too long for another request must not end the session")
	}
	if waited := time.Since(started); waited > oauthRefreshWaitMost+time.Second {
		t.Errorf("waited %v, longer than it should", waited)
	}
}

func TestOnlyASessionThatIsOverIsEnded(t *testing.T) {
	cases := []struct {
		err  error
		ends bool
	}{
		{fmt.Errorf("refresh token pair: %w", entities.ErrRefreshTokenReuse), true},
		{fmt.Errorf("refresh token pair: %w", entities.ErrRefreshTokenRevoked), true},
		{fmt.Errorf("refresh token pair: %w", entities.ErrRefreshTokenExpired), true},
		{fmt.Errorf("refresh token pair: %w", entities.ErrRefreshTokenNotFound), true},
		{entities.ErrRefreshTokenNotFound, true},
		{fmt.Errorf("refresh token pair: %w", services.ErrRefreshAccountUnavailable), true},
		{errNoRefreshToken, true},
		{errRenewingElsewhere, false},
		{fmt.Errorf("%w: redis: nil", errSessionGone), false},
		{fmt.Errorf("acquire refresh lock: %w", errors.New("connection refused")), false},
		{fmt.Errorf("client lookup: %w", errors.New("timeout")), false},
	}
	for _, c := range cases {
		if got := endsSession(c.err); got != c.ends {
			t.Errorf("endsSession(%v) = %v, want %v", c.err, got, c.ends)
		}
	}
}
