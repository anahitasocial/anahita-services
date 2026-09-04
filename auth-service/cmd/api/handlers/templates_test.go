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
func TestServerRenderedTemplatesParseAndExecute(t *testing.T) {
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
		// The restore pages are reached only by somebody who deleted their
		// account and changed their mind — a path nobody walks by accident, so
		// a broken template there would sit undiscovered until it mattered
		// most.
		{"account_restore.html", map[string]any{
			"PurgeDate": "3 October 2026",
		}},
		// PurgeDate empty: the template must decline to name a date rather
		// than print one derived from a zero time.
		{"account_restore.html", map[string]any{
			"PurgeDate": "",
		}},
		{"account_restore_success.html", map[string]any{}},
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
