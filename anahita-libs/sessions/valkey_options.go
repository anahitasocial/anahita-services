package sessions

import (
	"anahita-libs/getenv"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"

	"github.com/go-redis/redis"
)

// ValkeyTLSEnv turns on TLS for the Valkey connection. Managed Valkey and Redis
// offerings usually accept nothing else, and until this existed the address
// and password were the only things that could be configured.
//
// Read from the environment here, not through each service's config, because
// every service that uses Valkey builds its client in this package and the
// setting means the same thing in all of them.
const ValkeyTLSEnv = "VALKEY_TLS"

// oldTLSEnv is ValkeyTLSEnv's name before Valkey took Redis's place
// (roadmap row 165), read for one release.
const oldTLSEnv = "REDIS_TLS"

// valkeyConnection is what both clients in this package need to connect.
type valkeyConnection interface {
	GetValkeyAddr() string
	GetValkeyPassword() string
	GetValkeyDB() int
}

// valkeyOptions builds the client options, with TLS when VALKEY_TLS is true.
//
// An unparsable value is an error, not "false": a deployment that asked for
// TLS with VALKEY_TLS=yes and silently got a plaintext connection would send
// session data in the clear while believing otherwise.
func valkeyOptions(config valkeyConnection) (*redis.Options, error) {
	options := &redis.Options{
		Addr:     config.GetValkeyAddr(),
		Password: config.GetValkeyPassword(),
		DB:       config.GetValkeyDB(),
	}

	value := getenv.Renamed(ValkeyTLSEnv, oldTLSEnv)
	if value == "" {
		return options, nil
	}

	useTLS, err := strconv.ParseBool(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be true or false, not %q", ValkeyTLSEnv, value)
	}

	if useTLS {
		// The server's certificate is checked against the host in
		// VALKEY_ADDR, using the system's trusted roots.
		host, _, err := net.SplitHostPort(options.Addr)
		if err != nil {
			host = options.Addr
		}
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	}

	return options, nil
}
