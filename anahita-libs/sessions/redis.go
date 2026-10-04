package sessions

import (
	"fmt"
	"net/http"
	"time"

	"anahita-libs/crypt"

	"github.com/go-redis/redis"
	"github.com/labstack/echo/v4"
)

type redisSession struct {
	encrypt *crypt.Encryption
	client  *redis.Client
	config  ConfigInterface
}

func NewRedisSession(config ConfigInterface) SessionInterface {
	options, err := redisOptions(config)
	if err != nil {
		panic(err)
	}
	client := redis.NewClient(options)

	pong, err := client.Ping().Result()
	if err != nil {
		panic(err)
	}

	fmt.Println(pong)

	return &redisSession{
		encrypt: &crypt.Encryption{},
		client:  client,
		config:  config,
	}
}

func (s *redisSession) Add(c echo.Context, key string, value string) (string, error) {
	return s.AddWithTTL(c, key, value, SessionTimeout)
}

func (s *redisSession) AddWithTTL(c echo.Context, key string, value string, ttl time.Duration) (string, error) {
	rawID, err := GenerateRandomString(SessionIDLength)
	if err != nil {
		return "", err
	}

	cookie := new(http.Cookie)
	cookie.Name = key
	cookie.Value = rawID
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

	sessionID := HashString(rawID)

	aesKey := s.config.GetSecureCookieKey()
	redisValue, err := s.encrypt.Encrypt([]byte(aesKey), value)
	if err != nil {
		return "", err
	}

	if err := s.client.Set(sessionID, redisValue, ttl).Err(); err != nil {
		return "", err
	}

	return sessionID, nil
}

func (s *redisSession) Read(c echo.Context, key string) (string, error) {
	cookie, err := c.Cookie(key)
	if err != nil {
		return "", err
	}

	sessionID := HashString(cookie.Value)
	encryptedValue, err := s.client.Get(sessionID).Bytes()
	if err != nil {
		return "", err
	}

	aesKey := s.config.GetSecureCookieKey()
	value, err := s.encrypt.Decrypt([]byte(aesKey), string(encryptedValue))
	if err != nil {
		return "", err
	}

	return value, nil
}

func (s *redisSession) Edit(c echo.Context, key string, value string) error {
	cookie, err := c.Cookie(key)
	if err != nil {
		return err
	}

	sessionID := HashString(cookie.Value)

	ttl, err := s.client.TTL(sessionID).Result()
	if err != nil || ttl <= 0 {
		ttl = OAuthSessionTimeout
	}

	aesKey := s.config.GetSecureCookieKey()
	redisValue, err := s.encrypt.Encrypt([]byte(aesKey), value)
	if err != nil {
		return err
	}

	return s.client.Set(sessionID, redisValue, ttl).Err()
}

func (s *redisSession) Delete(c echo.Context, key string) error {
	cookie, err := c.Request().Cookie(key)
	if err == http.ErrNoCookie {
		return err
	}
	redisKey := HashString(cookie.Value)
	if err := s.client.Del(redisKey).Err(); err != nil {
		return err
	}

	cookie = &http.Cookie{
		Name:     key,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	}

	http.SetCookie(c.Response(), cookie)

	return nil
}

func (s *redisSession) HasSessionByID(c echo.Context, key string, sessionID string) (bool, error) {
	value, err := s.client.Get(sessionID).Bytes()
	if err != nil {
		return false, err
	}

	if len(value) != 0 {
		return true, nil
	}

	return false, nil
}

func (s *redisSession) DeleteBySessionID(c echo.Context, key string, sessionID string) error {
	if err := s.client.Del(sessionID).Err(); err != nil {
		return err
	}

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

func (s *redisSession) GetViewerSessionID(c echo.Context, key string) (string, error) {
	cookie, err := c.Request().Cookie(key)
	if err != nil {
		return "", err
	}

	sessionID := HashString(cookie.Value)

	return sessionID, nil
}

func (s *redisSession) SetNX(key string, value string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(key, value, ttl).Result()
}

// Del removes one or more keys. Deleting a key that does not exist is
// not an error — Redis reports a count, not a failure, and a caller
// dropping a list of session IDs will routinely include some that
// already expired on their own.
func (s *redisSession) Del(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(keys...).Err()
}

// SAdd adds a member to a set and refreshes the set's expiry.
//
// The TTL is applied to the whole set on every add, so the index
// outlives its newest member rather than expiring while sessions are
// still live. Members are not individually expirable in Redis, which
// means the set will accumulate IDs of sessions that have since
// expired on their own. That is harmless: a stale ID deletes to a
// no-op, and the set is dropped wholesale when the caller revokes.
func (s *redisSession) SAdd(key string, member string, ttl time.Duration) error {
	if err := s.client.SAdd(key, member).Err(); err != nil {
		return err
	}
	return s.client.Expire(key, ttl).Err()
}

// SMembers returns every member of a set, or an empty slice if the
// key does not exist.
//
// A missing key is not an error. Someone with no active sessions and
// someone whose index has expired are the same case from the caller's
// point of view: nothing to revoke.
func (s *redisSession) SMembers(key string) ([]string, error) {
	members, err := s.client.SMembers(key).Result()
	if err == redis.Nil {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return members, nil
}

// Incr increments a counter and returns its new value.
//
// The TTL is applied only on the increment that creates the key,
// which is what makes this a fixed window rather than a sliding one.
// Refreshing the expiry on every call would let a caller hammering
// the key push its own reset indefinitely into the future — the
// counter would never expire while abuse continued, which sounds
// strict but in practice means an attacker can lock a legitimate
// owner out of the same key forever.
//
// Not transactional. Two callers can both see a missing key and both
// call Expire; the second is a no-op against the same value, so the
// worst case is an identical expiry set twice. A crash between INCR
// and EXPIRE leaves a counter without a TTL, which fails closed —
// stricter than intended, never looser.
func (s *redisSession) Incr(key string, ttl time.Duration) (int64, error) {
	count, err := s.client.Incr(key).Result()
	if err != nil {
		return 0, err
	}

	if count == 1 {
		if err := s.client.Expire(key, ttl).Err(); err != nil {
			return count, err
		}
	}

	return count, nil
}

// Exists reports whether a key is present.
func (s *redisSession) Exists(key string) (bool, error) {
	n, err := s.client.Exists(key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
