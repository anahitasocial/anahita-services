package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"auth/constants"
	"auth/data/entities"

	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// Tests for the forgot/reset password flow.
//
// This flow is the account-recovery path: it is unauthenticated, it is
// driven entirely by a link in somebody's mailbox, and nobody exercises it
// on a good day. It is also the only place in the service where an
// unauthenticated caller can cause mail to be sent and can learn something
// about whether an address is registered — so the properties worth pinning
// down are as much about what it must NOT reveal or do as about the happy
// path.
//
// The handlers render templates from "web/templates/", relative to the
// process working directory, so these tests chdir to the service root. That
// is not incidental: it means every assertion below also proves the page it
// asserts on actually parses and executes.

func serviceRoot(t *testing.T) {
	t.Helper()
	t.Chdir("../../..")
}

func newTestEcho() *echo.Echo {
	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())
	return e
}

func postForm(e *echo.Echo, path string, form url.Values) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Header.Set("X-Real-IP", "203.0.113.9")
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func get(e *echo.Echo, path string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// testUser builds a user whose stored hash really is a bcrypt hash of
// password, so CheckPassword behaves as it does in production. The handler
// treats "is this the current password?" as a decision point, and a fake
// that always answers false would skip the branch entirely.
func testUser(t *testing.T, id int64, email, password string) *entities.User {
	t.Helper()
	u := entities.NewUser()
	u.ID = id
	u.Email = email
	u.Name = "Ada Lovelace"
	if err := u.SetPassword(password); err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return u
}

func resetToken(userID int64, email string) *entities.Token {
	token := entities.NewToken()
	token.ID = 1
	token.Type = constants.TokenTypePasswordReset
	token.RecipientEmail = email
	token.Body = strings.Repeat("a", 128)
	token.ExpiredAfter = 15 * 60
	token.CreatedAt = time.Now()
	token.CreatedBy = userID
	token.UpdatedAt = time.Now()
	token.UpdatedBy = userID
	token.SetStatus()
	return token
}

// Forgot, and the emailed code, are tested in password_reset_code_test.go.

// ---------------------------------------------------------------------
// ResetForm — the GET the link lands on
// ---------------------------------------------------------------------

func TestResetFormRejectsUnusableTokens(t *testing.T) {
	serviceRoot(t)

	used := resetToken(42, "ada@example.com")
	used.IsUsed = true
	used.SetStatus()

	expired := resetToken(42, "ada@example.com")
	expired.Body = strings.Repeat("b", 128)
	expired.StartTime = time.Now().Add(-time.Hour)
	expired.SetStatus()

	invite := resetToken(42, "ada@example.com")
	invite.Body = strings.Repeat("c", 128)
	invite.Type = constants.TokenTypeInvite

	cases := []struct {
		name  string
		query string
		token *entities.Token
		want  string
	}{
		// No token at all is no longer an error page: it means the code
		// path, which with no code entered goes back to the start. See
		// TestResetWithoutACodeOrALinkGoesBackToTheStart.
		{"unknown token", "?token=" + strings.Repeat("z", 128), nil, "not valid"},
		// A token of the wrong type must not open the password form —
		// otherwise an invite link resets a password.
		{"wrong type", "?token=" + invite.Body, invite, "not valid for resetting"},
		{"already used", "?token=" + used.Body, used, "already been used"},
		{"expired", "?token=" + expired.Body, expired, "has expired"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			te := newTestEnv()
			if tc.token != nil {
				te.Tokens.byBody[tc.token.Body] = tc.token
			}

			c, rec := get(newTestEcho(), "/password/reset"+tc.query)
			if err := newPasswordHandler(te.Env).ResetForm(c); err != nil {
				t.Fatalf("reset form: %v", err)
			}

			body := rec.Body.String()
			if !strings.Contains(body, tc.want) {
				t.Errorf("page does not mention %q", tc.want)
			}
			if strings.Contains(body, `name="password"`) {
				t.Error("the password form was rendered for an unusable token")
			}
		})
	}
}

