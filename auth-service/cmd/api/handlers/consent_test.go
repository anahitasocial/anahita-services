package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"auth/data/entities"

	"github.com/labstack/echo/v4"
)

func consentContext(headers map[string]string) echo.Context {
	req := httptest.NewRequest(http.MethodPatch, "/agreements/tos", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return echo.New().NewContext(req, httptest.NewRecorder())
}

// The version comes from the web app and is checked for SHAPE only.
//
// It is compared numerically, part by part, on the other side — so a value
// like "v2" or "1.0-beta" would compare as something nobody intended, and the
// check that decides who is asked to accept again would quietly misfire.
func TestAgreementVersionShape(t *testing.T) {
	for _, tc := range []struct {
		version string
		ok      bool
	}{
		{"1", true},
		{"1.0", true},
		{"1.0.0", true},
		{"12.4.3", true},
		{"", false},
		{"v1.0", false},
		{"1.0-beta", false},
		{"1.0.0.0", false},
		{"current", false},
		{" 1.0", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			consent := &entities.Consent{}
			err := acceptInto(consentContext(nil), consent, tc.version)

			if tc.ok && err != nil {
				t.Errorf("%q was refused: %v", tc.version, err)
			}
			if !tc.ok && !errors.Is(err, ErrAgreementVersionMalformed) {
				t.Errorf("%q was accepted as a version", tc.version)
			}
			if !tc.ok && consent.Accepted {
				t.Errorf("a refused version still recorded an acceptance")
			}
		})
	}
}

// The client names the version. The SERVER names when, and from where — those
// are what make the row evidence rather than a claim, and none of them comes
// from anything the client submitted.
func TestAcceptanceCircumstancesComeFromTheRequest(t *testing.T) {
	consent := &entities.Consent{}

	c := consentContext(map[string]string{
		"User-Agent":      "Anahita/2.1 CFNetwork/1494",
		"X-Client-OS":     "iOS 17.4",
		"X-Forwarded-For": "198.51.100.7, 10.0.0.1",
	})

	if err := acceptInto(c, consent, "1.0.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !consent.Accepted || consent.Version != "1.0.0" || consent.AcceptedAt == nil {
		t.Errorf("the acceptance was not recorded: %+v", consent)
	}
	if consent.OS != "iOS 17.4" {
		t.Errorf("OS = %q, want the explicit header", consent.OS)
	}
	if consent.IP != "198.51.100.7" {
		t.Errorf("IP = %q, want the first entry in X-Forwarded-For", consent.IP)
	}
}

// The columns are varchar(255) and varchar(100). MySQL under a strict sql_mode
// rejects an over-long value rather than trimming it, so an unusual browser
// would otherwise be unable to accept the terms at all.
func TestConsentValuesAreCappedAtTheColumnWidth(t *testing.T) {
	consent := &entities.Consent{}

	c := consentContext(map[string]string{
		"User-Agent":  strings.Repeat("x", 400),
		"X-Client-OS": strings.Repeat("y", 300),
	})

	if err := acceptInto(c, consent, "1.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := len(consent.UserAgent); got != 255 {
		t.Errorf("user agent is %d characters; the column holds 255", got)
	}
	if got := len(consent.OS); got != 100 {
		t.Errorf("os is %d characters; the column holds 100", got)
	}
}
