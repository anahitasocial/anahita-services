package handlers

import (
	"html/template"
	"io"
	"os"
	"strings"
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
		{"signup_confirm_code.html", map[string]any{"CSRFToken": "t", "Email": "ada@example.com"}},
		{"signup_confirm_code.html", map[string]any{
			"CSRFToken": "t",
			"Email":     "ada@example.com",
			"Error":     "That code is not correct.",
		}},
		{"signup_confirm_resend.html", map[string]any{"CSRFToken": "t"}},
		{"signup_confirm_success.html", map[string]any{"Email": "ada@example.com"}},
		{"signup_request_sent.html", map[string]any{"Email": "ada@example.com"}},
		{"login_code.html", map[string]any{"CSRFToken": "t", "FrontendURL": "https://example.com"}},
		{"login_code.html", map[string]any{
			"CSRFToken": "t",
			"Error":     "That code is no longer valid. Please request a new one.",
			"Hint":      "Enter the email address or username for your account.",
		}},
		{"login_code_method.html", map[string]any{
			"CSRFToken": "t",
			"Channels":  []string{"email", "sms"},
		}},
		{"login_code_verify.html", map[string]any{
			"CSRFToken":  "t",
			"Identifier": "ada@example.com",
		}},
		{"login_code_verify.html", map[string]any{
			"CSRFToken":  "t",
			"Identifier": "ada",
			"Error":      "That code is not correct. 2 tries left.",
		}},
		// The forgot/reset pages. These are the account-recovery path: nobody
		// walks it on a good day, and a page that fails to execute returns a
		// 500 carrying a Go template error to somebody who is already locked
		// out.
		{"password_forgot.html", map[string]any{}},
		{"password_forgot.html", map[string]any{
			"Error": "Please enter a valid email address.",
			"Email": "ada@example.com",
		}},
		{"password_reset_code.html", map[string]any{"CSRFToken": "t", "Email": "ada@example.com"}},
		{"password_reset_code.html", map[string]any{
			"CSRFToken": "t",
			"Email":     "ada@example.com",
			"Error":     "That code is not correct. 1 try left.",
		}},
		{"password_reset.html", map[string]any{"Token": "abc"}},
		{"password_reset.html", map[string]any{
			"Token": "abc",
			"Error": "Your new password must be different from your current password.",
		}},
		{"password_reset_success.html", map[string]any{}},
		// error.html is the terminal page for every unusable link in the
		// service, so it is reached from more flows than any other.
		{"error.html", map[string]any{
			"ErrorTitle":   "Link Expired",
			"ErrorMessage": "This password reset link has expired.",
			"FrontendURL":  "https://example.com",
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

// Every remaining page, parsed but not executed.
//
// Executing needs data this test would have to invent per page; parsing
// needs nothing and still catches the failure that actually happens — an
// unclosed action or a misspelled partial name, introduced while editing
// markup, in a page nobody renders until somebody is mid-flow.
func TestEveryServerRenderedTemplateParses(t *testing.T) {
	const dir = "../../../web/templates/"

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read templates: %v", err)
	}

	var pages int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		// base and logo are the set every page is parsed with, never on
		// their own.
		if name == "base.html" || name == "logo.html" {
			continue
		}

		pages++
		t.Run(name, func(t *testing.T) {
			if _, err := template.ParseFiles(
				dir+"base.html",
				dir+"logo.html",
				dir+name,
			); err != nil {
				t.Fatalf("parse: %v", err)
			}
		})
	}

	if pages == 0 {
		t.Fatal("no templates found; the path is wrong and this test proves nothing")
	}
}
