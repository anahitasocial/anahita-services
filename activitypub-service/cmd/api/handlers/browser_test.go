package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// THE RULE THIS GUARDS: a person's browser that opens an actor's or a
// post's address is sent to its page; a server asking for ActivityPub, by
// any of the types servers send, still gets the JSON.
func TestPageOrActivity(t *testing.T) {
	cases := []struct {
		accept   string
		redirect bool
	}{
		{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", true},
		{"text/html", true},
		{"application/activity+json", false},
		{`application/ld+json; profile="https://www.w3.org/ns/activitystreams"`, false},
		{"application/activity+json, text/html;q=0.1", false},
		{"application/json", false},
		{"*/*", false},
		{"", false},
	}
	for _, tc := range cases {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/ap/users/x", nil)
		if tc.accept != "" {
			req.Header.Set(echo.HeaderAccept, tc.accept)
		}
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := pageOrActivity(c, "https://banu.io/@anahita", map[string]string{"id": "x"}); err != nil {
			t.Fatal(err)
		}
		if tc.redirect {
			if rec.Code != http.StatusFound || rec.Header().Get(echo.HeaderLocation) != "https://banu.io/@anahita" {
				t.Errorf("%q: %d %q, want 302 to the profile", tc.accept, rec.Code, rec.Header().Get(echo.HeaderLocation))
			}
		} else if rec.Code != http.StatusOK {
			t.Errorf("%q: %d, want 200 with the JSON", tc.accept, rec.Code)
		}
		if rec.Header().Get("Vary") != "Accept" {
			t.Errorf("%q: Vary %q, want Accept", tc.accept, rec.Header().Get("Vary"))
		}
	}

	// With no page to go to, a browser gets the JSON as before.
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/ap/notes/x", nil)
	req.Header.Set(echo.HeaderAccept, "text/html")
	rec := httptest.NewRecorder()
	if err := pageOrActivity(e.NewContext(req, rec), "", map[string]string{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("no page: %d, want 200", rec.Code)
	}
}
