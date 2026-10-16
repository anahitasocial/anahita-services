package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"auth/cmd/api/permissions"
	"auth/data/entities"

	libconstants "anahita-libs/constants"
	"anahita-libs/crypt"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"

	"anahita-libs/validators"
)

// Enrolling in 2FA is three calls: POST /totp makes a secret, POST
// /totp/recovery-codes checks a passcode from it and hands out the codes, and
// PATCH /totp switches it on. Only the middle one proves the authenticator
// works, so the other two must not be able to stand in for it.

const testCryptKey = "0123456789abcdef0123456789abcdef"

func totpRequest(method, path string, form url.Values, viewerID int64) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())

	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(libconstants.ViewerContextKey, testViewer(viewerID, 0))
	return c, rec
}

// totpEnv is a test Env with the TOTP permissions the handler consults.
func totpEnv(users ...*entities.User) *testEnv {
	te := newTestEnv(users...)
	te.Env.Perms = &permissions.Permissions{Totp: permissions.NewTotpPerm(&permissions.Env{})}
	return te
}

func totpAccount(t *testing.T) *entities.User {
	t.Helper()
	user := entities.NewUser()
	user.ID = 7
	user.Email = "ana@example.com"
	user.Enabled = true
	if err := user.SetPassword("correct horse battery staple"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	return user
}

func TestTotpCannotBeEnabledBeforeEnrolmentIsFinished(t *testing.T) {
	cases := []struct {
		what   string
		change func(*entities.User)
	}{
		{"with no secret at all", func(u *entities.User) {}},
		{"with a secret but no passcode ever checked", func(u *entities.User) { u.TotpSecret = "encrypted-secret" }},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			user := totpAccount(t)
			c.change(user)
			te := totpEnv(user)

			ctx, rec := totpRequest(http.MethodPatch, "/totp", nil, user.ID)
			if err := newTotpHandler(te.Env).Edit(ctx); err != nil {
				t.Fatalf("handler: %v", err)
			}

			if rec.Code != http.StatusConflict {
				t.Errorf("status = %d, want 409", rec.Code)
			}
			if te.Users.byID[user.ID].TotpEnabled {
				t.Error("2FA was switched on for an account that cannot produce a passcode")
			}
		})
	}
}

// Enabling checks no passcode, so it must not record that one was verified.
// Anything gated on that freshness would otherwise be gated on nothing.
func TestTotpEnableDoesNotStampAVerification(t *testing.T) {
	user := totpAccount(t)
	user.TotpSecret = "encrypted-secret"
	if err := user.SetRecoveryCodes([]string{"aaaaaaaa", "bbbbbbbb"}); err != nil {
		t.Fatalf("recovery codes: %v", err)
	}
	stamped := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	user.TotpLastVerifiedAt = stamped
	te := totpEnv(user)

	ctx, rec := totpRequest(http.MethodPatch, "/totp", nil, user.ID)
	if err := newTotpHandler(te.Env).Edit(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := te.Users.byID[user.ID]
	if !got.TotpEnabled {
		t.Error("2FA was not switched on")
	}
	if !got.TotpLastVerifiedAt.Equal(stamped) {
		t.Errorf("TotpLastVerifiedAt moved to %v; enabling verified nothing", got.TotpLastVerifiedAt)
	}
}

// POST /totp replaces the secret and clears TotpEnabled. With 2FA on, that
// let a password alone switch the second factor off.
func TestTotpEnrolmentCannotBeRestartedWhile2FAIsOn(t *testing.T) {
	user := totpAccount(t)
	user.TotpEnabled = true
	user.TotpSecret = "encrypted-secret"
	te := totpEnv(user)

	ctx, rec := totpRequest(http.MethodPost, "/totp", url.Values{"password": {"correct horse battery staple"}}, user.ID)
	if err := newTotpHandler(te.Env).Add(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	got := te.Users.byID[user.ID]
	if !got.TotpEnabled || got.TotpSecret != "encrypted-secret" {
		t.Error("the working second factor was changed")
	}
}

// The passcode is checked when the recovery codes are issued, so that is where
// the verification is recorded.
func TestTotpRecoveryCodesStampTheVerification(t *testing.T) {
	secretCrypt, err := crypt.NewLocalCrypt(testCryptKey)
	if err != nil {
		t.Fatalf("crypt: %v", err)
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "test", AccountName: "ana@example.com"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	user := totpAccount(t)
	if err := user.SetTotpSecret(secretCrypt, key.Secret()); err != nil {
		t.Fatalf("set secret: %v", err)
	}
	te := totpEnv(user)
	te.Env.SecretCrypt = secretCrypt

	t.Run("a wrong passcode issues nothing and stamps nothing", func(t *testing.T) {
		ctx, rec := totpRequest(http.MethodPost, "/totp/recovery-codes", url.Values{"passcode": {"000000"}}, user.ID)
		if err := newTotpHandler(te.Env).AddRecoveryCodes(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
		got := te.Users.byID[user.ID]
		if got.CountRecoveryCodes() != 0 || !got.TotpLastVerifiedAt.IsZero() {
			t.Error("a wrong passcode issued codes or recorded a verification")
		}
	})

	t.Run("the right passcode issues the codes and stamps the time", func(t *testing.T) {
		passcode, err := totp.GenerateCode(key.Secret(), time.Now())
		if err != nil {
			t.Fatalf("passcode: %v", err)
		}
		before := time.Now().Add(-time.Second)

		ctx, rec := totpRequest(http.MethodPost, "/totp/recovery-codes", url.Values{"passcode": {passcode}}, user.ID)
		if err := newTotpHandler(te.Env).AddRecoveryCodes(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		got := te.Users.byID[user.ID]
		if got.CountRecoveryCodes() == 0 {
			t.Error("no recovery codes were stored")
		}
		if got.TotpLastVerifiedAt.Before(before) {
			t.Errorf("TotpLastVerifiedAt = %v, want it set to now", got.TotpLastVerifiedAt)
		}
	})
}
