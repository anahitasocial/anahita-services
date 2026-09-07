package validators

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

// The website is rendered as an anchor href on a public profile, so the value
// that reaches storage has to be one a browser will treat as a link and
// nothing else. Normalisation and validation are a pair: normalisation exists
// so a bare host passes, and it must not accidentally rescue a scheme that
// validation is there to reject.

type websiteProbe struct {
	URL string `validate:"omitempty,http_url,max=255"`
}

func accepted(t *testing.T, raw string) bool {
	t.Helper()
	return validator.New().Struct(websiteProbe{URL: NormalizeWebsiteURL(raw)}) == nil
}

func TestNormalizeAddsASchemeToABareHost(t *testing.T) {
	cases := map[string]string{
		"example.com":           "https://example.com",
		"www.example.com/about": "https://www.example.com/about",
		"  example.com  ":       "https://example.com",
		"example.com:8080":      "https://example.com:8080",
		"https://example.com":   "https://example.com",
		"http://localhost:3000": "http://localhost:3000",
		"":                      "",
		"   ":                   "",
	}

	for raw, want := range cases {
		if got := NormalizeWebsiteURL(raw); got != want {
			t.Errorf("NormalizeWebsiteURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

// The important half. A value that already declares a scheme is passed through
// untouched, so validation sees it and refuses — rather than being rewritten
// into a working link to a host called "alert(1)".
func TestNormalizeDoesNotRescueADangerousScheme(t *testing.T) {
	for _, raw := range []string{
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"file:///etc/passwd",
		"vbscript:msgbox",
	} {
		t.Run(raw, func(t *testing.T) {
			if got := NormalizeWebsiteURL(raw); got != raw {
				t.Errorf("rewrote %q to %q instead of leaving it to fail validation", raw, got)
			}
			if accepted(t, raw) {
				t.Errorf("%q survived validation — this renders as a live href", raw)
			}
		})
	}
}

func TestValidationAcceptsRealWebsites(t *testing.T) {
	for _, raw := range []string{
		"example.com",
		"https://example.com",
		"http://example.com",
		"https://sub.example.co.uk/path?ref=profile#frag",
		"https://example.com:8443/",
	} {
		t.Run(raw, func(t *testing.T) {
			if !accepted(t, raw) {
				t.Errorf("%q was rejected; it normalises to %q", raw, NormalizeWebsiteURL(raw))
			}
		})
	}
}

func TestValidationRejectsNonHTTPSchemes(t *testing.T) {
	for _, raw := range []string{
		"ftp://example.com",
		"mailto:someone@example.com",
		"javascript:alert(1)",
	} {
		t.Run(raw, func(t *testing.T) {
			if accepted(t, raw) {
				t.Errorf("%q was accepted as a website", raw)
			}
		})
	}
}

// Empty is a supported state — the field is optional and clearing it is how
// somebody removes their link.
func TestAnEmptyWebsiteIsValid(t *testing.T) {
	if !accepted(t, "") {
		t.Error("an empty website was rejected, so the field could never be cleared")
	}
}
