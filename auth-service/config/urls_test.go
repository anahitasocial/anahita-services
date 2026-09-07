package config

import "testing"

// Both public base URLs fall back to localhost outside prod and staging —
// correct on a laptop, wrong on any SHARED development deployment, where every
// reset link and notification action URL then points at whichever machine
// opened the mail rather than at the server that sent it.
//
// They travel together: one reset mail carries a link from each, so an
// override that fixed only one would leave the message half wrong.

func TestPublicURLsFallBackWhenUnset(t *testing.T) {
	cfg := &Config{Env: "dev", DomainName: "example.com"}

	if got := cfg.GetAuthURL(); got != "http://localhost" {
		t.Errorf("GetAuthURL() = %q; the derived default must not change for "+
			"local work", got)
	}
	if got := cfg.GetFrontendURL(); got != "http://localhost:3000" {
		t.Errorf("GetFrontendURL() = %q", got)
	}
}

func TestPublicURLsTakeAnExplicitOverride(t *testing.T) {
	cfg := &Config{
		Env:         "dev",
		DomainName:  "example.com",
		AuthURL:     "https://auth.dev.example.com",
		FrontendURL: "https://dev.example.com",
	}

	if got := cfg.GetAuthURL(); got != "https://auth.dev.example.com" {
		t.Errorf("GetAuthURL() = %q, want the override", got)
	}
	if got := cfg.GetFrontendURL(); got != "https://dev.example.com" {
		t.Errorf("GetFrontendURL() = %q, want the override", got)
	}
}

// A trailing slash is the obvious way to get this wrong in a .env file, and it
// produces "https://host//password/reset?token=..." — which some servers
// answer and some do not.
func TestPublicURLsTrimATrailingSlash(t *testing.T) {
	cfg := &Config{
		Env:         "dev",
		AuthURL:     "https://auth.dev.example.com/",
		FrontendURL: "https://dev.example.com/",
	}

	if got := cfg.GetAuthURL(); got != "https://auth.dev.example.com" {
		t.Errorf("GetAuthURL() = %q; a trailing slash doubles up in the link", got)
	}
	if got := cfg.GetFrontendURL(); got != "https://dev.example.com" {
		t.Errorf("GetFrontendURL() = %q", got)
	}
}

// Production must not be overridable by a stray environment variable in a way
// that silently redirects password reset links somewhere else — but it IS, and
// deliberately: an installation serving auth from a different host than
// auth.<domain> has no other way to say so. Pinned so the choice is visible.
func TestOverrideAppliesInProductionToo(t *testing.T) {
	cfg := &Config{Env: "prod", DomainName: "example.com"}
	if got := cfg.GetAuthURL(); got != "https://auth.example.com" {
		t.Fatalf("derived prod URL = %q", got)
	}

	cfg.AuthURL = "https://id.example.com"
	if got := cfg.GetAuthURL(); got != "https://id.example.com" {
		t.Errorf("GetAuthURL() = %q; the override is meant to win everywhere, "+
			"so an installation can serve auth from its own host", got)
	}
}
