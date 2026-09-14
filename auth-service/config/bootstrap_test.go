package config

import "testing"

// Where the terms live is a deployment decision, and the two shapes it takes
// are genuinely different: a path, when the documents are pages of the web
// app, or a whole URL, when the canonical text is kept somewhere else. Getting
// the second wrong produces a link like
// "https://example.org/https://legal.example.org/terms", which renders fine
// and goes nowhere.

func agreementConfig(frontend, tos, privacy string) *Config {
	return &Config{
		FrontendURL: frontend,
		TosURL:      tos,
		PrivacyURL:  privacy,
	}
}

func TestAgreementURLs(t *testing.T) {
	const frontend = "https://example.org"

	for _, tc := range []struct {
		name       string
		configured string
		want       string
		because    string
	}{
		{
			name: "unset", configured: "",
			want:    frontend + "/legal/tos",
			because: "an installation that sets nothing still needs a working link",
		},
		{
			name: "a path", configured: "/legal/terms",
			want:    frontend + "/legal/terms",
			because: "renaming the route must not need a code change",
		},
		{
			// "legal/terms" and "/legal/terms" are the same intention, and
			// only one of them concatenates into a working URL.
			name: "a path without its leading slash", configured: "legal/terms",
			want:    frontend + "/legal/terms",
			because: "the missing slash is a typo, not a different address",
		},
		{
			name: "an absolute https URL", configured: "https://legal.example.org/terms",
			want:    "https://legal.example.org/terms",
			because: "documents hosted elsewhere must not be prefixed with our own host",
		},
		{
			name: "an absolute http URL", configured: "http://legal.example.org/terms",
			want:    "http://legal.example.org/terms",
			because: "http is still absolute, however unwise",
		},
		{
			name: "surrounding whitespace", configured: "  /legal/terms  ",
			want:    frontend + "/legal/terms",
			because: "a stray space in an env file should not break the link",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := agreementConfig(frontend, tc.configured, tc.configured)

			if got := config.GetTosURL(); got != tc.want {
				t.Errorf("GetTosURL() = %q, want %q — %s", got, tc.want, tc.because)
			}
		})
	}
}

// The privacy policy takes its own setting and its own default. They were one
// helper, and a helper shared by two settings is one rename away from both
// pointing at the same document.
func TestTheTwoAgreementsAreConfiguredSeparately(t *testing.T) {
	config := agreementConfig("https://example.org", "", "")

	if config.GetTosURL() == config.GetPrivacyURL() {
		t.Fatal("both agreements default to the same URL")
	}

	config.PrivacyURL = "https://legal.example.org/privacy"

	if config.GetTosURL() != "https://example.org/legal/tos" {
		t.Errorf("setting the privacy URL moved the terms URL to %q", config.GetTosURL())
	}
	if config.GetPrivacyURL() != "https://legal.example.org/privacy" {
		t.Errorf("GetPrivacyURL() = %q", config.GetPrivacyURL())
	}
}

// A trailing slash on FRONTEND_URL is the operator writing a URL the way
// people write URLs. It must not double up.
func TestAgreementURLSurvivesATrailingSlashOnTheFrontend(t *testing.T) {
	config := agreementConfig("https://example.org/", "/legal/terms", "")

	if got := config.GetTosURL(); got != "https://example.org/legal/terms" {
		t.Errorf("GetTosURL() = %q; the slashes doubled up", got)
	}
}
