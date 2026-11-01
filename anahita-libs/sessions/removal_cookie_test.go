package sessions

import (
	"net/http"
	"testing"
	"time"
)

// A cookie is taken away as it was given: Secure outside development, or a
// browser may keep the one it has (CodeQL's alerts 19 to 22).
func TestRemovalCookieMatchesTheCookieItRemoves(t *testing.T) {
	cases := []struct {
		name     string
		dev      bool
		secure   bool
		sameSite http.SameSite
	}{
		{"on a site", false, true, http.SameSiteStrictMode},
		{"in development", true, false, http.SameSiteLaxMode},
	}

	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			cookie := removalCookie("session", each.dev)

			if cookie.Name != "session" || cookie.Value != "" || cookie.Path != "/" {
				t.Errorf("not the same cookie, emptied: %+v", cookie)
			}
			if !cookie.Expires.Equal(time.Unix(0, 0)) {
				t.Errorf("Expires = %v, want long ago", cookie.Expires)
			}
			if !cookie.HttpOnly {
				t.Error("HttpOnly is not set")
			}
			if cookie.Secure != each.secure {
				t.Errorf("Secure = %v, want %v", cookie.Secure, each.secure)
			}
			if cookie.SameSite != each.sameSite {
				t.Errorf("SameSite = %v, want %v", cookie.SameSite, each.sameSite)
			}
		})
	}
}
