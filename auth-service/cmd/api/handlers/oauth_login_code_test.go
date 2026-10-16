package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"auth/constants"
	"auth/data/entities"

	libs_constants "anahita-libs/constants"

	"github.com/labstack/echo/v4"
)

// ---------------------------------------------------------------------
// Scaffolding
// ---------------------------------------------------------------------

const testCSRF = "csrf-for-tests"

// codeFlow is one browser walking the sign-in-with-a-code pages.
type codeFlow struct {
	t  *testing.T
	te *testEnv
	h  *oauthHandler
	e  *echo.Echo
}

func newCodeFlow(t *testing.T, users ...*entities.User) *codeFlow {
	t.Helper()
	serviceRoot(t)

	return anotherCodeFlow(t, users...)
}

// anotherCodeFlow is a second browser, and a separate installation, in a test
// that already has one. It does not change directory again.
func anotherCodeFlow(t *testing.T, users ...*entities.User) *codeFlow {
	t.Helper()

	te := newTestEnv(users...)
	te.Config.SecureCookieKey = "0123456789abcdef0123456789abcdef"
	te.Sess.values[sessionKeyLoginCSRF] = testCSRF

	return &codeFlow{t: t, te: te, h: newOAuthHandler(te.Env), e: newTestEcho()}
}

// signInUser is an account that can sign in: enabled, with a confirmed address.
func signInUser(t *testing.T, id int64, email, alias string) *entities.User {
	t.Helper()
	u := testUser(t, id, email, "correct horse battery staple")
	u.Alias = alias
	u.Enabled = true
	u.MarkEmailVerified()
	return u
}

func (f *codeFlow) post(path string, form url.Values, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	f.t.Helper()
	if form.Get("csrf_token") == "" {
		form.Set("csrf_token", testCSRF)
	}
	c, rec := postForm(f.e, path, form)
	if err := handler(c); err != nil {
		f.t.Fatalf("POST %s: %v", path, err)
	}
	answers.Handler(handler, rec)
	return rec
}

func (f *codeFlow) get(path string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	f.t.Helper()
	c, rec := get(f.e, path)
	if err := handler(c); err != nil {
		f.t.Fatalf("GET %s: %v", path, err)
	}
	answers.Handler(handler, rec)
	return rec
}

// ask submits the identifier, which with one channel also sends the code.
func (f *codeFlow) ask(identifier string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.post("/oauth/login/code", url.Values{"identifier": {identifier}}, f.h.AddLoginCode)
}

func (f *codeFlow) submit(code string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.post("/oauth/login/code/verify", url.Values{"code": {code}}, f.h.AddLoginCodeVerify)
}

// lastCode is the code most recently issued, as it was handed to the mailer.
func (f *codeFlow) lastCode() string {
	f.t.Helper()
	added := f.te.Tokens.added
	if len(added) == 0 {
		f.t.Fatal("no code was issued")
	}
	return added[len(added)-1].Code
}

func (f *codeFlow) lastToken() *entities.Token {
	f.t.Helper()
	added := f.te.Tokens.added
	if len(added) == 0 {
		f.t.Fatal("no code was issued")
	}
	return added[len(added)-1]
}

// wrongCode is six digits that are not the given code.
func wrongCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

func (f *codeFlow) signedIn() bool {
	return f.te.Sess.values[libs_constants.UserSessionKey] != ""
}

func (f *codeFlow) pending() *pendingCodeState {
	raw := f.te.Sess.values[sessionKeyPendingCode]
	if raw == "" {
		return nil
	}
	var state pendingCodeState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		f.t.Fatalf("pending state: %v", err)
	}
	return &state
}

func wantRedirect(t *testing.T, rec *httptest.ResponseRecorder, location string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want a redirect; body: %.200s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != location {
		t.Fatalf("redirected to %q, want %q", got, location)
	}
}

// ---------------------------------------------------------------------
// The flow
// ---------------------------------------------------------------------

