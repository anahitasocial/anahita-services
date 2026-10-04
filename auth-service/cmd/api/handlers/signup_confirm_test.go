package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/data/repositories"
)

// fakeSignupRequests holds requests to join, by address.
type fakeSignupRequests struct {
	repositories.SignupRequestsInterface

	byEmail map[string]*entities.SignupRequest
}

func (f *fakeSignupRequests) ReadByEmail(email string) (*entities.SignupRequest, error) {
	if sr, ok := f.byEmail[email]; ok {
		return sr, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeSignupRequests) MarkEmailVerified(id int64) error {
	for _, sr := range f.byEmail {
		if sr.ID == id {
			now := time.Now()
			sr.EmailVerifiedAt = &now
		}
	}
	return nil
}

// confirmFlow is one browser at the confirm-your-address pages.
type confirmFlow struct {
	*codeFlow
	h        *signupHandler
	requests *fakeSignupRequests
}

func newConfirmFlow(t *testing.T, users ...*entities.User) *confirmFlow {
	t.Helper()

	base := newCodeFlow(t, users...)
	requests := &fakeSignupRequests{byEmail: map[string]*entities.SignupRequest{}}
	base.te.Repos.SignupRequests = requests

	return &confirmFlow{codeFlow: base, h: newSignupHandler(base.te.Env), requests: requests}
}

// unconfirmedUser is an account straight out of the signup form.
func unconfirmedUser(t *testing.T, id int64, email, alias string) *entities.User {
	t.Helper()
	u := signInUser(t, id, email, alias)
	u.EmailVerifiedAt = nil
	return u
}

// begin does what POST /signup does once the account exists: sends the code
// and goes to the page waiting for it.
func (f *confirmFlow) begin(user *entities.User) {
	f.t.Helper()

	tokenID, err := f.h.sendConfirmationCode(user.Email, user.ID, user.Alias)
	if err != nil {
		f.t.Fatalf("sendConfirmationCode: %v", err)
	}

	c, rec := get(f.e, "/signup")
	if err := f.h.awaitConfirmation(c, user.Email, tokenID); err != nil {
		f.t.Fatal(err)
	}
	wantRedirect(f.t, rec, "/signup/confirm")
}

func (f *confirmFlow) confirm(code string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.post("/signup/confirm", url.Values{"code": {code}}, f.h.ConfirmCode)
}

func TestSignupIsConfirmedByTheEmailedCode(t *testing.T) {
	user := unconfirmedUser(t, 7, "ada@example.com", "ada")
	f := newConfirmFlow(t, user)
	f.begin(user)

	token := f.lastToken()
	if token.Type != constants.TokenTypeSignupConfirmCode || token.ExpiredAfter != constants.TokenExpiryTimeCode {
		t.Errorf("token type %q lifetime %d", token.Type, token.ExpiredAfter)
	}

	mail := f.te.Mail.waitFor(t, 1)[0]
	data, _ := mail.Data.(map[string]string)
	if data["code"] != token.Code || data["actionURL"] != "" {
		t.Errorf("mail data %v", data)
	}
	if !strings.Contains(mail.Subject, token.Code) {
		t.Errorf("subject %q", mail.Subject)
	}

	page := f.get("/signup/confirm", f.h.Confirm)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "ada@example.com") {
		t.Fatalf("the code page: status %d", page.Code)
	}

	rec := f.confirm(token.Code)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Email confirmed") {
		t.Fatalf("status %d body %.200s", rec.Code, rec.Body.String())
	}

	if !user.IsEmailVerified() {
		t.Error("the address is still unconfirmed")
	}
	if !token.IsUsed {
		t.Error("the code was not used up")
	}
	if f.signedIn() {
		t.Error("confirming an address signed the person in")
	}

	// The welcome follows, once.
	welcome := f.te.Mail.waitFor(t, 2)[1]
	if !strings.Contains(welcome.Subject, "Welcome") {
		t.Errorf("second message %q, want the welcome", welcome.Subject)
	}
}

func TestSignupConfirmWrongCodeRunsOut(t *testing.T) {
	user := unconfirmedUser(t, 7, "ada@example.com", "ada")
	f := newConfirmFlow(t, user)
	f.begin(user)
	code := f.lastCode()

	if rec := f.confirm("12"); !strings.Contains(rec.Body.String(), "6-digit code") {
		t.Fatal("a malformed code was not caught")
	}
	if f.lastToken().CodeAttempts != 0 {
		t.Error("a malformed code cost a try")
	}

	for try := 1; try < constants.OTPMaxAttempts; try++ {
		if rec := f.confirm(wrongCode(code)); !strings.Contains(rec.Body.String(), "not correct") {
			t.Fatalf("try %d: %.160s", try, rec.Body.String())
		}
	}

	wantRedirect(t, f.confirm(wrongCode(code)), "/signup/confirm/resend?error=expired")
	if user.IsEmailVerified() {
		t.Error("confirmed without the code")
	}
}

