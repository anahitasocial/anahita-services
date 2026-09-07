package validators

import "strings"

// NormalizeWebsiteURL prepares a person-supplied website for validation.
//
// People type "example.com". The `http_url` rule requires a scheme — that
// requirement is the point, because the value is rendered as an anchor href
// and a javascript: or data: value would be stored XSS — so a bare host has to
// gain one before it is checked, or the field rejects the most natural thing
// anyone enters.
//
// https rather than http: every host worth linking to serves TLS, and
// defaulting to http would downgrade a link the person meant to be secure.
//
// Deliberately does NOT rescue a bad scheme. "javascript:alert(1)" already has
// one and is left exactly as it is, so validation sees it and refuses. The
// alternative — stripping what is there and prepending https — would turn an
// attack into a working link to a host named "alert(1)".
func NormalizeWebsiteURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	// Anything that already declares a scheme is left alone. Only http and
	// https survive validation, so the rest fail there rather than here.
	if strings.Contains(trimmed, "://") {
		return trimmed
	}

	// A colon with no slashes is still a scheme — "javascript:alert(1)" and
	// "mailto:someone" both land here. The check excludes a colon that is
	// really a port ("example.com:8080") by requiring the part before it to
	// look like a scheme rather than a host.
	if i := strings.Index(trimmed, ":"); i > 0 && !strings.ContainsAny(trimmed[:i], "./") {
		return trimmed
	}

	return "https://" + trimmed
}