func TestResetFormRendersTheFormForALiveToken(t *testing.T) {
	serviceRoot(t)

	token := resetToken(42, "ada@example.com")
	te := newTestEnv()
	te.Tokens.byBody[token.Body] = token

	c, rec := get(newTestEcho(), "/password/reset?token="+token.Body)
	if err := newPasswordHandler(te.Env).ResetForm(c); err != nil {
		t.Fatalf("reset form: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `name="password"`) {
		t.Error("no password field on the form")
	}
	// The token has to survive into the POST, or the form cannot be
	// submitted at all.
	if !strings.Contains(body, token.Body) {
		t.Error("the token is not carried in the rendered form")
	}
}

// ---------------------------------------------------------------------
// Reset — the POST that changes the credential
// ---------------------------------------------------------------------

// The whole point of the flow, and everything it is supposed to drag along
// with it: the password changes, the link is spent, and the credentials a
// password change does not touch — passkeys and live sessions — go too.
func TestResetChangesThePasswordAndRevokesWhatSurvivesIt(t *testing.T) {
	serviceRoot(t)

	user := testUser(t, 42, "ada@example.com", "the old password value")
	oldHash := user.Password

	te := newTestEnv(user)
	token := resetToken(42, "ada@example.com")
	te.Tokens.byBody[token.Body] = token

	te.Passkeys.count = 2
	te.AuthLogs.logs = []*entities.AuthLog{{ID: 1, OAuthRefreshTokenID: 7}}
	te.Refresh.byID[7] = &entities.OAuthRefreshToken{
		ID:        7,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	te.Sess.sets[personSessionIndexKey(42)] = []string{"idp-session-1"}

	c, rec := postForm(newTestEcho(), "/password/reset", url.Values{
		"token":    {token.Body},
		"password": {"a brand new password"},
	})
	if err := newPasswordHandler(te.Env).Reset(c); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	if te.Users.editCalls != 1 {
		t.Fatalf("EditPassword called %d times, want 1", te.Users.editCalls)
	}
	if te.Users.editedID != 42 {
		t.Errorf("password written for person %d, want 42", te.Users.editedID)
	}
	if te.Users.editedPassword == oldHash {
		t.Error("the stored hash did not change")
	}
	// A hash, not the plaintext. Cheap to assert and catastrophic to miss.
	if strings.Contains(te.Users.editedPassword, "a brand new password") {
		t.Error("the password was stored in the clear")
	}
	if !strings.HasPrefix(te.Users.editedPassword, "$2") {
		t.Errorf("stored value %q is not a bcrypt hash", te.Users.editedPassword)
	}

	// The link is spent. Without this a reset email stays live for its
	// full window after it has been used.
	if len(te.Tokens.edited) != 1 || !te.Tokens.edited[0].IsUsed {
		t.Error("the token was not consumed")
	}

	if len(te.Passkeys.deletedFor) != 1 || te.Passkeys.deletedFor[0] != 42 {
		t.Errorf("passkeys revoked for %v, want [42]", te.Passkeys.deletedFor)
	}
	if len(te.Refresh.revoked) != 1 || te.Refresh.revoked[0] != 7 {
		t.Errorf("refresh tokens revoked %v, want [7]", te.Refresh.revoked)
	}

	// The IdP cookie survives refresh-token revocation, and /oauth/authorize
	// would honour it and mint a fresh pair without prompting.
	var idpCleared bool
	for _, key := range te.Sess.deleted {
		if key == "idp-session-1" {
			idpCleared = true
		}
	}
	if !idpCleared {
		t.Error("the IdP session was left live")
	}
}

// Reusing the current password would perform the entire recovery ritual —
// revocations, confirmation email — while leaving the credential the reset
// was for exactly where it was.
func TestResetRefusesToSetTheCurrentPassword(t *testing.T) {
	serviceRoot(t)

	const current = "the current password value"
	te := newTestEnv(testUser(t, 42, "ada@example.com", current))
	token := resetToken(42, "ada@example.com")
	te.Tokens.byBody[token.Body] = token

	c, rec := postForm(newTestEcho(), "/password/reset", url.Values{
		"token":    {token.Body},
		"password": {current},
	})
	if err := newPasswordHandler(te.Env).Reset(c); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if !strings.Contains(rec.Body.String(), "different from your current password") {
		t.Error("the page does not say why the submission was refused")
	}
	if te.Users.editCalls != 0 {
		t.Error("the password was written anyway")
	}
	if len(te.Passkeys.deletedFor) != 0 || len(te.Refresh.revoked) != 0 {
		t.Error("credentials were revoked on a refused reset")
	}
	// Deliberately still usable: the person fixes what they typed and
	// resubmits on the same link rather than going back for another email.
	if len(te.Tokens.edited) != 0 {
		t.Error("the token was consumed by a refused attempt")
	}
}

// That same comparison is the only thing on the unauthenticated side that
// can answer "is this the current password?", and a refused attempt leaves
// the link live — so it has to be metered, or whoever holds the token gets
// an unlimited password oracle for the token's lifetime.
func TestResetThrottlesRepeatedCurrentPasswordProbes(t *testing.T) {
	serviceRoot(t)

	const current = "the current password value"
	te := newTestEnv(testUser(t, 42, "ada@example.com", current))
	token := resetToken(42, "ada@example.com")
	te.Tokens.byBody[token.Body] = token

	h := newPasswordHandler(te.Env)
	e := newTestEcho()

	// Budget is 5 per 15 minutes, so the sixth probe is over.
	var last string
	for i := range 6 {
		c, rec := postForm(e, "/password/reset", url.Values{
			"token":    {token.Body},
			"password": {current},
		})
		if err := h.Reset(c); err != nil {
			t.Fatalf("probe %d: %v", i+1, err)
		}
		last = rec.Body.String()
	}

	if !strings.Contains(last, "too many attempts") &&
		!strings.Contains(last, "Too Many Attempts") {
		t.Error("the sixth probe was not refused")
	}
}

// An ordinary form error is not a probe, and must not spend from the same
// budget — otherwise somebody mistyping a short password five times loses
// the link they were sent.
func TestResetDoesNotSpendTheProbeBudgetOnFormErrors(t *testing.T) {
	serviceRoot(t)

	te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))
	token := resetToken(42, "ada@example.com")
	te.Tokens.byBody[token.Body] = token

	h := newPasswordHandler(te.Env)
	e := newTestEcho()

	for range 6 {
		c, rec := postForm(e, "/password/reset", url.Values{
			"token":    {token.Body},
			"password": {"short"},
		})
		if err := h.Reset(c); err != nil {
			t.Fatalf("reset: %v", err)
		}
		if !strings.Contains(rec.Body.String(), "15 characters") {
			t.Fatal("a too-short password was not reported as such")
		}
	}

	// Still able to complete afterwards.
	c, rec := postForm(e, "/password/reset", url.Values{
		"token":    {token.Body},
		"password": {"a perfectly good new password"},
	})
	if err := h.Reset(c); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if te.Users.editCalls != 1 {
		t.Errorf("EditPassword called %d times after six form errors, want 1",
			te.Users.editCalls)
	}
	if strings.Contains(rec.Body.String(), "Too Many Attempts") {
		t.Error("form errors consumed the probe budget")
	}
}

