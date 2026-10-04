package sessions

import (
	"crypto/tls"
	"testing"
)

type fakeRedisConnection struct{ addr string }

func (f fakeRedisConnection) GetRedisAddr() string     { return f.addr }
func (f fakeRedisConnection) GetRedisPassword() string { return "secret" }
func (f fakeRedisConnection) GetRedisDB() int          { return 2 }

func TestRedisOptions(t *testing.T) {
	conn := fakeRedisConnection{addr: "cache.example.com:6380"}

	t.Run("no TLS unless asked for", func(t *testing.T) {
		for _, value := range []string{"", "false", "0"} {
			t.Setenv(RedisTLSEnv, value)
			options, err := redisOptions(conn)
			if err != nil {
				t.Fatalf("%q: %v", value, err)
			}
			if options.TLSConfig != nil {
				t.Errorf("%q: TLS was turned on", value)
			}
			if options.Addr != conn.addr || options.Password != "secret" || options.DB != 2 {
				t.Errorf("%q: the connection settings were not carried over: %+v", value, options)
			}
		}
	})

	t.Run("TLS checks the certificate against the host", func(t *testing.T) {
		for _, value := range []string{"true", "1"} {
			t.Setenv(RedisTLSEnv, value)
			options, err := redisOptions(conn)
			if err != nil {
				t.Fatalf("%q: %v", value, err)
			}
			if options.TLSConfig == nil {
				t.Fatalf("%q: TLS was not turned on", value)
			}
			if options.TLSConfig.ServerName != "cache.example.com" {
				t.Errorf("server name = %q, want the host from the address", options.TLSConfig.ServerName)
			}
			if options.TLSConfig.InsecureSkipVerify {
				t.Error("certificate verification is off")
			}
			if options.TLSConfig.MinVersion < tls.VersionTLS12 {
				t.Error("TLS below 1.2 is allowed")
			}
		}
	})

	// "yes" must not quietly mean a plaintext connection.
	t.Run("a value that is not a boolean is an error", func(t *testing.T) {
		t.Setenv(RedisTLSEnv, "yes")
		if options, err := redisOptions(conn); err == nil {
			t.Errorf("accepted, with TLS %v", options.TLSConfig != nil)
		}
	})
}
