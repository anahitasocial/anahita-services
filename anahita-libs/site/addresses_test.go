package site

import (
	"errors"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestAddressesAreWorkedOutAsAuthServiceWorksOutItsOwn(t *testing.T) {
	cases := map[string]struct {
		env              map[string]string
		domain, web, api string
	}{
		"a live site": {
			map[string]string{"ENV": "prod", "DOMAIN_NAME": "example.org"},
			"example.org", "https://example.org", "https://api.example.org",
		},
		"a staging site": {
			map[string]string{"ENV": "staging", "DOMAIN_NAME": "example.org"},
			"staging.example.org", "https://staging.example.org", "https://api.staging.example.org",
		},
		"development": {
			map[string]string{"ENV": "dev", "DOMAIN_NAME": "example.org"},
			"localhost", "http://localhost:3000", "http://localhost",
		},
		"nothing set": {
			map[string]string{},
			"localhost", "http://localhost:3000", "http://localhost",
		},
		"each said outright": {
			map[string]string{
				"ENV": "prod", "DOMAIN_NAME": "example.org",
				"PUBLIC_DOMAIN": "Social.Example.org", "PUBLIC_WEB_URL": "https://social.example.org/", "PUBLIC_API_URL": " https://social-api.example.org/ ",
			},
			"social.example.org", "https://social.example.org", "https://social-api.example.org",
		},
		"the web app moved, and the domain follows it": {
			map[string]string{"ENV": "prod", "DOMAIN_NAME": "example.org", "PUBLIC_WEB_URL": "https://www.example.net"},
			"example.net", "https://www.example.net", "https://api.example.org",
		},
		"the web app as auth-service was told": {
			map[string]string{"ENV": "prod", "DOMAIN_NAME": "example.org", "FRONTEND_URL": "https://app.example.org/"},
			"app.example.org", "https://app.example.org", "https://api.example.org",
		},
		"a tunnel to a developer's machine": {
			map[string]string{"ENV": "dev", "PUBLIC_DOMAIN": "fedi-dev.example.org", "PUBLIC_WEB_URL": "https://fedi-dev.example.org", "PUBLIC_API_URL": "https://api.fedi-dev.example.org"},
			"fedi-dev.example.org", "https://fedi-dev.example.org", "https://api.fedi-dev.example.org",
		},
	}

	for name, c := range cases {
		got := from(env(c.env))
		if got.Domain != c.domain || got.WebURL != c.web || got.APIURL != c.api {
			t.Errorf("%s: got %+v", name, got)
		}
	}
}

func TestWhatIsMadeOfThem(t *testing.T) {
	a := Addresses{Domain: "example.org", WebURL: "https://example.org", APIURL: "https://api.example.org"}

	if got := a.Handle("ana"); got != "ana@example.org" {
		t.Errorf("handle %q", got)
	}
	if got := a.Web("/people/ana"); got != "https://example.org/people/ana" {
		t.Errorf("web %q", got)
	}
	if got := a.API("/ap/users/1"); got != "https://api.example.org/ap/users/1" {
		t.Errorf("api %q", got)
	}

	for address, want := range map[string]bool{
		"https://api.example.org/ap/users/1":     true,
		"https://API.example.org/ap/users/1":     true,
		"https://example.org/people/ana":         true,
		"https://example.org.evil.test/x":        false,
		"https://evil.test/?https://example.org": false,
		"https://mastodon.example/users/bob":     false,
		"/people/ana":                            false,
		"":                                       false,
	} {
		if got := a.IsOurs(address); got != want {
			t.Errorf("%q ours: %v", address, got)
		}
	}
}

func TestOnlyASiteOnTheInternetIsSpokenOf(t *testing.T) {
	cases := map[string]struct {
		a    Addresses
		want error
	}{
		"a live site":              {Addresses{"example.org", "https://example.org", "https://api.example.org"}, nil},
		"no domain":                {Addresses{"", "https://example.org", "https://api.example.org"}, ErrNoDomain},
		"development":              {from(env(map[string]string{})), ErrNotPublic},
		"a name of its own":        {Addresses{"anahita", "https://anahita", "https://anahita"}, ErrNotPublic},
		"a private address":        {Addresses{"example.org", "https://example.org", "https://10.0.0.5"}, ErrNotPublic},
		"the API in the clear":     {Addresses{"example.org", "https://example.org", "http://api.example.org"}, ErrNotSecure},
		"the web app in the clear": {Addresses{"example.org", "http://example.org", "https://api.example.org"}, ErrNotSecure},
	}

	for name, c := range cases {
		if got := c.a.CanFederate(); !errors.Is(got, c.want) {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
}
