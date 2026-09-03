package handlers

import (
	"html/template"
	"io"
	"testing"
)

// The renderTemplate path parses base.html + logo.html + the page on
// every request and returns a 500 carrying the parse error if any of
// them is malformed. These pages are only ever reached from a link in
// somebody's mailbox, so a broken one fails where nobody is watching.
func TestEmailChangeTemplatesParseAndExecute(t *testing.T) {
	cases := []struct {
		name string
		data map[string]any
	}{
		{"email_change_confirm.html", map[string]any{
			"Token":    "abc",
			"NewEmail": "new@example.com",
			"Error":    "",
		}},
		{"email_change_confirm.html", map[string]any{
			"Token":    "abc",
			"NewEmail": "new@example.com",
			"Error":    "That does not match.",
		}},
		{"email_change_success.html", map[string]any{
			"NewEmail":    "new@example.com",
			"FrontendURL": "https://example.com",
		}},
	}

	for _, tc := range cases {
		tmpl, err := template.ParseFiles(
			"../../../web/templates/base.html",
			"../../../web/templates/logo.html",
			"../../../web/templates/"+tc.name,
		)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		if err := tmpl.ExecuteTemplate(io.Discard, "base", tc.data); err != nil {
			t.Fatalf("%s: execute: %v", tc.name, err)
		}
	}
}
