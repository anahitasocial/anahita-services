package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"auth/data/entities"

	libconstants "anahita-libs/constants"
	libentities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// PATCH /password/change — the authenticated half of the flow.
//
// Different threat model from the reset: the caller already holds a
// session, so the session cannot be the proof. Everything here is about
// what the endpoint demands beyond it, and about the ways in which it
// deliberately behaves UNLIKE the reset.

func patchJSON(e *echo.Echo, body string, viewer *libentities.Viewer) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPatch, "/password/change", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if viewer != nil {
		c.Set(libconstants.ViewerContextKey, viewer)
	}
	return c, rec
}

func testViewer(id, refreshTokenID int64) *libentities.Viewer {
	v := libentities.NewViewer()
	v.ID = id
	v.OAuthRefreshTokenID = refreshTokenID
	return v
}

// Without a live step-up marker the answer is 403 — which the SPA reads as
// "collect proof and retry", not as a refusal.
func TestPasswordEditRequiresAStepUp(t *testing.T) {
	te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))

	c, rec := patchJSON(newTestEcho(), `{"password":"a brand new password"}`, testViewer(42, 7))
	if err := newPasswordHandler(te.Env).Edit(c); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if te.Users.editCalls != 0 {
		t.Error("the password was changed with no step-up")
	}
}

// A store outage must not read as "prove yourself again" — the client would
// prompt, write nothing, retry, and blame the person for an outage.
func TestPasswordEditReportsAStoreFailureAsOurs(t *testing.T) {
	te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))
	te.Sess.existsErr = errStoreDown

	c, rec := patchJSON(newTestEcho(), `{"password":"a brand new password"}`, testViewer(42, 7))
	if err := newPasswordHandler(te.Env).Edit(c); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestPasswordEditChangesThePasswordAndSpendsTheStepUp(t *testing.T) {
	te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))
	if err := te.Env.markStepUp(42); err != nil {
		t.Fatalf("mark step-up: %v", err)
	}

	// Two sessions: the one this request arrived on, and another device.
	te.AuthLogs.logs = []*entities.AuthLog{
		{ID: 1, OAuthRefreshTokenID: 7},
		{ID: 2, OAuthRefreshTokenID: 8},
	}
	te.Refresh.byID[7] = &entities.OAuthRefreshToken{ID: 7, ExpiresAt: time.Now().Add(time.Hour)}
	te.Refresh.byID[8] = &entities.OAuthRefreshToken{ID: 8, ExpiresAt: time.Now().Add(time.Hour)}

	c, rec := patchJSON(newTestEcho(), `{"password":"a brand new password"}`, testViewer(42, 7))
	if err := newPasswordHandler(te.Env).Edit(c); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if te.Users.editCalls != 1 {
		t.Errorf("EditPassword called %d times, want 1", te.Users.editCalls)
	}

	// Every other session, and only the others — signing the person out of
	// the tab they are working in turns a security action into a logout.
	if len(te.Refresh.revoked) != 1 || te.Refresh.revoked[0] != 8 {
		t.Errorf("revoked %v, want only the other session [8]", te.Refresh.revoked)
	}
	if !strings.Contains(rec.Body.String(), `"revoked_sessions":1`) {
		t.Errorf("body %s does not report the session count", rec.Body.String())
	}

	// One step-up authorises one credential change.
	if err := te.Env.requireStepUp(42); err == nil {
		t.Error("the step-up marker survived the change")
	}

	// UNLIKE the reset. A passkey holder changing their password must not
	// lose the credential they sign in with — that is the trap this flow
	// exists to close.
	if len(te.Passkeys.deletedFor) != 0 {
		t.Error("passkeys were revoked by an authenticated password change")
	}
}

