package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"auth/constants"
	"auth/data/entities"

	libconstants "anahita-libs/constants"
	libentities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// emailFlow is a signed-in person changing their address from the app.
type emailFlow struct {
	t  *testing.T
	te *testEnv
	h  *emailChangeHandler
	e  *echo.Echo
}

func newEmailFlow(t *testing.T, users ...*entities.User) *emailFlow {
	t.Helper()
	te := newTestEnv(users...)
	te.Config.SecureCookieKey = "0123456789abcdef0123456789abcdef"
	return &emailFlow{t: t, te: te, h: newEmailChangeHandler(te.Env), e: newTestEcho()}
}

func (f *emailFlow) call(method, path, body string, viewer *libentities.Viewer, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := f.e.NewContext(req, rec)
	if viewer != nil {
		c.Set(libconstants.ViewerContextKey, viewer)
	}
	if err := handler(c); err != nil {
		f.t.Fatalf("%s %s: %v", method, path, err)
	}
	return rec
}

// ask starts a change, with the step-up it requires already done.
func (f *emailFlow) ask(personID int64, newEmail string) *httptest.ResponseRecorder {
	f.t.Helper()
	if err := f.te.markStepUp(personID); err != nil {
		f.t.Fatal(err)
	}
	return f.call(http.MethodPatch, "/email/change",
		`{"email":"`+newEmail+`"}`, testViewer(personID, 7), f.h.Edit)
}

func (f *emailFlow) confirm(personID int64, code string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.call(http.MethodPost, "/email/change/confirm",
		`{"code":"`+code+`"}`, testViewer(personID, 7), f.h.Confirm)
}

func (f *emailFlow) lastToken() *entities.Token {
	f.t.Helper()
	added := f.te.Tokens.added
	if len(added) == 0 {
		f.t.Fatal("no code was issued")
	}
	return added[len(added)-1]
}

func TestEmailChangeIsApprovedByACodeSentToTheCurrentAddress(t *testing.T) {
	user := signInUser(t, 42, "ada@example.com", "ada")
	f := newEmailFlow(t, user)

	if rec := f.ask(42, "Ada.Lovelace@Example.org"); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	token := f.lastToken()
	if token.Type != constants.TokenTypeEmailChangeCode || token.ExpiredAfter != constants.TokenExpiryTimeCode {
		t.Errorf("type %q lifetime %d", token.Type, token.ExpiredAfter)
	}
	if token.RecipientEmail != "ada@example.com" || token.NewEmail != "ada.lovelace@example.org" {
		t.Errorf("sent to %q, moving to %q", token.RecipientEmail, token.NewEmail)
	}

	// To the CURRENT address, naming the new one, with no link.
	mail := f.te.Mail.waitFor(t, 1)[0]
	data, _ := mail.Data.(map[string]string)
	if mail.To[0] != "ada@example.com" {
		t.Errorf("the code went to %v", mail.To)
	}
	if data["code"] != token.Code || data["actionURL"] != "" ||
		!strings.Contains(data["body"], "ada.lovelace@example.org") {
		t.Errorf("mail data %v", data)
	}

	// Nothing has changed yet, and the step-up was spent on the request.
	if user.Email != "ada@example.com" {
		t.Fatal("the address changed before the code was entered")
	}
	if err := f.te.requireStepUp(42); err == nil {
		t.Error("the step-up survived the request")
	}

	if rec := f.confirm(42, token.Code); rec.Code != http.StatusNoContent {
		t.Fatalf("confirm status = %d, want 204", rec.Code)
	}

	if user.Email != "ada.lovelace@example.org" {
		t.Errorf("address is %q", user.Email)
	}
	if !token.IsUsed {
		t.Error("the code was not used up")
	}

	// Both addresses are told.
	sent := f.te.Mail.waitFor(t, 3)
	recipients := sent[1].To[0] + " " + sent[2].To[0]
	if !strings.Contains(recipients, "ada@example.com") || !strings.Contains(recipients, "ada.lovelace@example.org") {
		t.Errorf("notices went to %q", recipients)
	}

	// The same code does nothing a second time.
	if rec := f.confirm(42, token.Code); rec.Code != http.StatusGone {
		t.Errorf("second confirm status = %d, want 410", rec.Code)
	}
}

func TestEmailChangeNeedsAStepUpToStart(t *testing.T) {
	f := newEmailFlow(t, signInUser(t, 42, "ada@example.com", "ada"))

	rec := f.call(http.MethodPatch, "/email/change", `{"email":"new-address@example.org"}`,
		testViewer(42, 7), f.h.Edit)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if len(f.te.Tokens.added) != 0 {
		t.Error("a code was sent with no step-up")
	}
}

