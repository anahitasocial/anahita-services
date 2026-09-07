package signing

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"auth/data/entities"
	"auth/data/repositories"
)

// The signing package mints and verifies the tokens every service in the fleet
// trusts, and had no tests. These use real RSA keys and real signatures rather
// than doubles for the crypto — the properties worth holding are cryptographic
// ones, and a fake signer would assert nothing about them.
//
// The property that matters most is alg-confusion: a verifier that takes the
// algorithm from the token's own header, rather than from the key it looked
// up, can be talked into using the wrong primitive by an attacker who controls
// that header.

// testKey holds a generated RSA keypair in the PEM forms the entity stores.
type testKey struct {
	kid        string
	privatePEM string
	publicPEM  string
	private    *rsa.PrivateKey
}

// 2048 bits, generated once per run: key generation is slow enough that doing
// it per subtest noticeably lengthens the suite.
var sharedKey = mustGenerateKey("kid-1")
var otherKey = mustGenerateKey("kid-2")

func mustGenerateKey(kid string) testKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  PEMBlockTypeRSAPrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		panic(err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  PEMBlockTypePublicKey,
		Bytes: publicDER,
	})

	return testKey{
		kid:        kid,
		privatePEM: string(privatePEM),
		publicPEM:  string(publicPEM),
		private:    key,
	}
}

func (k testKey) entity(alg string, active bool) *entities.OAuthSigningKey {
	return &entities.OAuthSigningKey{
		KeyID:      k.kid,
		Algorithm:  alg,
		PublicKey:  k.publicPEM,
		PrivateKey: k.privatePEM,
		Active:     active,
		ExpiresAt:  time.Now().Add(90 * 24 * time.Hour),
	}
}

// fakeKeys stands in for the signing-key repository.
//
// Embeds the interface so only the two methods the signer and verifier use are
// written; anything else this package starts calling panics by name.
type fakeKeys struct {
	repositories.OAuthSigningKeysInterface

	active  *entities.OAuthSigningKey
	byKeyID map[string]*entities.OAuthSigningKey

	activeErr error
	readErr   error
}

func newFakeKeys(keys ...*entities.OAuthSigningKey) *fakeKeys {
	f := &fakeKeys{byKeyID: map[string]*entities.OAuthSigningKey{}}
	for _, k := range keys {
		f.byKeyID[k.KeyID] = k
		if k.Active {
			f.active = k
		}
	}
	return f
}

func (f *fakeKeys) ReadActiveSigningKey() (*entities.OAuthSigningKey, error) {
	if f.activeErr != nil {
		return nil, f.activeErr
	}
	if f.active == nil {
		return nil, entities.ErrSigningKeyNotFound
	}
	return f.active, nil
}

func (f *fakeKeys) ReadByKeyID(keyID string) (*entities.OAuthSigningKey, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	if k, ok := f.byKeyID[keyID]; ok {
		return k, nil
	}
	return nil, entities.ErrSigningKeyNotFound
}

// ---------------------------------------------------------------------
// Round trip
// ---------------------------------------------------------------------

func TestSignThenVerifyRoundTrip(t *testing.T) {
	repo := newFakeKeys(sharedKey.entity(entities.SigningAlgRS256, true))

	signer, err := NewSignerLocal(repo, entities.SigningAlgRS256)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	input := []byte("eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiI0MiJ9")

	signature, err := signer.Sign(context.Background(), input)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(signature) == 0 {
		t.Fatal("empty signature")
	}

	verifier := NewVerifierLocal(repo)
	if err := verifier.Verify(context.Background(), sharedKey.kid, entities.SigningAlgRS256, input, signature); err != nil {
		t.Errorf("a signature this package produced did not verify: %v", err)
	}

	if kid, err := signer.ActiveKeyID(context.Background()); err != nil || kid != sharedKey.kid {
		t.Errorf("ActiveKeyID = %q, %v; want %q", kid, err, sharedKey.kid)
	}
	if got := signer.Algorithm(); got != entities.SigningAlgRS256 {
		t.Errorf("Algorithm = %q, want RS256", got)
	}
}