// 409 rather than 400: the payload is well-formed and conflicts with stored
// state. Reusing the current password would revoke every other session and
// send "your password was updated" while changing nothing.
func TestPasswordEditRefusesTheCurrentPassword(t *testing.T) {
	const current = "the current password value"
	te := newTestEnv(testUser(t, 42, "ada@example.com", current))
	if err := te.Env.markStepUp(42); err != nil {
		t.Fatalf("mark step-up: %v", err)
	}

	c, rec := patchJSON(newTestEcho(), `{"password":"`+current+`"}`, testViewer(42, 7))
	if err := newPasswordHandler(te.Env).Edit(c); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if te.Users.editCalls != 0 {
		t.Error("the password was written anyway")
	}
	// The marker is NOT spent by a refused attempt — but the rate-limit
	// budget is, which is what stops the endpoint being a password oracle
	// for whoever holds the session.
	if err := te.Env.requireStepUp(42); err != nil {
		t.Error("a refused attempt consumed the step-up")
	}
}

func TestPasswordEditRejectsAnUnauthenticatedCaller(t *testing.T) {
	te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))

	c, rec := patchJSON(newTestEcho(), `{"password":"a brand new password"}`, nil)
	if err := newPasswordHandler(te.Env).Edit(c); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if te.Users.editCalls != 0 {
		t.Error("an unauthenticated request changed a password")
	}
}

func TestPasswordEditThrottles(t *testing.T) {
	const current = "the current password value"
	te := newTestEnv(testUser(t, 42, "ada@example.com", current))
	h := newPasswordHandler(te.Env)
	e := newTestEcho()

	// Budget is 5 per 15 minutes. Refused attempts spend it; the marker
	// survives them, so this really is five tries at the same wall.
	if err := te.Env.markStepUp(42); err != nil {
		t.Fatalf("mark step-up: %v", err)
	}
	for i := range 5 {
		c, rec := patchJSON(e, `{"password":"`+current+`"}`, testViewer(42, 7))
		if err := h.Edit(c); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
		if rec.Code != http.StatusConflict {
			t.Fatalf("attempt %d: status = %d, want 409", i+1, rec.Code)
		}
	}

	c, rec := patchJSON(e, `{"password":"a brand new password"}`, testViewer(42, 7))
	if err := h.Edit(c); err != nil {
		t.Fatalf("sixth attempt: %v", err)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
	if te.Users.editCalls != 0 {
		t.Error("the throttled request still changed the password")
	}
}

// ---------------------------------------------------------------------
// Step-up markers
// ---------------------------------------------------------------------

func TestStepUpLifecycle(t *testing.T) {
	te := newTestEnv()

	if err := te.Env.requireStepUp(42); err == nil {
		t.Error("a person with no marker passed the gate")
	}

	if err := te.Env.markStepUp(42); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if err := te.Env.requireStepUp(42); err != nil {
		t.Errorf("a marked person was refused: %v", err)
	}

	// Markers are per person: proving who you are does not authorise a
	// change to somebody else's credentials.
	if err := te.Env.requireStepUp(99); err == nil {
		t.Error("one person's step-up satisfied another's gate")
	}

	te.Env.clearStepUp(42)
	if err := te.Env.requireStepUp(42); err == nil {
		t.Error("the marker survived being cleared")
	}
}

// Fails CLOSED, deliberately unlike the rate limiter: degrading open here
// would hand every credential mutation to anyone holding a session, which
// is the exact thing the gate was added to stop.
func TestRequireStepUpFailsClosed(t *testing.T) {
	te := newTestEnv()
	te.Sess.existsErr = errStoreDown

	err := te.Env.requireStepUp(42)
	if err == nil {
		t.Fatal("a store outage let the request through")
	}
	// And it must not masquerade as "prove yourself again".
	if got := reauthStatus(err); got != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", got)
	}
}

func TestReauthStatusDistinguishesMissingProofFromOutage(t *testing.T) {
	if got := reauthStatus(errStepUpRequired); got != http.StatusForbidden {
		t.Errorf("missing proof = %d, want 403", got)
	}
	if got := reauthStatus(errStoreDown); got != http.StatusInternalServerError {
		t.Errorf("store failure = %d, want 500", got)
	}
}
