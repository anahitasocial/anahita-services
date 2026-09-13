package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"auth/config"
	"auth/data/entities"

	"github.com/labstack/echo/v4"
)

func consentEnv(tos, privacy string) *Env {
	return &Env{Config: &config.Config{TosVersion: tos, PrivacyVersion: privacy}}
}

func consentContext(headers map[string]string) echo.Context {
	req := httptest.NewRequest(http.MethodPost, "/signups", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return echo.New().NewContext(req, httptest.NewRecorder())
}

// The version check is the point of recording a version at all.
//
// Without it, somebody who opened the signup page before a revision and
// submitted after it is recorded as having accepted the new text — and the
// row would say so, which is worse than having no row. They are refused and
// asked again instead.
func TestRecordAgreementsRefusesAStaleVersion(t *testing.T) {
	env := consentEnv("2.0", "2.0")

	for _, tc := range []struct {
		name         string
		tos, privacy string
		wantErr      bool
	}{
		{"both current", "2.0", "2.0", false},
		{"terms superseded", "1.0", "2.0", true},
		{"privacy superseded", "2.0", "1.0", true},
		{"neither submitted", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := entities.NewUser()

			err := env.recordAgreements(consentContext(nil), user, tc.tos, tc.privacy)

			if tc.wantErr {
				if !errors.Is(err, ErrAgreementVersionStale) {
					t.Fatalf("got %v, want ErrAgreementVersionStale", err)
				}
				if user.TermsOfService.Accepted || user.PrivacyPolicy.Accepted {
					t.Error("a refused submission still recorded an acceptance")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !user.TermsOfService.Accepted || !user.PrivacyPolicy.Accepted {
				t.Error("a valid submission recorded nothing")
			}
		})
	}
}

// The version STORED is the server's, never the client's. They are equal by
// the time this runs, but taking it from the request would mean a consent row
// whose version came from the thing being consented to.
func TestRecordAgreementsStoresTheServersVersion(t *testing.T) {
	env := consentEnv("3.1", "3.1")
	user := entities.NewUser()

	if err := env.recordAgreements(consentContext(nil), user, "3.1", "3.1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.TermsOfService.Version != "3.1" || user.PrivacyPolicy.Version != "3.1" {
		t.Errorf("recorded %q/%q, want 3.1 for both",
			user.TermsOfService.Version, user.PrivacyPolicy.Version)
	}
}

// X-Client-OS wins over the parsed User-Agent, matching auth_log.go — the
// native apps send it explicitly because their UA strings parse to nothing,
// and a person's consent row should describe the same device their sign-in
// history does.
func TestConsentPrefersTheExplicitClientOS(t *testing.T) {
	env := consentEnv("1.0", "1.0")
	user := entities.NewUser()

	c := consentContext(map[string]string{
		"User-Agent":      "Anahita/2.1 CFNetwork/1494",
		"X-Client-OS":     "iOS 17.4",
		"X-Forwarded-For": "198.51.100.7, 10.0.0.1",
	})

	if err := env.recordAgreements(c, user, "1.0", "1.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.TermsOfService.OS != "iOS 17.4" {
		t.Errorf("OS = %q, want the explicit header", user.TermsOfService.OS)
	}
	if user.TermsOfService.IP != "198.51.100.7" {
		t.Errorf("IP = %q, want the first entry in X-Forwarded-For",
			user.TermsOfService.IP)
	}
}

// The columns are varchar(255) and varchar(100). MySQL under a strict
// sql_mode rejects an over-long value rather than trimming it, so an unusual
// browser would otherwise be told its account could not be created.
func TestConsentValuesAreCappedAtTheColumnWidth(t *testing.T) {
	env := consentEnv("1.0", "1.0")
	user := entities.NewUser()

	c := consentContext(map[string]string{
		"User-Agent":  strings.Repeat("x", 400),
		"X-Client-OS": strings.Repeat("y", 300),
	})

	if err := env.recordAgreements(c, user, "1.0", "1.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := len(user.TermsOfService.UserAgent); got != 255 {
		t.Errorf("user agent is %d characters; the column holds 255", got)
	}
	if got := len(user.TermsOfService.OS); got != 100 {
		t.Errorf("os is %d characters; the column holds 100", got)
	}
}