// A signature is over specific bytes. Changing one of them must break it —
// this is the whole point of signing the token rather than merely stamping it.
func TestVerifyRejectsTamperedInput(t *testing.T) {
	repo := newFakeKeys(sharedKey.entity(entities.SigningAlgRS256, true))
	signer, _ := NewSignerLocal(repo, entities.SigningAlgRS256)
	verifier := NewVerifierLocal(repo)

	input := []byte("eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiI0MiJ9")
	signature, err := signer.Sign(context.Background(), input)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	tampered := []byte("eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiI5OSJ9")

	err = verifier.Verify(context.Background(), sharedKey.kid, entities.SigningAlgRS256, tampered, signature)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("a token with a swapped subject verified: %v", err)
	}

	// And a mangled signature over the correct input.
	bad := make([]byte, len(signature))
	copy(bad, signature)
	bad[0] ^= 0xff

	if err := verifier.Verify(context.Background(), sharedKey.kid, entities.SigningAlgRS256, input, bad); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("a corrupted signature verified: %v", err)
	}
}

// Signed by one key, verified against another. This is the forged-token case.
func TestVerifyRejectsAnotherKeysSignature(t *testing.T) {
	signingRepo := newFakeKeys(sharedKey.entity(entities.SigningAlgRS256, true))
	signer, _ := NewSignerLocal(signingRepo, entities.SigningAlgRS256)

	input := []byte("payload")
	signature, err := signer.Sign(context.Background(), input)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// The verifier resolves kid-2, whose public key does not match.
	impostor := otherKey.entity(entities.SigningAlgRS256, true)
	impostor.KeyID = sharedKey.kid
	verifier := NewVerifierLocal(newFakeKeys(impostor))

	if err := verifier.Verify(context.Background(), sharedKey.kid, entities.SigningAlgRS256, input, signature); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("a signature from a different key verified: %v", err)
	}
}

// ---------------------------------------------------------------------
// Alg confusion — the attack this design exists to stop
// ---------------------------------------------------------------------

// The alg in a JWT header is attacker-controlled. If the verifier trusts it
// over the algorithm registered against the key, an attacker can coerce the
// wrong primitive. The verifier must compare the two and refuse.
func TestVerifyRejectsAlgorithmMismatch(t *testing.T) {
	repo := newFakeKeys(sharedKey.entity(entities.SigningAlgRS256, true))
	verifier := NewVerifierLocal(repo)

	for _, headerAlg := range []string{"ES256", "HS256", "none", "", "rs256"} {
		t.Run("header alg "+headerAlg, func(t *testing.T) {
			err := verifier.Verify(context.Background(), sharedKey.kid, headerAlg, []byte("x"), []byte("y"))
			if !errors.Is(err, ErrAlgorithmMismatch) {
				t.Errorf("alg %q was not rejected as a mismatch: %v", headerAlg, err)
			}
		})
	}
}

// The mismatch check must run BEFORE any cryptographic work, so a token
// claiming "none" never reaches a verification path at all.
func TestAlgorithmIsCheckedBeforeTheSignature(t *testing.T) {
	repo := newFakeKeys(sharedKey.entity(entities.SigningAlgRS256, true))
	verifier := NewVerifierLocal(repo)

	// An empty signature with alg=none: if the order were reversed this could
	// only fail as an invalid signature, which is a weaker refusal.
	err := verifier.Verify(context.Background(), sharedKey.kid, "none", []byte("x"), nil)
	if !errors.Is(err, ErrAlgorithmMismatch) {
		t.Errorf("expected an algorithm mismatch, got %v", err)
	}
}

func TestVerifyUnknownKID(t *testing.T) {
	repo := newFakeKeys(sharedKey.entity(entities.SigningAlgRS256, true))
	verifier := NewVerifierLocal(repo)

	if err := verifier.Verify(context.Background(), "kid-that-does-not-exist", entities.SigningAlgRS256, []byte("x"), []byte("y")); !errors.Is(err, ErrUnknownKID) {
		t.Errorf("expected ErrUnknownKID, got %v", err)
	}

	// An empty kid short-circuits without a repository read.
	if err := verifier.Verify(context.Background(), "", entities.SigningAlgRS256, []byte("x"), []byte("y")); !errors.Is(err, ErrUnknownKID) {
		t.Errorf("expected ErrUnknownKID for an empty kid, got %v", err)
	}
}

