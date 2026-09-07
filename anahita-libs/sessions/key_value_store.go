package sessions

import (
	"github.com/go-redis/redis"
)

// A Redis store for services that need keys and nothing else.
//
// The full session store carries cookie handling, encryption and a request
// context, and its config interface asks for a domain name and a cookie key to
// match. A service that only reads a step-up marker needs none of that, and
// making it supply them would be six methods of ceremony for two Redis calls.

// StoreConfigInterface is the configuration a bare key store needs.
type StoreConfigInterface interface {
	GetRedisAddr() string
	GetRedisPassword() string
	GetRedisDB() int
}

type keyValueStore struct {
	client *redis.Client
}

// NewKeyValueStore connects to Redis for key operations only.
//
// Returns an error rather than panicking, unlike NewRedisSession. A service
// that cannot reach Redis should be able to decide for itself whether that is
// fatal — for person-service it means losing a guard, which is worth starting
// without and logging loudly, not worth refusing to serve profiles over.
func NewKeyValueStore(config StoreConfigInterface) (KeyValueInterface, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.GetRedisAddr(),
		Password: config.GetRedisPassword(),
		DB:       config.GetRedisDB(),
	})

	if _, err := client.Ping().Result(); err != nil {
		return nil, err
	}

	return &keyValueStore{client: client}, nil
}

func (s *keyValueStore) Exists(key string) (bool, error) {
	count, err := s.client.Exists(key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *keyValueStore) Del(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(keys...).Err()
}