func TestResetRefusesASpentToken(t *testing.T) {
	serviceRoot(t)

	te := newTestEnv(testUser(t, 42, "ada@example.com", "the old password value"))
	token := resetToken(42, "ada@example.com")
	token.IsUsed = true
	token.SetStatus()
	te.Tokens.byBody[token.Body] = token

	c, rec := postForm(newTestEcho(), "/password/reset", url.Values{
		"token":    {token.Body},
		"password": {"a brand new password"},
	})
	if err := newPasswordHandler(te.Env).Reset(c); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if !strings.Contains(rec.Body.String(), "No Longer Valid") {
		t.Error("a used token was accepted")
	}
	if te.Users.editCalls != 0 {
		t.Error("the password was changed using a spent token")
	}
}

// The account is never signed in as part of a reset. The login path is
// where TOTP and the disabled check live, so completing a reset with a
// session would walk straight past both.
func TestResetDoesNotSignThePersonIn(t *testing.T) {
	serviceRoot(t)

	te := newTestEnv(testUser(t, 42, "ada@example.com", "the old password value"))
	token := resetToken(42, "ada@example.com")
	te.Tokens.byBody[token.Body] = token

	c, rec := postForm(newTestEcho(), "/password/reset", url.Values{
		"token":    {token.Body},
		"password": {"a brand new password"},
	})
	if err := newPasswordHandler(te.Env).Reset(c); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if cookies := rec.Result().Cookies(); len(cookies) > 0 {
		t.Errorf("the reset set %d cookie(s); it must not establish a session", len(cookies))
	}
}

// The reset form validates two fields and renders one message, so a truncated
// or mangled link reported "Password must be at least 15 characters long" —
// sending somebody back to correct the only part of the submission that was
// already right.
//
// The two failures must reach different pages, and a bad link must reach the
// one that offers a fresh link rather than a form whose hidden field is the
// fault.
func TestResetTellsALinkProblemFromAPasswordProblem(t *testing.T) {
	serviceRoot(t)
	e := newTestEcho()

	longEnough := strings.Repeat("a", passwordResetTokenLength)

	cases := []struct {
		name        string
		token       string
		password    string
		wantContain string
		wantAbsent  string
	}{
		{
			name:        "short password, good-shaped token",
			token:       longEnough,
			password:    "short",
			wantContain: "Password must be at least",
			wantAbsent:  "request a new one",
		},
		{
			name:        "truncated token, good password",
			token:       "too-short",
			password:    "a perfectly acceptable password",
			wantContain: "not valid",
			wantAbsent:  "Password must be at least",
		},
		// An empty token is the code path, and is covered in
		// password_reset_code_test.go.
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv()
			c, rec := postForm(e, "/password/reset", url.Values{
				"token":    {tc.token},
				"password": {tc.password},
			})

			if err := newPasswordHandler(env.Env).Reset(c); err != nil {
				t.Fatalf("Reset: %v", err)
			}

			body := rec.Body.String()
			if !strings.Contains(body, tc.wantContain) {
				t.Errorf("page does not mention %q", tc.wantContain)
			}
			if strings.Contains(body, tc.wantAbsent) {
				t.Errorf("page mentions %q, which is the other failure",
					tc.wantAbsent)
			}
		})
	}
}