// A sign-in code must not confirm an address, nor a confirmation code sign in.
func TestSignupConfirmRefusesACodeOfAnotherKind(t *testing.T) {
	user := unconfirmedUser(t, 7, "ada@example.com", "ada")
	f := newConfirmFlow(t, user)

	login := newOAuthHandler(f.te.Env)
	tokenID := login.sendLoginCode("ada@example.com", constants.CodeChannelEmail)
	loginCode := f.lastCode()

	c, _ := get(f.e, "/signup")
	if err := f.h.awaitConfirmation(c, user.Email, tokenID); err != nil {
		t.Fatal(err)
	}

	wantRedirect(t, f.confirm(loginCode), "/signup/confirm/resend?error=expired")
	if user.IsEmailVerified() {
		t.Error("a sign-in code confirmed an address")
	}
}

func TestSignupConfirmWithNothingPendingAsksForTheAddress(t *testing.T) {
	f := newConfirmFlow(t)

	wantRedirect(t, f.get("/signup/confirm?token=an-old-link", f.h.Confirm), "/signup/confirm/resend")
	wantRedirect(t, f.confirm("123456"), "/signup/confirm/resend?error=expired")
}

func TestSignupResendSendsANewCodeAndRetiresTheOld(t *testing.T) {
	user := unconfirmedUser(t, 7, "ada@example.com", "ada")
	f := newConfirmFlow(t, user)
	f.begin(user)
	first := f.lastToken()

	// The button on the code page sends no address.
	wantRedirect(t,
		f.post("/signup/confirm/resend", url.Values{}, f.h.Resend),
		"/signup/confirm")

	second := f.lastToken()
	if second == first || !first.IsUsed {
		t.Fatal("the first code is still live, or no new one was issued")
	}

	if rec := f.confirm(second.Code); !strings.Contains(rec.Body.String(), "Email confirmed") {
		t.Fatalf("%.200s", rec.Body.String())
	}
}

// Asking for a code for an address that needs none, or has no account, leads
// to the same page and sends nothing.
func TestSignupResendSaysNothingAboutTheAddress(t *testing.T) {
	confirmed := signInUser(t, 7, "ada@example.com", "ada")
	f := newConfirmFlow(t, confirmed)

	for _, email := range []string{"ada@example.com", "nobody@example.com"} {
		wantRedirect(t,
			f.post("/signup/confirm/resend", url.Values{"email": {email}}, f.h.Resend),
			"/signup/confirm")

		if len(f.te.Tokens.added) != 0 {
			t.Fatalf("%s: a code was issued", email)
		}

		page := f.get("/signup/confirm", f.h.Confirm)
		if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), email) {
			t.Fatalf("%s: the code page did not render", email)
		}
	}
	f.te.Mail.expectNoneMatching(t, "")

	for try := 1; try < constants.OTPMaxAttempts; try++ {
		if rec := f.confirm("123456"); !strings.Contains(rec.Body.String(), "not correct") {
			t.Fatalf("try %d: %.160s", try, rec.Body.String())
		}
	}
	wantRedirect(t, f.confirm("123456"), "/signup/confirm/resend?error=expired")
}

func TestSignupResendIsLimitedPerAddress(t *testing.T) {
	user := unconfirmedUser(t, 7, "ada@example.com", "ada")
	f := newConfirmFlow(t, user)

	limit := int(rateLimits[rateLimitScopeSignupConfirm].max)
	for range limit + 2 {
		wantRedirect(t,
			f.post("/signup/confirm/resend", url.Values{"email": {"ada@example.com"}}, f.h.Resend),
			"/signup/confirm")
	}

	if len(f.te.Tokens.added) != limit {
		t.Errorf("%d codes issued, want %d", len(f.te.Tokens.added), limit)
	}
}

func TestSignupResendNeedsTheFormToken(t *testing.T) {
	user := unconfirmedUser(t, 7, "ada@example.com", "ada")
	f := newConfirmFlow(t, user)

	rec := f.post("/signup/confirm/resend",
		url.Values{"email": {"ada@example.com"}, "csrf_token": {"forged"}}, f.h.Resend)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "expired") {
		t.Fatalf("status %d", rec.Code)
	}
	if len(f.te.Tokens.added) != 0 {
		t.Error("a forged form sent a code")
	}
}

// A request to join has no account. Its code moves it into the queue.
func TestSignupRequestIsConfirmedByCode(t *testing.T) {
	f := newConfirmFlow(t)

	sr := entities.NewSignupRequest()
	sr.ID = 3
	sr.Email = "grace@example.com"
	sr.Username = "grace"
	sr.Name = "Grace Hopper"
	sr.Status = entities.SignupRequestStatusPending
	f.requests.byEmail[sr.Email] = sr

	wantRedirect(t,
		f.post("/signup/confirm/resend", url.Values{"email": {sr.Email}}, f.h.Resend),
		"/signup/confirm")

	token := f.lastToken()
	if token.CreatedBy != 0 || token.Type != constants.TokenTypeSignupConfirmCode {
		t.Errorf("token for a request: creator %d type %q", token.CreatedBy, token.Type)
	}

	rec := f.confirm(token.Code)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Request received") {
		t.Fatalf("status %d body %.200s", rec.Code, rec.Body.String())
	}
	if !sr.IsEmailVerified() {
		t.Error("the request is still unconfirmed")
	}
}