func TestLoginCodeSignsInWithTheEmailedCode(t *testing.T) {
	user := signInUser(t, 7, "ada@example.com", "ada")
	f := newCodeFlow(t, user)

	wantRedirect(t, f.ask("  Ada@Example.com "), "/oauth/login/code/verify")

	token := f.lastToken()
	if token.Type != constants.TokenTypeLoginOTP || token.Channel != constants.CodeChannelEmail {
		t.Errorf("token type %q channel %q", token.Type, token.Channel)
	}
	if token.ExpiredAfter != constants.TokenExpiryTimeOTP {
		t.Errorf("lifetime %d s, want %d", token.ExpiredAfter, constants.TokenExpiryTimeOTP)
	}
	if token.CodeHash == "" || strings.Contains(token.CodeHash, token.Code) {
		t.Errorf("stored value %q for code %q is not a hash of it", token.CodeHash, token.Code)
	}

	mail := f.te.Mail.waitFor(t, 1)[0]
	if len(mail.To) != 1 || mail.To[0] != "ada@example.com" {
		t.Errorf("sent to %v", mail.To)
	}
	if !strings.Contains(mail.Subject, token.Code) {
		t.Errorf("subject %q does not carry the code", mail.Subject)
	}
	data, _ := mail.Data.(map[string]string)
	// The code travels in its own field, which the template boxes, and
	// not buried in the sentence under it.
	if data["code"] != token.Code {
		t.Errorf("code field %q, want %q", data["code"], token.Code)
	}
	if strings.Contains(data["body"], token.Code) || !strings.Contains(data["body"], "never ask") {
		t.Errorf("body %q", data["body"])
	}
	if data["actionURL"] != "" {
		t.Errorf("the message carries a link: %q", data["actionURL"])
	}

	if f.get("/oauth/login/code/verify", f.h.ReadLoginCodeVerify).Code != http.StatusOK {
		t.Fatal("the verify page did not render")
	}

	wantRedirect(t, f.submit(token.Code), f.te.Config.GetFrontendURL())

	if !f.signedIn() {
		t.Error("no session after a correct code")
	}
	if !token.IsUsed {
		t.Error("the code was not used up")
	}
	if f.pending() != nil {
		t.Error("the pending state outlived the sign-in")
	}
}

func TestLoginCodeAcceptsAUsernameWithoutShowingItsAddress(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))

	wantRedirect(t, f.ask("ada"), "/oauth/login/code/verify")

	body := f.get("/oauth/login/code/verify", f.h.ReadLoginCodeVerify).Body.String()
	if strings.Contains(body, "ada@example.com") {
		t.Error("the page shows the address behind a username")
	}

	wantRedirect(t, f.submit(f.lastCode()), f.te.Config.GetFrontendURL())
	if !f.signedIn() {
		t.Error("no session after a correct code")
	}
}

// An address with no account must be indistinguishable from one with: the
// same redirect, the same page, the same five tries. It is just never mailed.
func TestLoginCodeUnknownAccountLooksTheSame(t *testing.T) {
	known := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	wantRedirect(t, known.ask("ada@example.com"), "/oauth/login/code/verify")
	knownPage := known.get("/oauth/login/code/verify", known.h.ReadLoginCodeVerify).Body.String()

	f := anotherCodeFlow(t)
	wantRedirect(t, f.ask("nobody@example.com"), "/oauth/login/code/verify")

	if len(f.te.Tokens.added) != 0 {
		t.Error("a code was issued for an address with no account")
	}
	f.te.Mail.expectNoneMatching(t, "")

	page := f.get("/oauth/login/code/verify", f.h.ReadLoginCodeVerify).Body.String()
	if strings.ReplaceAll(page, "nobody@example.com", "X") != strings.ReplaceAll(knownPage, "ada@example.com", "X") {
		t.Error("the verify page differs between a known and an unknown address")
	}

	for try := 1; try < constants.OTPMaxAttempts; try++ {
		rec := f.submit("123456")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "not correct") {
			t.Fatalf("try %d: status %d", try, rec.Code)
		}
	}

	wantRedirect(t, f.submit("123456"), "/oauth/login/code?error=code_expired")
	if f.signedIn() {
		t.Error("signed in to an account that does not exist")
	}
}

func TestLoginCodeWrongCodeRunsOutAfterFiveTries(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.ask("ada@example.com")
	code := f.lastCode()

	for try := 1; try < constants.OTPMaxAttempts; try++ {
		rec := f.submit(wrongCode(code))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "not correct") {
			t.Fatalf("try %d: status %d body %.120s", try, rec.Code, rec.Body.String())
		}
	}

	if got := f.lastToken().CodeAttempts; got != constants.OTPMaxAttempts-1 {
		t.Fatalf("%d attempts recorded, want %d", got, constants.OTPMaxAttempts-1)
	}

	wantRedirect(t, f.submit(wrongCode(code)), "/oauth/login/code?error=code_expired")

	// Even the right code is dead now. Put the state back, as a second tab
	// would still have it.
	f.te.Sess.values[sessionKeyPendingCode] = mustJSON(t, pendingCodeState{
		Identifier: "ada@example.com", Sent: true, TokenID: f.lastToken().ID,
		ExpiresAt: time.Now().Add(time.Minute),
	})
	wantRedirect(t, f.submit(code), "/oauth/login/code?error=code_expired")

	if f.signedIn() {
		t.Error("signed in after running out of tries")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLoginCodeMalformedSubmissionCostsNothing(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.ask("ada@example.com")

	for _, typed := range []string{"", "12345", "1234567", "12a456", "12 456"} {
		rec := f.submit(typed)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "6-digit code") {
			t.Fatalf("%q: status %d", typed, rec.Code)
		}
	}

	if got := f.lastToken().CodeAttempts; got != 0 {
		t.Errorf("%d attempts spent on submissions that could not be a code", got)
	}
}

