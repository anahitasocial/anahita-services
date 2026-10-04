package sessions

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/go-redis/redis"
)

// RedisTLSEnv turns on TLS for the Redis connection. Managed Redis and Valkey
// offerings usually accept nothing else, and until this existed the address
// and password were the only things that could be configured.
//
// Read from the environment here, not through each service's config, because
// every service that uses Redis builds its client in this package and the
// setting means the same thing in all of them.
const RedisTLSEnv = "REDIS_TLS"

// redisConnection is what both clients in this package need to connect.
type redisConnection interface {
	GetRedisAddr() string
	GetRedisPassword() string
	GetRedisDB() int
}

// redisOptions builds the client options, with TLS when REDIS_TLS is true.
//
// An unparsable value is an error, not "false": a deployment that asked for
// TLS with REDIS_TLS=yes and silently got a plaintext connection would send
// session data in the clear while believing otherwise.
func redisOptions(config redisConnection) (*redis.Options, error) {
	options := &redis.Options{
		Addr:     config.GetRedisAddr(),
		Password: config.GetRedisPassword(),
		DB:       config.GetRedisDB(),
	}

	value := os.Getenv(RedisTLSEnv)
	if value == "" {
		return options, nil
	}

	useTLS, err := strconv.ParseBool(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be true or false, not %q", RedisTLSEnv, value)
	}

	if useTLS {
		// The server's certificate is checked against the host in
		// REDIS_ADDR, using the system's trusted roots.
		host, _, err := net.SplitHostPort(options.Addr)
		if err != nil {
			host = options.Addr
		}
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	}

	return options, nil
}
