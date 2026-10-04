package grpctls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// authority is a throwaway certificate authority.
type authority struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

var serial int64

func nextSerial() *big.Int {
	serial++
	return big.NewInt(serial)
}

func newAuthority(t *testing.T) *authority {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber:          nextSerial(),
		Subject:               pkix.Name{CommonName: "test authority"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return &authority{
		cert: cert,
		key:  key,
		pem:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}
}

// issue writes tls.crt, tls.key and ca.crt into dir: a certificate for the
// given names signed by this authority, and `trusted` as the authority to
// check the other side against. It returns the certificate's serial number.
func (a *authority) issue(t *testing.T, dir string, trusted *authority, names ...string) *big.Int {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: nextSerial(),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     names,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, a.cert, &key.PublicKey, a.key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	write(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	write(caFile, trusted.pem)

	return template.SerialNumber
}

func mustStore(t *testing.T, dir string) *store {
	t.Helper()
	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// serve starts a gRPC server on a free local port and returns its port.
func serve(t *testing.T, option grpc.ServerOption) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	server := grpc.NewServer(option)
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	return port
}

// call makes one request over a new connection.
func call(t *testing.T, target string, option grpc.DialOption) error {
	t.Helper()

	conn, err := grpc.NewClient(target, option)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	return err
}

func tlsServer(s *store) grpc.ServerOption {
	return grpc.Creds(credentials.NewTLS(s.serverConfig()))
}

func tlsClient(s *store) grpc.DialOption {
	return grpc.WithTransportCredentials(credentials.NewTLS(s.clientConfig()))
}

func TestBothSidesWithCertificatesFromTheAuthorityConnect(t *testing.T) {
	ca := newAuthority(t)
	serverDir, clientDir := t.TempDir(), t.TempDir()
	ca.issue(t, serverDir, ca, "localhost")
	ca.issue(t, clientDir, ca, "some-caller")

	port := serve(t, tlsServer(mustStore(t, serverDir)))

	if err := call(t, "localhost:"+port, tlsClient(mustStore(t, clientDir))); err != nil {
		t.Fatalf("call failed: %v", err)
	}
}

func TestServerRefusesACallerWithoutACertificate(t *testing.T) {
	ca := newAuthority(t)
	serverDir := t.TempDir()
	ca.issue(t, serverDir, ca, "localhost")
	port := serve(t, tlsServer(mustStore(t, serverDir)))

	// Plain text.
	if err := call(t, "localhost:"+port, grpc.WithTransportCredentials(insecure.NewCredentials())); err == nil {
		t.Error("a plain-text caller was served")
	}

	// TLS, trusting the server, but presenting nothing.
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	anonymous := grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: pool}))
	if err := call(t, "localhost:"+port, anonymous); err == nil {
		t.Error("a caller with no certificate was served")
	}
}

func TestServerRefusesACallerFromAnotherAuthority(t *testing.T) {
	ca, stranger := newAuthority(t), newAuthority(t)
	serverDir, clientDir := t.TempDir(), t.TempDir()
	ca.issue(t, serverDir, ca, "localhost")
	// Signed by the stranger, though it trusts the real server.
	stranger.issue(t, clientDir, ca, "some-caller")

	port := serve(t, tlsServer(mustStore(t, serverDir)))

	if err := call(t, "localhost:"+port, tlsClient(mustStore(t, clientDir))); err == nil {
		t.Error("a caller signed by another authority was served")
	}
}

func TestClientRefusesAServerFromAnotherAuthority(t *testing.T) {
	ca, stranger := newAuthority(t), newAuthority(t)
	serverDir, clientDir := t.TempDir(), t.TempDir()
	// An impostor: signed by the stranger, and willing to accept our callers.
	stranger.issue(t, serverDir, ca, "localhost")
	ca.issue(t, clientDir, ca, "some-caller")

	port := serve(t, tlsServer(mustStore(t, serverDir)))

	if err := call(t, "localhost:"+port, tlsClient(mustStore(t, clientDir))); err == nil {
		t.Error("the client talked to a server signed by another authority")
	}
}

func TestClientRefusesAServerUnderAnotherName(t *testing.T) {
	ca := newAuthority(t)
	serverDir, clientDir := t.TempDir(), t.TempDir()
	// A genuine certificate, for a different service than the one dialled.
	ca.issue(t, serverDir, ca, "mail-grpc-service")
	ca.issue(t, clientDir, ca, "some-caller")

	port := serve(t, tlsServer(mustStore(t, serverDir)))

	if err := call(t, "localhost:"+port, tlsClient(mustStore(t, clientDir))); err == nil {
		t.Error("the client accepted a certificate that does not name the host it dialled")
	}
}