func TestLoginCodeExpiredCodeIsRefused(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.ask("ada@example.com")
	code := f.lastCode()

	f.lastToken().StartedAt = time.Now().Add(-11 * time.Minute)

	wantRedirect(t, f.submit(code), "/oauth/login/code?error=code_expired")
	if f.signedIn() {
		t.Error("signed in with an expired code")
	}
}

func TestLoginCodeWorksOnce(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.ask("ada@example.com")
	code, tokenID := f.lastCode(), f.lastToken().ID

	wantRedirect(t, f.submit(code), f.te.Config.GetFrontendURL())

	// A second browser holding the same pending state.
	delete(f.te.Sess.values, libs_constants.UserSessionKey)
	f.te.Sess.values[sessionKeyLoginCSRF] = testCSRF
	f.te.Sess.values[sessionKeyPendingCode] = mustJSON(t, pendingCodeState{
		Identifier: "ada@example.com", Sent: true, TokenID: tokenID,
		ExpiresAt: time.Now().Add(time.Minute),
	})

	wantRedirect(t, f.submit(code), "/oauth/login/code?error=code_expired")
	if f.signedIn() {
		t.Error("the same code signed in twice")
	}
}

func TestLoginCodeResendRetiresTheEarlierCode(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.ask("ada@example.com")
	first, firstCode := f.lastToken(), f.lastCode()

	wantRedirect(t,
		f.post("/oauth/login/code/resend", url.Values{}, f.h.AddLoginCodeResend),
		"/oauth/login/code/verify")

	second := f.lastToken()
	if second == first {
		t.Fatal("no new code was issued")
	}
	if !first.IsUsed {
		t.Error("the first code is still live")
	}
	if f.pending().TokenID != second.ID {
		t.Error("the page still points at the first code")
	}

	if _, _, err := f.h.verifyCode(first.ID, constants.TokenTypeLoginOTP, firstCode); !errors.Is(err, errCodeGone) {
		t.Errorf("the retired code: %v, want errCodeGone", err)
	}

	wantRedirect(t, f.submit(second.Code), f.te.Config.GetFrontendURL())
}

// The sixth request in an hour for one address sends nothing, and looks
// exactly like the five before it.
func TestLoginCodeSendsAreLimitedPerAddress(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))

	limit := int(rateLimits[rateLimitScopeLoginCodeEmail].max)
	for range limit {
		wantRedirect(t, f.ask("ada@example.com"), "/oauth/login/code/verify")
	}
	if len(f.te.Tokens.added) != limit {
		t.Fatalf("%d codes issued, want %d", len(f.te.Tokens.added), limit)
	}

	wantRedirect(t, f.ask("ada@example.com"), "/oauth/login/code/verify")
	if len(f.te.Tokens.added) != limit {
		t.Error("a code was issued past the limit")
	}
	if state := f.pending(); state == nil || !state.Sent || state.TokenID != 0 {
		t.Errorf("state after a limited request: %+v", state)
	}
}

func TestLoginCodeMethodPageNeverSends(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.te.Sess.values[sessionKeyPendingCode] = mustJSON(t, pendingCodeState{
		Identifier: "ada@example.com", ExpiresAt: time.Now().Add(time.Minute),
	})

	wantRedirect(t, f.get("/oauth/login/code/method", f.h.ReadLoginCodeMethod), "/oauth/login/code")
	if len(f.te.Tokens.added) != 0 {
		t.Error("a GET issued a code")
	}
}

func TestLoginCodeFormsNeedTheSessionToken(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))

	rec := f.post("/oauth/login/code",
		url.Values{"identifier": {"ada@example.com"}, "csrf_token": {"forged"}}, f.h.AddLoginCode)
	wantRedirect(t, rec, "/oauth/login/code?error=session_expired")
	if len(f.te.Tokens.added) != 0 {
		t.Error("a forged form issued a code")
	}

	f.ask("ada@example.com")
	rec = f.post("/oauth/login/code/verify",
		url.Values{"code": {f.lastCode()}, "csrf_token": {"forged"}}, f.h.AddLoginCodeVerify)
	wantRedirect(t, rec, "/oauth/login/code?error=session_expired")
	if f.signedIn() {
		t.Error("a forged form signed in")
	}
}

