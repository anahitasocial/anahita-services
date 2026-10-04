package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"auth/constants"
	"auth/data/entities"
)

const (
	oldPassword = "correct horse battery staple"
	newPassword = "a different and longer passphrase"
)

// resetFlow is one browser at the forgotten-password pages.
type resetFlow struct {
	*codeFlow
	h *passwordHandler
}

func newResetFlow(t *testing.T, users ...*entities.User) *resetFlow {
	t.Helper()
	base := newCodeFlow(t, users...)
	return &resetFlow{codeFlow: base, h: newPasswordHandler(base.te.Env)}
}

func anotherResetFlow(t *testing.T, users ...*entities.User) *resetFlow {
	t.Helper()
	base := anotherCodeFlow(t, users...)
	return &resetFlow{codeFlow: base, h: newPasswordHandler(base.te.Env)}
}

func (f *resetFlow) forgot(email string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.post("/password/forgot", url.Values{"email": {email}}, f.h.Forgot)
}

func (f *resetFlow) verify(code string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.post("/password/reset/verify", url.Values{"code": {code}}, f.h.Verify)
}

func (f *resetFlow) setPassword(password string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.post("/password/reset", url.Values{"password": {password}}, f.h.Reset)
}

func TestForgotResetsThePasswordWithTheEmailedCode(t *testing.T) {
	user := signInUser(t, 42, "ada@example.com", "ada")
	f := newResetFlow(t, user)

	wantRedirect(t, f.forgot(" Ada@Example.com "), "/password/reset/verify")

	token := f.lastToken()
	if token.Type != constants.TokenTypePasswordResetCode {
		t.Errorf("type = %q", token.Type)
	}
	// Fifteen minutes, as the link it replaces had.
	if token.ExpiredAfter != 15*60 {
		t.Errorf("lifetime = %d seconds, want 900", token.ExpiredAfter)
	}
	if token.RecipientEmail != "ada@example.com" || token.CreatedBy != 42 {
		t.Errorf("recipient %q creator %d", token.RecipientEmail, token.CreatedBy)
	}
	if token.CodeHash == "" || token.CodeHash == token.Code {
		t.Error("the code is not stored as a hash")
	}

	mail := f.te.Mail.waitFor(t, 1)[0]
	data, _ := mail.Data.(map[string]string)
	if len(mail.To) != 1 || mail.To[0] != "ada@example.com" {
		t.Errorf("addressed to %v", mail.To)
	}
	if data["code"] != token.Code || data["actionURL"] != "" || !strings.Contains(mail.Subject, token.Code) {
		t.Errorf("mail subject %q data %v", mail.Subject, data)
	}

	if page := f.get("/password/reset/verify", f.h.VerifyForm); page.Code != http.StatusOK {
		t.Fatalf("the code page: status %d", page.Code)
	}

	// Before the code, the new-password form is not reachable.
	wantRedirect(t, f.get("/password/reset", f.h.ResetForm), "/password/forgot")

	wantRedirect(t, f.verify(token.Code), "/password/reset")

	if token.IsUsed {
		t.Error("the code was used up before the password was set")
	}

	form := f.get("/password/reset", f.h.ResetForm)
	if form.Code != http.StatusOK || !strings.Contains(form.Body.String(), "Choose a new password") {
		t.Fatalf("the new-password form: status %d", form.Code)
	}

	rec := f.setPassword(newPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %.200s", rec.Code, rec.Body.String())
	}

	if f.te.Users.editedID != 42 || f.te.Users.editCalls != 1 {
		t.Fatalf("password written %d time(s) for %d", f.te.Users.editCalls, f.te.Users.editedID)
	}
	if !token.IsUsed {
		t.Error("the code is still live after the password was set")
	}
	if f.te.Sess.values[sessionKeyPasswordReset] != "" {
		t.Error("the browser can still set a password")
	}
	if f.signedIn() {
		t.Error("resetting a password signed the person in")
	}

	// And it cannot be done twice.
	wantRedirect(t, f.setPassword("yet another long passphrase"), "/password/forgot?error=expired")
	if f.te.Users.editCalls != 1 {
		t.Error("the password was set a second time")
	}
}

