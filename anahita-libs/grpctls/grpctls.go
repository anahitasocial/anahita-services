// Package grpctls gives the internal gRPC calls mutual TLS: the call is
// encrypted, and each side proves who it is with a certificate signed by the
// installation's own certificate authority.
//
// Without it graph-grpc, file-grpc and mail-grpc answer anything that can
// reach their ports, which inside a cluster is every pod in it.
//
// Two settings, read from the environment by every service the same way:
//
//	GRPC_TLS      "on" or "off". Unset means on, except with ENV=dev.
//	GRPC_TLS_DIR  the directory holding tls.crt, tls.key and ca.crt.
//	              Default /run/secrets/grpc.
//
// The three file names are the ones cert-manager writes into a Secret, so a
// Certificate's Secret mounts straight onto the directory.
//
// It fails closed. Outside development a service with no certificates does
// not fall back to plain text: a server refuses to start and a client refuses
// to dial. Turning it off takes an explicit GRPC_TLS=off, which is for an
// installation whose service mesh does this job instead.
package grpctls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"anahita-libs/constants"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ModeOn  = "on"
	ModeOff = "off"

	// DefaultDir is where the certificates are mounted unless GRPC_TLS_DIR
	// says otherwise.
	DefaultDir = "/run/secrets/grpc"

	certFile = "tls.crt"
	keyFile  = "tls.key"
	caFile   = "ca.crt"

	// recheckEvery is how often the files are looked at again. cert-manager
	// renews a certificate weeks before it expires, so a minute's delay in
	// picking the new one up costs nothing, and a handshake never reads the
	// disk more often than this.
	recheckEvery = time.Minute
)

// Settings is what the environment asks for.
type Settings struct {
	Enabled bool
	Dir     string
}

// FromEnv reads GRPC_TLS, GRPC_TLS_DIR and ENV.
func FromEnv() (Settings, error) {
	settings := Settings{Dir: os.Getenv("GRPC_TLS_DIR")}
	if settings.Dir == "" {
		settings.Dir = DefaultDir
	}

	switch mode := strings.ToLower(strings.TrimSpace(os.Getenv("GRPC_TLS"))); mode {
	case ModeOn:
		settings.Enabled = true
	case ModeOff:
		settings.Enabled = false
	case "":
		// Unset is only plain text on a developer's machine, where ENV is dev
		// or unset. Anywhere else, forgetting the setting must not be what
		// turns encryption off.
		env := os.Getenv("ENV")
		settings.Enabled = env != "" && env != constants.ConfigEnvDev
	default:
		return settings, fmt.Errorf("GRPC_TLS must be %q or %q, not %q", ModeOn, ModeOff, mode)
	}

	return settings, nil
}

var (
	shared     *store
	sharedErr  error
	sharedOnce sync.Once
	sharedOn   bool
)

// load reads the settings once per process. Clients ask for their dial option
// on every call, and must not re-read the environment or the disk each time.
func load() (*store, bool, error) {
	sharedOnce.Do(func() {
		settings, err := FromEnv()
		if err != nil {
			sharedErr = err
			return
		}

		sharedOn = settings.Enabled
		if !settings.Enabled {
			log.Println("grpctls: internal gRPC calls are NOT encrypted (GRPC_TLS is off)")
			return
		}

		shared, sharedErr = newStore(settings.Dir)
		if sharedErr == nil {
			log.Printf("grpctls: internal gRPC calls use mutual TLS, certificates from %s", settings.Dir)
		}
	})

	return shared, sharedOn, sharedErr
}

// ServerOption is passed to grpc.NewServer. With TLS on, the server accepts
// only callers holding a certificate from the installation's authority.
func ServerOption() (grpc.ServerOption, error) {
	s, enabled, err := load()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return grpc.Creds(insecure.NewCredentials()), nil
	}

	return grpc.Creds(credentials.NewTLS(s.serverConfig())), nil
}

// DialOption is passed to grpc.NewClient. With TLS on, the client presents
// its certificate and accepts only a server whose certificate comes from the
// installation's authority and names the host that was dialled.
func DialOption() (grpc.DialOption, error) {
	s, enabled, err := load()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(s.clientConfig())), nil
}