// ---------------------------------------------------------------------
// The account, as it is when the code is checked
// ---------------------------------------------------------------------

func TestLoginCodeDisabledAccountIsSentNothing(t *testing.T) {
	user := signInUser(t, 7, "ada@example.com", "ada")
	user.Enabled = false
	f := newCodeFlow(t, user)

	wantRedirect(t, f.ask("ada@example.com"), "/oauth/login/code/verify")
	if len(f.te.Tokens.added) != 0 {
		t.Error("a disabled account was issued a code")
	}
}

func TestLoginCodeAccountDisabledAfterSendingIsRefused(t *testing.T) {
	user := signInUser(t, 7, "ada@example.com", "ada")
	f := newCodeFlow(t, user)
	f.ask("ada@example.com")

	user.Enabled = false

	rec := f.submit(f.lastCode())
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "could not sign you in") {
		t.Fatalf("status %d body %.200s", rec.Code, rec.Body.String())
	}
	if f.signedIn() {
		t.Error("a disabled account signed in")
	}
	if !f.lastToken().IsUsed {
		t.Error("the code was left usable")
	}
}

func TestLoginCodeDeletedAccountIsOfferedRestore(t *testing.T) {
	user := signInUser(t, 7, "ada@example.com", "ada")
	deleted := time.Now().Add(-time.Hour)
	user.DeletedAt = &deleted
	f := newCodeFlow(t, user)
	f.ask("ada@example.com")

	rec := f.submit(f.lastCode())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if f.te.Sess.values[sessionKeyPendingRestore] != "ada@example.com" {
		t.Error("the restore step was not started")
	}
	if f.signedIn() {
		t.Error("a deleted account signed in without restoring")
	}
}

func TestLoginCodeArchivedAccountIsRefused(t *testing.T) {
	user := signInUser(t, 7, "ada@example.com", "ada")
	archived := time.Now().Add(-time.Hour)
	user.ArchivedAt = &archived
	f := newCodeFlow(t, user)
	f.ask("ada@example.com")

	rec := f.submit(f.lastCode())
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "archived") {
		t.Fatalf("status %d body %.200s", rec.Code, rec.Body.String())
	}
	if f.signedIn() {
		t.Error("an archived account signed in")
	}
}

func TestLoginCodeConfirmsAnUnconfirmedAddress(t *testing.T) {
	user := signInUser(t, 7, "ada@example.com", "ada")
	user.EmailVerifiedAt = nil
	f := newCodeFlow(t, user)
	f.ask("ada@example.com")

	wantRedirect(t, f.submit(f.lastCode()), f.te.Config.GetFrontendURL())

	if !user.IsEmailVerified() {
		t.Error("the address is still unconfirmed after a code sent to it came back")
	}
	if !f.signedIn() {
		t.Error("no session")
	}
}

// The code stands in for the password. The second factor is still asked for.
func TestLoginCodeGoesOnToTwoFactor(t *testing.T) {
	user := signInUser(t, 7, "ada@example.com", "ada")
	user.TotpEnabled = true
	f := newCodeFlow(t, user)
	f.ask("ada@example.com")

	wantRedirect(t, f.submit(f.lastCode()), "/oauth/login/totp")

	if f.signedIn() {
		t.Error("signed in without the second factor")
	}
	if f.te.Sess.values[sessionKeyPendingTOTP] == "" {
		t.Error("the two-factor step was not started")
	}
}

// ---------------------------------------------------------------------
// Codes, under concurrency and across types
// ---------------------------------------------------------------------

func TestCodeIssuedForOnePurposeIsRefusedForAnother(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.ask("ada@example.com")

	_, _, err := f.h.verifyCode(f.lastToken().ID, constants.TokenTypePasswordReset, f.lastCode())
	if !errors.Is(err, errCodeGone) {
		t.Errorf("a sign-in code checked as a password reset: %v, want errCodeGone", err)
	}
	if f.lastToken().CodeAttempts != 0 {
		t.Error("an attempt was spent on a code of the wrong type")
	}
}