// The page after "forgot" must not depend on whether the address exists:
// compared on the bytes, because a difference in wording is as much of an
// oracle as a difference in status.
func TestForgotDoesNotRevealWhetherAnAccountExists(t *testing.T) {
	known := newResetFlow(t, signInUser(t, 42, "ada@example.com", "ada"))
	wantRedirect(t, known.forgot("ada@example.com"), "/password/reset/verify")
	knownPage := known.get("/password/reset/verify", known.h.VerifyForm).Body.String()

	unknown := anotherResetFlow(t)
	wantRedirect(t, unknown.forgot("nobody@example.com"), "/password/reset/verify")
	unknownPage := unknown.get("/password/reset/verify", unknown.h.VerifyForm).Body.String()

	if strings.ReplaceAll(knownPage, "ada@example.com", "") != strings.ReplaceAll(unknownPage, "nobody@example.com", "") {
		t.Error("the code page differs between a registered and an unregistered address")
	}

	if len(known.te.Tokens.added) != 1 || len(unknown.te.Tokens.added) != 0 {
		t.Errorf("codes issued: known %d, unknown %d", len(known.te.Tokens.added), len(unknown.te.Tokens.added))
	}
	unknown.te.Mail.expectNoneMatching(t, "")

	// Five tries, then it is over, exactly as for a real account.
	for try := 1; try < constants.OTPMaxAttempts; try++ {
		if rec := unknown.verify("123456"); !strings.Contains(rec.Body.String(), "not correct") {
			t.Fatalf("try %d: %.160s", try, rec.Body.String())
		}
	}
	wantRedirect(t, unknown.verify("123456"), "/password/forgot?error=expired")
}

// Over budget, nothing is sent, and the answer is the same, or the limiter
// becomes the oracle the handler avoids everywhere else.
func TestForgotThrottlesWithoutChangingTheResponse(t *testing.T) {
	f := newResetFlow(t, signInUser(t, 42, "ada@example.com", "ada"))

	// The per-address budget is 3 an hour, so the fourth is over.
	for range 4 {
		wantRedirect(t, f.forgot("ada@example.com"), "/password/reset/verify")
	}

	if len(f.te.Tokens.added) != 3 {
		t.Errorf("%d codes issued, want 3", len(f.te.Tokens.added))
	}
}

func TestForgotRejectsAMalformedAddressWithoutIssuingAnything(t *testing.T) {
	f := newResetFlow(t)

	rec := f.forgot("not-an-address")
	if !strings.Contains(rec.Body.String(), "valid email address") {
		t.Error("the form did not come back with a validation error")
	}
	if len(f.te.Tokens.added) != 0 {
		t.Errorf("%d codes issued, want 0", len(f.te.Tokens.added))
	}
}

func TestForgotNeedsTheFormToken(t *testing.T) {
	f := newResetFlow(t, signInUser(t, 42, "ada@example.com", "ada"))

	rec := f.post("/password/forgot",
		url.Values{"email": {"ada@example.com"}, "csrf_token": {"forged"}}, f.h.Forgot)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "expired") {
		t.Fatalf("status %d", rec.Code)
	}
	if len(f.te.Tokens.added) != 0 {
		t.Error("a forged form sent a code")
	}
}

// Asking again retires the first code, including for a browser that had
// already entered it.
func TestForgotRetiresTheEarlierCode(t *testing.T) {
	f := newResetFlow(t, signInUser(t, 42, "ada@example.com", "ada"))

	f.forgot("ada@example.com")
	first := f.lastToken()
	wantRedirect(t, f.verify(first.Code), "/password/reset")
	verifiedStep := f.te.Sess.values[sessionKeyPasswordReset]

	// The "send a new code" button sends no address.
	wantRedirect(t, f.post("/password/forgot", url.Values{}, f.h.Forgot), "/password/reset/verify")

	second := f.lastToken()
	if second == first || !first.IsUsed || second.IsUsed {
		t.Fatal("the first code is still live, or the new one is not")
	}

	// A second tab still holding the first code's verified step.
	f.te.Sess.values[sessionKeyPasswordReset] = verifiedStep
	wantRedirect(t, f.setPassword(newPassword), "/password/forgot?error=expired")
	if f.te.Users.editCalls != 0 {
		t.Error("a retired code set a password")
	}
}

