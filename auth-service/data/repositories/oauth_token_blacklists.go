package repositories

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

// OAuthTokenBlacklistInterface is the oauth token blacklist repository interface
type OAuthTokenBlacklistInterface interface {
	Add(jti string, ttl time.Duration) error
	Exists(jti string) (bool, error)
}

// NewOAuthTokenBlacklist creates a new oauth token blacklist repository
func NewOAuthTokenBlacklist(rdb *redis.Client) OAuthTokenBlacklistInterface {
	return &OAuthTokenBlacklist{rdb: rdb}
}

// OAuthTokenBlacklist is the oauth token blacklist repository
type OAuthTokenBlacklist struct {
	rdb *redis.Client
}

func (repo *OAuthTokenBlacklist) key(jti string) string {
	return fmt.Sprintf("oauth:blacklist:%s", jti)
}

// Add adds a token jti to the blacklist with a TTL matching the token's remaining lifetime
func (repo *OAuthTokenBlacklist) Add(jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	if err := repo.rdb.Set(repo.key(jti), "1", ttl).Err(); err != nil {
		return err
	}

	return nil
}

// Exists checks if a token jti is in the blacklist
func (repo *OAuthTokenBlacklist) Exists(jti string) (bool, error) {
	result, err := repo.rdb.Exists(repo.key(jti)).Result()
	if err != nil {
		return false, err
	}

	return result > 0, nil
}