// Twenty wrong guesses at once must spend exactly five attempts between them.
func TestCodeGuessesArrivingTogetherShareNoAttempt(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.ask("ada@example.com")
	token, code := f.lastToken(), f.lastCode()

	var wg sync.WaitGroup
	var mu sync.Mutex
	mismatches, gone := 0, 0

	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := f.h.verifyCode(token.ID, constants.TokenTypeLoginOTP, wrongCode(code))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case errors.Is(err, errCodeMismatch):
				mismatches++
			case errors.Is(err, errCodeGone):
				gone++
			default:
				t.Errorf("unexpected result: %v", err)
			}
		}()
	}
	wg.Wait()

	if token.CodeAttempts != constants.OTPMaxAttempts {
		t.Errorf("%d attempts recorded, want exactly %d", token.CodeAttempts, constants.OTPMaxAttempts)
	}
	if mismatches+gone != 20 || mismatches > constants.OTPMaxAttempts {
		t.Errorf("%d mismatches and %d refusals", mismatches, gone)
	}

	if _, _, err := f.h.verifyCode(token.ID, constants.TokenTypeLoginOTP, code); !errors.Is(err, errCodeGone) {
		t.Errorf("the right code after the cap: %v, want errCodeGone", err)
	}
}

// Of several correct submissions at once, exactly one uses the code.
func TestCodeIsConsumedByExactlyOneSubmission(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	f.ask("ada@example.com")
	token, code := f.lastToken(), f.lastCode()

	var wg sync.WaitGroup
	var mu sync.Mutex
	consumed := 0

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			verified, _, err := f.h.verifyCode(token.ID, constants.TokenTypeLoginOTP, code)
			if err != nil {
				return
			}
			if f.h.consumeCode(verified, 7) {
				mu.Lock()
				consumed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if consumed != 1 {
		t.Errorf("the code was used %d times, want exactly once", consumed)
	}
}

// ---------------------------------------------------------------------
// Password sign-in: the token and the limit added alongside
// ---------------------------------------------------------------------

func passwordForm(identifier, password string) url.Values {
	return url.Values{"identifier": {identifier}, "password": {password}}
}

func TestPasswordLoginNeedsTheSessionToken(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))

	form := passwordForm("ada@example.com", "correct horse battery staple")
	form.Set("csrf_token", "forged")

	rec := f.post("/oauth/login", form, f.h.AddCredentials)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "expired") {
		t.Fatalf("status %d", rec.Code)
	}
	if f.signedIn() {
		t.Error("a forged form signed in")
	}

	wantRedirect(t,
		f.post("/oauth/login", passwordForm("ada@example.com", "correct horse battery staple"), f.h.AddCredentials),
		f.te.Config.GetFrontendURL())
	if !f.signedIn() {
		t.Error("the genuine form did not sign in")
	}
}

func TestPasswordLoginIsLimitedPerIdentifier(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))

	limit := int(rateLimits[rateLimitScopeLoginIdentifier].max)
	for try := 1; try <= limit; try++ {
		rec := f.post("/oauth/login", passwordForm("Ada@example.com", "a wrong password"), f.h.AddCredentials)
		if !strings.Contains(rec.Body.String(), "Invalid email or password") {
			t.Fatalf("try %d: %.200s", try, rec.Body.String())
		}
	}

	// Past the limit even the right password is refused, and the page says
	// why without saying whether the account exists.
	rec := f.post("/oauth/login", passwordForm("ada@example.com", "correct horse battery staple"), f.h.AddCredentials)
	if !strings.Contains(rec.Body.String(), "Too many sign-in attempts") {
		t.Fatalf("not limited: %.200s", rec.Body.String())
	}
	if f.signedIn() {
		t.Error("signed in past the limit")
	}

	// The same wording for an address with no account.
	for range limit {
		f.post("/oauth/login", passwordForm("nobody@example.com", "a wrong password"), f.h.AddCredentials)
	}
	rec = f.post("/oauth/login", passwordForm("nobody@example.com", "a wrong password"), f.h.AddCredentials)
	if !strings.Contains(rec.Body.String(), "Too many sign-in attempts") {
		t.Error("an unknown address is not limited the same way")
	}
}

func TestPasswordLoginSuccessClearsTheCount(t *testing.T) {
	f := newCodeFlow(t, signInUser(t, 7, "ada@example.com", "ada"))
	limit := int(rateLimits[rateLimitScopeLoginIdentifier].max)

	for range limit - 1 {
		f.post("/oauth/login", passwordForm("ada@example.com", "a wrong password"), f.h.AddCredentials)
	}

	wantRedirect(t,
		f.post("/oauth/login", passwordForm("ada@example.com", "correct horse battery staple"), f.h.AddCredentials),
		f.te.Config.GetFrontendURL())

	if got := f.te.Sess.counters[rateLimitKey(rateLimitScopeLoginIdentifier, "ada@example.com")]; got != 0 {
		t.Errorf("count after a successful sign-in = %d, want 0", got)
	}
}