// A key past its expiry is still a valid verification source: tokens it signed
// may still be in flight, and pruning is the cron's job. The signer is what
// guarantees nothing new is signed with it.
func TestVerifyAcceptsAnExpiredOrInactiveKey(t *testing.T) {
	key := sharedKey.entity(entities.SigningAlgRS256, false)
	key.ExpiresAt = time.Now().Add(-time.Hour)

	repo := newFakeKeys(sharedKey.entity(entities.SigningAlgRS256, true))
	signer, _ := NewSignerLocal(repo, entities.SigningAlgRS256)

	input := []byte("payload")
	signature, _ := signer.Sign(context.Background(), input)

	verifier := NewVerifierLocal(newFakeKeys(key))
	if err := verifier.Verify(context.Background(), key.KeyID, entities.SigningAlgRS256, input, signature); err != nil {
		t.Errorf("an expired-but-present key refused verification: %v", err)
	}
}

// ---------------------------------------------------------------------
// Signer construction and key loading
// ---------------------------------------------------------------------

func TestNewSignerLocalRejectsUnsupportedAlgorithms(t *testing.T) {
	repo := newFakeKeys()

	if _, err := NewSignerLocal(repo, ""); err == nil {
		t.Error("accepted an empty algorithm")
	}

	_, err := NewSignerLocal(repo, "ES256")
	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Errorf("expected ErrUnsupportedAlgorithm for ES256, got %v", err)
	}
}

// A key of the wrong algorithm in the table would otherwise be signed with as
// though it were RS256 — producing tokens whose alg header says one thing and
// whose signature is another.
func TestSignerRefusesAKeyOfTheWrongAlgorithm(t *testing.T) {
	repo := newFakeKeys(sharedKey.entity("ES256", true))
	signer, _ := NewSignerLocal(repo, entities.SigningAlgRS256)

	if _, err := signer.Sign(context.Background(), []byte("x")); !errors.Is(err, ErrAlgorithmMismatch) {
		t.Errorf("expected ErrAlgorithmMismatch, got %v", err)
	}
	if _, err := signer.ActiveKeyID(context.Background()); !errors.Is(err, ErrAlgorithmMismatch) {
		t.Errorf("ActiveKeyID: expected ErrAlgorithmMismatch, got %v", err)
	}
}

// No key at all is a startup misconfiguration and must surface as its own
// sentinel rather than as a generic database error.
func TestSignerReportsNoActiveKey(t *testing.T) {
	repo := newFakeKeys()
	signer, _ := NewSignerLocal(repo, entities.SigningAlgRS256)

	if _, err := signer.Sign(context.Background(), []byte("x")); !errors.Is(err, ErrNoActiveSigningKey) {
		t.Errorf("expected ErrNoActiveSigningKey, got %v", err)
	}
}

// A database failure is NOT the same as "no key configured" — one is
// transient, the other is fatal at startup, and collapsing them would send an
// operator looking for a misconfiguration during an outage.
func TestSignerDistinguishesAStoreFailureFromAMissingKey(t *testing.T) {
	repo := newFakeKeys()
	repo.activeErr = errors.New("connection refused")

	signer, _ := NewSignerLocal(repo, entities.SigningAlgRS256)

	_, err := signer.Sign(context.Background(), []byte("x"))
	if errors.Is(err, ErrNoActiveSigningKey) {
		t.Error("a store failure was reported as a missing key")
	}
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("the underlying failure was lost: %v", err)
	}
}

// ---------------------------------------------------------------------
// PEM parsing
// ---------------------------------------------------------------------

func TestParseRSAKeysAcceptsBothEncodings(t *testing.T) {
	// PKCS#1, which is what the generator above emits.
	if _, err := ParseRSAPrivateKeyFromPEM(sharedKey.privatePEM); err != nil {
		t.Errorf("PKCS#1 private key: %v", err)
	}
	// PKIX public.
	if _, err := ParseRSAPublicKeyFromPEM(sharedKey.publicPEM); err != nil {
		t.Errorf("PKIX public key: %v", err)
	}

	// PKCS#8 private, the other accepted form.
	der, err := x509.MarshalPKCS8PrivateKey(sharedKey.private)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pkcs8 := pem.EncodeToMemory(&pem.Block{Type: PEMBlockTypePrivateKey, Bytes: der})
	if _, err := ParseRSAPrivateKeyFromPEM(string(pkcs8)); err != nil {
		t.Errorf("PKCS#8 private key: %v", err)
	}

	// PKCS#1 public, the other accepted form.
	pkcs1Pub := pem.EncodeToMemory(&pem.Block{
		Type:  PEMBlockTypeRSAPublicKey,
		Bytes: x509.MarshalPKCS1PublicKey(&sharedKey.private.PublicKey),
	})
	if _, err := ParseRSAPublicKeyFromPEM(string(pkcs1Pub)); err != nil {
		t.Errorf("PKCS#1 public key: %v", err)
	}
}

