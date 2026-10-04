package handlers

import (
	"crypto/subtle"
	"log"

	"github.com/labstack/echo/v4"
)

// sessionKeyLoginCSRF holds one token for the forms a person fills in before
// they are signed in: sign-in, its code and two-factor steps, confirming an
// address, and resetting a password.
//
// One per browser session, not one per page shown. A token replaced on
// every render breaks the second tab somebody opened, and the back
// button, for no gain: what it protects against is another site
// submitting the form, and that site cannot read either token.
const sessionKeyLoginCSRF = "oauth_login_csrf"

// loginCSRFToken returns this browser session's token, making one if there is
// none. Empty only if the session store is failing, in which case the form
// that carries it will be refused, which is the safe way for that to go.
func (env *Env) loginCSRFToken(c echo.Context) string {
	if existing, err := env.Session.Read(c, sessionKeyLoginCSRF); err == nil && existing != "" {
		return existing
	}

	token, err := generateCSRFToken()
	if err != nil {
		log.Println("form token: could not generate one:", err)
		return ""
	}

	if _, err := env.Session.Add(c, sessionKeyLoginCSRF, token); err != nil {
		log.Println("form token: could not store it:", err)
		return ""
	}

	return token
}

// loginCSRFValid reports whether a submitted form carries this session's
// token. A session with no token has no valid form.
func (env *Env) loginCSRFValid(c echo.Context, submitted string) bool {
	stored, err := env.Session.Read(c, sessionKeyLoginCSRF)
	if err != nil || stored == "" || submitted == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(stored), []byte(submitted)) == 1
}
