package handlers

import "testing"

func TestIsSafeReturnTo(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		// --- Allowed ---
		{"oauth authorize with query", "/oauth/authorize?client_id=x&response_type=code", true},
		{"oauth login", "/oauth/login", true},
		{"oauth nested", "/oauth/authorize/something", true},

		// --- Empty / trivially bad ---
		{"empty", "", false},
		{"just root", "/", false},
		{"relative no slash", "oauth/authorize", false},

		// --- Open-redirect vectors ---
		{"absolute https", "https://evil.com/x", false},
		{"absolute http", "http://evil.com/x", false},
		{"protocol relative", "//evil.com/x", false},
		{"protocol relative triple", "///evil.com/x", false},
		{"javascript scheme", "javascript:alert(1)", false},
		{"data scheme", "data:text/html,foo", false},
		{"file scheme", "file:///etc/passwd", false},
		{"userinfo in absolute", "https://user@evil.com/x", false},

		// --- Backslash and control-char tricks ---
		{"backslash host", "/\\evil.com/x", false},
		{"backslash mixed", "/oauth\\@evil.com", false},
		{"CR injection", "/oauth/authorize\r\nLocation: https://evil.com", false},
		{"LF injection", "/oauth/authorize\nfoo", false},
		{"tab", "/oauth/authorize\t", false},
		{"null byte", "/oauth/authorize\x00", false},
		{"del char", "/oauth/authorize\x7f", false},

		// --- Path-allowlist enforcement ---
		{"unknown idp path", "/admin/console", false},
		{"static file", "/static/images/logo.svg", false},
		{"healthcheck", "/healthz", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isSafeReturnTo(tc.input)
			if got != tc.want {
				t.Errorf("isSafeReturnTo(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"123456", true},
		{"12345", true},
		{"12345a", false},
		{"12 456", false},
		{"-12345", false},
		{"١٢٣٤٥٦", false}, // Arabic-Indic digits, must be rejected
	}
	for _, tc := range cases {
		if got := isAllDigits(tc.in); got != tc.want {
			t.Errorf("isAllDigits(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