// A renewed certificate, and a replaced authority, are picked up without a
// restart: the failure this guards against shows up weeks later, when the
// certificate read at start-up expires.
func TestReplacedFilesArePickedUp(t *testing.T) {
	ca := newAuthority(t)
	serverDir, clientDir := t.TempDir(), t.TempDir()
	first := ca.issue(t, serverDir, ca, "localhost")
	ca.issue(t, clientDir, ca, "some-caller")

	clock := time.Now()
	serverStore, clientStore := mustStore(t, serverDir), mustStore(t, clientDir)
	serverStore.now = func() time.Time { return clock }
	clientStore.now = func() time.Time { return clock }

	port := serve(t, tlsServer(serverStore))
	target := "localhost:" + port

	presented := func() *big.Int {
		cert, _, err := serverStore.current()
		if err != nil {
			t.Fatal(err)
		}
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			t.Fatal(err)
		}
		return leaf.SerialNumber
	}

	if err := call(t, target, tlsClient(clientStore)); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Renewal: a new certificate from the same authority.
	second := ca.issue(t, serverDir, ca, "localhost")

	if got := presented(); got.Cmp(first) != 0 {
		t.Fatalf("the files were re-read before the recheck interval passed (serial %v)", got)
	}

	clock = clock.Add(recheckEvery + time.Second)

	if got := presented(); got.Cmp(second) != 0 {
		t.Fatalf("serving serial %v after renewal, want %v", got, second)
	}
	if err := call(t, target, tlsClient(clientStore)); err != nil {
		t.Fatalf("call after renewal failed: %v", err)
	}

	// The authority itself is replaced, on both sides.
	replacement := newAuthority(t)
	replacement.issue(t, serverDir, replacement, "localhost")
	replacement.issue(t, clientDir, replacement, "some-caller")
	clock = clock.Add(recheckEvery + time.Second)

	if err := call(t, target, tlsClient(clientStore)); err != nil {
		t.Fatalf("call after the authority was replaced failed: %v", err)
	}
}

// A renewal caught half-written keeps the working certificate.
func TestABrokenRenewalKeepsTheCurrentCertificate(t *testing.T) {
	ca := newAuthority(t)
	dir := t.TempDir()
	first := ca.issue(t, dir, ca, "localhost")

	clock := time.Now()
	s := mustStore(t, dir)
	s.now = func() time.Time { return clock }

	if err := os.WriteFile(filepath.Join(dir, certFile), []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(recheckEvery + time.Second)

	cert, _, err := s.current()
	if err != nil {
		t.Fatalf("a broken renewal became an error: %v", err)
	}
	leaf, _ := x509.ParseCertificate(cert.Certificate[0])
	if leaf.SerialNumber.Cmp(first) != 0 {
		t.Errorf("serial %v, want the original %v", leaf.SerialNumber, first)
	}
}

func TestMissingFilesAreAnError(t *testing.T) {
	if _, err := newStore(t.TempDir()); err == nil {
		t.Error("an empty directory was accepted")
	}
}

func TestFromEnv(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		env     string
		enabled bool
		wantErr bool
	}{
		{name: "unset on a developer's machine is off", mode: "", env: "dev", enabled: false},
		{name: "unset with no ENV is off", mode: "", env: "", enabled: false},
		{name: "unset in production is on", mode: "", env: "prod", enabled: true},
		{name: "unset on staging is on", mode: "", env: "staging", enabled: true},
		{name: "on in development", mode: "on", env: "dev", enabled: true},
		{name: "off in production, for a service mesh", mode: "off", env: "prod", enabled: false},
		{name: "case and spaces are forgiven", mode: " ON ", env: "dev", enabled: true},
		{name: "anything else is refused", mode: "true", env: "prod", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GRPC_TLS", tc.mode)
			t.Setenv("ENV", tc.env)
			t.Setenv("GRPC_TLS_DIR", "")

			settings, err := FromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatal("no error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if settings.Enabled != tc.enabled {
				t.Errorf("enabled = %v, want %v", settings.Enabled, tc.enabled)
			}
			if settings.Dir != DefaultDir {
				t.Errorf("dir = %q, want %q", settings.Dir, DefaultDir)
			}
		})
	}
}