func TestResetCodeWrongCodeRunsOut(t *testing.T) {
	f := newResetFlow(t, signInUser(t, 42, "ada@example.com", "ada"))
	f.forgot("ada@example.com")
	code := f.lastCode()

	if rec := f.verify("12"); !strings.Contains(rec.Body.String(), "6-digit code") {
		t.Fatal("a malformed code was not caught")
	}

	for try := 1; try < constants.OTPMaxAttempts; try++ {
		if rec := f.verify(wrongCode(code)); !strings.Contains(rec.Body.String(), "not correct") {
			t.Fatalf("try %d: %.160s", try, rec.Body.String())
		}
	}
	wantRedirect(t, f.verify(wrongCode(code)), "/password/forgot?error=expired")
	wantRedirect(t, f.get("/password/reset", f.h.ResetForm), "/password/forgot")
}

// A code sent to sign in must not open the password.
func TestResetRefusesACodeOfAnotherKind(t *testing.T) {
	f := newResetFlow(t, signInUser(t, 42, "ada@example.com", "ada"))
	f.forgot("ada@example.com")

	login := newOAuthHandler(f.te.Env)
	loginTokenID := login.sendLoginCode("ada@example.com", constants.CodeChannelEmail)
	loginCode := f.lastCode()

	// Even with the session pointed at the sign-in code's row.
	f.te.Sess.values[sessionKeyPasswordReset] = strings.Replace(
		f.te.Sess.values[sessionKeyPasswordReset], `"token_id":1`, `"token_id":`+itoa(loginTokenID), 1)

	wantRedirect(t, f.verify(loginCode), "/password/forgot?error=expired")
	wantRedirect(t, f.get("/password/reset", f.h.ResetForm), "/password/forgot")
}

func itoa(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	out := ""
	for ; n > 0; n /= 10 {
		out = string(digits[n%10]) + out
	}
	return out
}

func TestResetAfterCodeChecksTheNewPassword(t *testing.T) {
	f := newResetFlow(t, signInUser(t, 42, "ada@example.com", "ada"))
	f.forgot("ada@example.com")
	wantRedirect(t, f.verify(f.lastCode()), "/password/reset")

	if rec := f.setPassword("too short"); !strings.Contains(rec.Body.String(), "at least 15 characters") {
		t.Fatalf("a short password: %.200s", rec.Body.String())
	}
	if rec := f.setPassword(oldPassword); !strings.Contains(rec.Body.String(), "must be different") {
		t.Fatalf("the current password: %.200s", rec.Body.String())
	}
	if f.te.Users.editCalls != 0 || f.lastToken().IsUsed {
		t.Fatal("a refused password changed something")
	}

	// The same code still lets them correct it.
	if rec := f.setPassword(newPassword); rec.Code != http.StatusOK || f.te.Users.editCalls != 1 {
		t.Fatalf("status %d, %d writes", rec.Code, f.te.Users.editCalls)
	}
}

func TestResetWithoutACodeOrALinkGoesBackToTheStart(t *testing.T) {
	f := newResetFlow(t, signInUser(t, 42, "ada@example.com", "ada"))

	wantRedirect(t, f.get("/password/reset", f.h.ResetForm), "/password/forgot")
	wantRedirect(t, f.setPassword(newPassword), "/password/forgot?error=expired")
	wantRedirect(t, f.get("/password/reset/verify", f.h.VerifyForm), "/password/forgot")

	if f.te.Users.editCalls != 0 {
		t.Error("a password was set with no code and no link")
	}
}