// store holds the certificate and the authority, and notices when the files
// change.
//
// The reason it exists: a certificate read once at start-up is held for the
// life of the process, and cert-manager replaces the files long before that
// ends. Without a re-read everything keeps working until the old certificate
// expires, then every service fails at the same moment, weeks after the
// change that caused it.
type store struct {
	dir string

	// now is replaced in tests, to move past recheckEvery without waiting.
	now func() time.Time

	mu      sync.Mutex
	checked time.Time
	stamp   string
	cert    *tls.Certificate
	pool    *x509.CertPool
}

func newStore(dir string) (*store, error) {
	s := &store{dir: dir, now: time.Now}

	// Read now, so a missing or broken file stops the service at start-up
	// rather than at the first call.
	if _, _, err := s.current(); err != nil {
		return nil, err
	}

	return s, nil
}

// current returns the certificate and authority in force, re-reading the
// files when they have changed since the last look.
func (s *store) current() (*tls.Certificate, *x509.CertPool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cert != nil && s.now().Sub(s.checked) < recheckEvery {
		return s.cert, s.pool, nil
	}
	s.checked = s.now()

	stamp, err := s.fileStamp()
	if err == nil && stamp == s.stamp && s.cert != nil {
		return s.cert, s.pool, nil
	}

	var cert *tls.Certificate
	var pool *x509.CertPool
	if err == nil {
		cert, pool, err = s.read()
	}

	if err != nil {
		// A renewal caught half-written must not take down a service that
		// has a working certificate. Keep the one in hand and look again
		// next time. Only with nothing in hand is this an error.
		if s.cert != nil {
			log.Printf("grpctls: could not re-read certificates, keeping the current ones: %v", err)
			return s.cert, s.pool, nil
		}
		return nil, nil, err
	}

	if s.cert != nil {
		log.Printf("grpctls: certificates in %s changed, now using the new ones", s.dir)
	}
	s.cert, s.pool, s.stamp = cert, pool, stamp

	return s.cert, s.pool, nil
}

// fileStamp changes whenever any of the three files is replaced.
func (s *store) fileStamp() (string, error) {
	var parts []string
	for _, name := range []string{certFile, keyFile, caFile} {
		// Stat follows symbolic links, which is how a Kubernetes Secret
		// volume swaps its files.
		info, err := os.Stat(filepath.Join(s.dir, name))
		if err != nil {
			return "", fmt.Errorf("grpctls: %w (GRPC_TLS is on; set GRPC_TLS_DIR, or GRPC_TLS=off if a service mesh encrypts these calls)", err)
		}
		parts = append(parts, fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size()))
	}
	return strings.Join(parts, "|"), nil
}

func (s *store) read() (*tls.Certificate, *x509.CertPool, error) {
	cert, err := tls.LoadX509KeyPair(filepath.Join(s.dir, certFile), filepath.Join(s.dir, keyFile))
	if err != nil {
		return nil, nil, fmt.Errorf("grpctls: reading %s and %s: %w", certFile, keyFile, err)
	}

	authority, err := os.ReadFile(filepath.Join(s.dir, caFile))
	if err != nil {
		return nil, nil, fmt.Errorf("grpctls: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(authority) {
		return nil, nil, fmt.Errorf("grpctls: %s holds no certificate", caFile)
	}

	return &cert, pool, nil
}

// serverConfig builds a fresh configuration for each connection, so both the
// server's own certificate and the authority it checks callers against follow
// the files.
func (s *store) serverConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			cert, pool, err := s.current()
			if err != nil {
				return nil, err
			}

			return &tls.Config{
				MinVersion:   tls.VersionTLS13,
				Certificates: []tls.Certificate{*cert},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    pool,
				// gRPC adds this to the outer configuration only, and
				// refuses a connection that did not negotiate it.
				NextProtos: []string{"h2"},
			}, nil
		},
	}
}

func (s *store) clientConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, _, err := s.current()
			return cert, err
		},

		// The server IS verified, in VerifyConnection below. The built-in
		// check is switched off only because it reads the authority once,
		// from RootCAs, and could not follow a replaced ca.crt.
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			_, pool, err := s.current()
			if err != nil {
				return err
			}
			if len(state.PeerCertificates) == 0 {
				return errors.New("grpctls: the server sent no certificate")
			}

			options := x509.VerifyOptions{
				Roots: pool,
				// The name that was dialled, not one the server offers.
				DNSName:       state.ServerName,
				Intermediates: x509.NewCertPool(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
			for _, intermediate := range state.PeerCertificates[1:] {
				options.Intermediates.AddCert(intermediate)
			}

			_, err = state.PeerCertificates[0].Verify(options)
			return err
		},
	}
}