func TestEmailChangeWrongCodeSaysHowManyTriesAreLeft(t *testing.T) {
	user := signInUser(t, 42, "ada@example.com", "ada")
	f := newEmailFlow(t, user)
	f.ask(42, "new-address@example.org")
	code := f.lastToken().Code

	// Not six digits: refused, and not counted.
	if rec := f.confirm(42, "12ab"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed: status = %d, want 400", rec.Code)
	}
	if f.lastToken().CodeAttempts != 0 {
		t.Error("a malformed code cost a try")
	}

	for try := 1; try < constants.OTPMaxAttempts; try++ {
		rec := f.confirm(42, wrongCode(code))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("try %d: status = %d, want 422", try, rec.Code)
		}
		var body map[string]int
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["remaining"] != constants.OTPMaxAttempts-try {
			t.Errorf("try %d: remaining = %d", try, body["remaining"])
		}
	}

	if rec := f.confirm(42, wrongCode(code)); rec.Code != http.StatusGone {
		t.Errorf("the fifth wrong code: status = %d, want 410", rec.Code)
	}
	if rec := f.confirm(42, code); rec.Code != http.StatusGone {
		t.Errorf("the right code after the cap: status = %d, want 410", rec.Code)
	}
	if user.Email != "ada@example.com" {
		t.Error("the address changed")
	}
}

func TestEmailChangeCodeExpiresAndIsReplacedByANewRequest(t *testing.T) {
	user := signInUser(t, 42, "ada@example.com", "ada")
	f := newEmailFlow(t, user)

	f.ask(42, "first-choice@example.org")
	first := f.lastToken()

	// Asking again, for another address, retires the first request.
	f.ask(42, "second-choice@example.org")
	second := f.lastToken()
	if !first.IsUsed {
		t.Fatal("the first request is still live")
	}

	if rec := f.confirm(42, first.Code); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("the first code against the second request: status = %d, want 422", rec.Code)
	}

	second.StartTime = time.Now().Add(-16 * time.Minute)
	if rec := f.confirm(42, second.Code); rec.Code != http.StatusGone {
		t.Errorf("an expired code: status = %d, want 410", rec.Code)
	}
	if user.Email != "ada@example.com" {
		t.Error("the address changed")
	}
}

// The code belongs to the person it was issued to. Somebody else, signed in
// as themselves, cannot use it, and has nothing pending of their own.
func TestEmailChangeCodeOnlyWorksForItsOwner(t *testing.T) {
	ada := signInUser(t, 42, "ada@example.com", "ada")
	grace := signInUser(t, 43, "grace@example.com", "grace")
	f := newEmailFlow(t, ada, grace)

	f.ask(42, "new-address@example.org")

	if rec := f.confirm(43, f.lastToken().Code); rec.Code != http.StatusGone {
		t.Errorf("status = %d, want 410", rec.Code)
	}
	if ada.Email != "ada@example.com" || grace.Email != "grace@example.com" {
		t.Error("an address changed")
	}
	if f.lastToken().CodeAttempts != 0 {
		t.Error("somebody else's guess was counted against the owner's code")
	}
}

func TestEmailChangeRefusesAnAddressTakenInTheMeantime(t *testing.T) {
	ada := signInUser(t, 42, "ada@example.com", "ada")
	f := newEmailFlow(t, ada)
	f.ask(42, "wanted@example.org")

	// Somebody signs up with it before the code is entered.
	f.te.Users.byEmail["wanted@example.org"] = signInUser(t, 44, "wanted@example.org", "other")

	if rec := f.confirm(42, f.lastToken().Code); rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if ada.Email != "ada@example.com" {
		t.Error("the address changed")
	}
}

// A sign-in code must not approve an email change.
func TestEmailChangeRefusesACodeOfAnotherKind(t *testing.T) {
	ada := signInUser(t, 42, "ada@example.com", "ada")
	f := newEmailFlow(t, ada)

	newOAuthHandler(f.te.Env).sendLoginCode("ada@example.com", constants.CodeChannelEmail)

	if rec := f.confirm(42, f.lastToken().Code); rec.Code != http.StatusGone {
		t.Errorf("status = %d, want 410", rec.Code)
	}
}

func TestEmailChangeConfirmNeedsASession(t *testing.T) {
	f := newEmailFlow(t, signInUser(t, 42, "ada@example.com", "ada"))

	rec := f.call(http.MethodPost, "/email/change/confirm", `{"code":"123456"}`, nil, f.h.Confirm)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
