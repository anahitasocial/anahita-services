package sessions

import (
	"net/http"
	"time"

	"anahita-libs/crypt"

	"github.com/labstack/echo/v4"
)

type cookieSession struct {
	encrypt *crypt.Encryption
	config  ConfigInterface
}

func NewCookieSession(config ConfigInterface) SessionInterface {
	return &cookieSession{
		config:  config,
		encrypt: &crypt.Encryption{},
	}
}

func (s *cookieSession) Add(c echo.Context, key string, value string) (string, error) {
	return s.AddWithTTL(c, key, value, SessionTimeout)
}

func (s *cookieSession) AddWithTTL(c echo.Context, key string, value string, ttl time.Duration) (string, error) {
	aesKey := s.config.GetSecureCookieKey()
	encrypted, err := s.encrypt.Encrypt([]byte(aesKey), value)

	if err != nil {
		return "", err
	}

	cookie := new(http.Cookie)
	cookie.Name = key
	cookie.Value = encrypted
	cookie.Expires = time.Now().Add(ttl)
	cookie.HttpOnly = true
	cookie.Path = "/"

	if s.config.IsDev() {
		cookie.SameSite = http.SameSiteLaxMode
		cookie.Secure = false
	} else {
		cookie.SameSite = http.SameSiteStrictMode
		cookie.Secure = true
	}

	c.SetCookie(cookie)

	return encrypted, nil
}

func (s *cookieSession) Read(c echo.Context, key string) (string, error) {
	cookie, err := c.Request().Cookie(key)
	if err == http.ErrNoCookie {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	aesKey := s.config.GetSecureCookieKey()
	decrypted, err := s.encrypt.Decrypt([]byte(aesKey), cookie.Value)

	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

func (s *cookieSession) Edit(c echo.Context, key string, value string) error {
	_, err := s.AddWithTTL(c, key, value, SessionTimeout)
	return err
}

func (s *cookieSession) Delete(c echo.Context, key string) error {
	cookie := &http.Cookie{
		Name:     key,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	}

	http.SetCookie(c.Response(), cookie)

	return nil
}

func (s *cookieSession) HasSessionByID(c echo.Context, key string, sessionID string) (bool, error) {
	cookie, err := c.Request().Cookie(key)
	if err == http.ErrNoCookie {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if cookie.Value == sessionID {
		return true, nil
	}

	return false, nil
}

func (s *cookieSession) DeleteBySessionID(c echo.Context, key string, sessionID string) error {
	for _, cookie := range c.Request().Cookies() {
		if cookie.Value == sessionID {
			cookie := &http.Cookie{
				Name:     cookie.Name,
				Value:    "",
				Path:     "/",
				Expires:  time.Unix(0, 0),
				HttpOnly: true,
			}

			http.SetCookie(c.Response(), cookie)
		}
	}

	return nil
}

func (s *cookieSession) GetViewerSessionID(c echo.Context, key string) (string, error) {
	cookie, err := c.Request().Cookie(key)
	if err == http.ErrNoCookie {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}

func (s *cookieSession) SetNX(key string, value string, ttl time.Duration) (bool, error) {
	return true, nil
}

func (s *cookieSession) Del(keys ...string) error {
	return nil
}

// SAdd is a no-op.
//
// KNOWN LIMITATION, and the reason this backend should not be used
// anywhere real. Cookie sessions live entirely in the client's cookie
// — there is no server-side record to index and nothing to delete
// remotely. A caller maintaining a session index gets an index that
// is always empty, and "sign this person out everywhere" silently
// does nothing.
//
// That is not a gap this backend can close: revoking a session you
// cannot reach is not possible by construction. Redis is required for
// server-side revocation, which in practice means required for any
// environment where an account might need to be taken back.
func (s *cookieSession) SAdd(key string, member string, ttl time.Duration) error {
	return nil
}

// SMembers always returns empty. See SAdd.
func (s *cookieSession) SMembers(key string) ([]string, error) {
	return []string{}, nil
}
