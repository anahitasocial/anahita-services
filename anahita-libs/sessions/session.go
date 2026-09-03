package sessions

import (
	"time"

	"github.com/labstack/echo/v4"
)

type SessionInterface interface {
	// Core session operations
	Add(echo.Context, string, string) (string, error)
	AddWithTTL(echo.Context, string, string, time.Duration) (string, error)
	Read(echo.Context, string) (string, error)
	Edit(echo.Context, string, string) error
	Delete(echo.Context, string) error

	// Session ID operations
	HasSessionByID(echo.Context, string, string) (bool, error)
	DeleteBySessionID(echo.Context, string, string) error
	GetViewerSessionID(echo.Context, string) (string, error)

	// Redis operations
	SetNX(string, string, time.Duration) (bool, error)

	// Del removes one or more keys. Variadic so a caller holding a
	// list of session IDs can drop them in a single round trip
	// instead of a loop; single-key callers are unaffected.
	Del(...string) error

	// Set operations.
	//
	// Present so a caller can keep its own index of session IDs — the
	// store itself has no notion of who a session belongs to, since
	// keys are hashed random strings with no structure to enumerate.
	// Without an index there is no way to answer "sign this person out
	// everywhere", which is what a password reset has to do to
	// actually be a recovery.
	//
	// The index is deliberately the caller's business rather than
	// this package's: only the caller knows what a session is "of",
	// and baking person semantics into a generic session store would
	// be the wrong place for it.
	SAdd(key string, member string, ttl time.Duration) error
	SMembers(key string) ([]string, error)

	// Counter operations.
	//
	// Present so callers can rate limit. A limiter needs an atomic
	// read-modify-write — anything built out of Read-then-Edit races
	// exactly when it matters, which is when a lot of requests arrive
	// at once. Incr is that operation.
	//
	// Incr increments the counter at key and returns its new value,
	// applying ttl on the increment that creates it and leaving the
	// expiry alone afterwards. That gives a fixed window: the counter
	// dies ttl after the FIRST request in it, so a caller hammering
	// the key cannot push its own reset further away.
	//
	// Exists reports whether a key is present, for markers where the
	// value carries no information and only the presence does.
	Incr(key string, ttl time.Duration) (int64, error)
	Exists(key string) (bool, error)
}

func NewSession(config ConfigInterface) SessionInterface {
	if config.GetRedisAddr() != "" {
		return NewRedisSession(config)
	}

	return NewCookieSession(config)
}