func TestParseRSAKeysRejectsGarbage(t *testing.T) {
	cases := map[string]string{
		"empty":          "",
		"not pem":        "hello",
		"truncated":      "-----BEGIN RSA PRIVATE KEY-----\nnope\n-----END RSA PRIVATE KEY-----",
		"wrong block":    "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----",
		"public as priv": sharedKey.publicPEM,
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseRSAPrivateKeyFromPEM(input); err == nil {
				t.Error("accepted as a private key")
			}
		})
	}

	if _, err := ParseRSAPublicKeyFromPEM("garbage"); err == nil {
		t.Error("accepted garbage as a public key")
	}
}

// A key that will not parse must fail the sign rather than producing something
// unverifiable.
func TestSignerFailsOnAnUnparseableKey(t *testing.T) {
	broken := sharedKey.entity(entities.SigningAlgRS256, true)
	broken.PrivateKey = "-----BEGIN RSA PRIVATE KEY-----\nnot base64\n-----END RSA PRIVATE KEY-----"

	signer, _ := NewSignerLocal(newFakeKeys(broken), entities.SigningAlgRS256)

	if _, err := signer.Sign(context.Background(), []byte("x")); err == nil {
		t.Error("signed with a key that does not parse")
	}
}

func TestVerifierFailsOnAnUnparseablePublicKey(t *testing.T) {
	broken := sharedKey.entity(entities.SigningAlgRS256, true)
	broken.PublicKey = "not a pem"

	verifier := NewVerifierLocal(newFakeKeys(broken))

	err := verifier.Verify(context.Background(), broken.KeyID, entities.SigningAlgRS256, []byte("x"), []byte("y"))
	if err == nil {
		t.Fatal("verified against a key that does not parse")
	}
	// Must NOT report a forged signature — the token may be perfectly good and
	// the problem is ours.
	if errors.Is(err, ErrInvalidSignature) {
		t.Error("a broken stored key was reported as an invalid signature")
	}
}

// ---------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------

type fakeFactoryConfig struct {
	backend string
	alias   string
}

func (c fakeFactoryConfig) GetOAuthSignerBackend() string { return c.backend }
func (c fakeFactoryConfig) GetOAuthKMSKeyAlias() string   { return c.alias }

func TestFactoryRejectsAnUnknownBackend(t *testing.T) {
	repo := newFakeKeys()
	cfg := fakeFactoryConfig{backend: "carrier-pigeon"}

	if _, err := NewSigner(context.Background(), cfg, repo); err == nil {
		t.Error("NewSigner accepted an unknown backend")
	}
	if _, err := NewVerifier(cfg, repo); err == nil {
		t.Error("NewVerifier accepted an unknown backend")
	}

	// An empty backend is the "operator forgot to set it" case, and must fail
	// at startup rather than defaulting to something.
	if _, err := NewSigner(context.Background(), fakeFactoryConfig{}, repo); err == nil {
		t.Error("NewSigner accepted an empty backend")
	}
}

// Both backends verify locally, because KMS-backed keys persist their public
// key in the same table — that is what lets other services verify with
// database access alone and no KMS permissions.
func TestFactoryVerifierIsLocalForBothBackends(t *testing.T) {
	repo := newFakeKeys()

	for _, backend := range []string{"local", "kms"} {
		t.Run(backend, func(t *testing.T) {
			v, err := NewVerifier(fakeFactoryConfig{backend: backend}, repo)
			if err != nil {
				t.Fatalf("NewVerifier: %v", err)
			}
			if _, ok := v.(*VerifierLocal); !ok {
				t.Errorf("backend %q produced %T, want *VerifierLocal", backend, v)
			}
		})
	}
}

func TestFactoryLocalSignerIsRS256(t *testing.T) {
	s, err := NewSigner(context.Background(), fakeFactoryConfig{backend: "local"}, newFakeKeys())
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if got := s.Algorithm(); got != entities.SigningAlgRS256 {
		t.Errorf("Algorithm = %q, want RS256", got)
	}
}
