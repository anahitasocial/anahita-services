package repositories

import (
	"errors"
	"strings"
	"testing"

	"auth/data/entities"

	"anahita-libs/crypt"
)

const (
	testPEM      = "-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAKj34GkxFhD90vcNLYLInFEX6Ppy1tPf9Cnzj4p4WGeKLs1Pt8Qu\n-----END RSA PRIVATE KEY-----\n"
	sealerKey    = "0123456789abcdef0123456789abcdef"
	otherKey     = "fedcba9876543210fedcba9876543210"
	sealedMarker = "sealed:v1:"
)

// memoryKeys is the table, as far as the wrapper is concerned: what was
// written is what is read back, with nothing done to it.
type memoryKeys struct {
	OAuthSigningKeysInterface
	rows map[int64]*entities.OAuthSigningKey
	next int64
}

func newMemoryKeys() *memoryKeys {
	return &memoryKeys{rows: map[int64]*entities.OAuthSigningKey{}}
}

func (m *memoryKeys) Add(key *entities.OAuthSigningKey) (*entities.OAuthSigningKey, error) {
	m.next++
	stored := *key
	stored.ID = m.next
	m.rows[stored.ID] = &stored
	out := stored
	return &out, nil
}

func (m *memoryKeys) ReadByID(id int64) (*entities.OAuthSigningKey, error) {
	row, ok := m.rows[id]
	if !ok {
		return nil, entities.ErrSigningKeyNotFound
	}
	out := *row
	return &out, nil
}

func (m *memoryKeys) ReadByKeyID(keyID string) (*entities.OAuthSigningKey, error) {
	for _, row := range m.rows {
		if row.KeyID == keyID {
			out := *row
			return &out, nil
		}
	}
	return nil, entities.ErrSigningKeyNotFound
}

func (m *memoryKeys) ReadActiveSigningKey() (*entities.OAuthSigningKey, error) {
	for _, row := range m.rows {
		if row.Active {
			out := *row
			return &out, nil
		}
	}
	return nil, entities.ErrSigningKeyNotFound
}

// Browse leaves the private key out, as the real one does.
func (m *memoryKeys) Browse() ([]*entities.OAuthSigningKey, error) {
	var out []*entities.OAuthSigningKey
	for _, row := range m.rows {
		listed := *row
		listed.PrivateKey = ""
		out = append(out, &listed)
	}
	return out, nil
}

func (m *memoryKeys) EditPrivateKey(id int64, privateKey string) error {
	m.rows[id].PrivateKey = privateKey
	return nil
}

func sealerWith(t *testing.T, key string) KeySealer {
	t.Helper()
	sealer, err := crypt.NewLocalCrypt(key)
	if err != nil {
		t.Fatalf("crypt: %v", err)
	}
	return sealer
}

func localKey(kid, privateKey string) *entities.OAuthSigningKey {
	key := entities.NewOAuthSigningKey()
	key.KeyID = kid
	key.Active = true
	key.PrivateKey = privateKey
	return key
}

// The table must never hold a private key anybody with read access to it
// could use. What is written is sealed; what is read back is the PEM.
func TestAPrivateKeyIsSealedInTheTableAndReadBackAsPEM(t *testing.T) {
	table := newMemoryKeys()
	keys := NewSealedOAuthSigningKeys(table, sealerWith(t, sealerKey))

	added, err := keys.Add(localKey("kid-1", testPEM))
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if added.PrivateKey != testPEM {
		t.Error("the key handed back by Add is not the PEM that was given")
	}

	stored := table.rows[added.ID].PrivateKey
	if !strings.HasPrefix(stored, sealedMarker) {
		t.Fatalf("stored value does not start with %q: %.40s", sealedMarker, stored)
	}
	if strings.Contains(stored, "BEGIN RSA PRIVATE KEY") || strings.Contains(stored, "MIIBOgIBAAJBAKj34") {
		t.Fatal("the PEM is readable in the stored value")
	}

	reads := map[string]func() (*entities.OAuthSigningKey, error){
		"by id":      func() (*entities.OAuthSigningKey, error) { return keys.ReadByID(added.ID) },
		"by kid":     func() (*entities.OAuthSigningKey, error) { return keys.ReadByKeyID("kid-1") },
		"the active": keys.ReadActiveSigningKey,
	}
	for name, read := range reads {
		t.Run("read "+name, func(t *testing.T) {
			key, err := read()
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if key.PrivateKey != testPEM {
				t.Errorf("private key = %.40q, want the PEM", key.PrivateKey)
			}
		})
	}

	// Reading must not write the PEM back into the table.
	if !isSealed(table.rows[added.ID].PrivateKey) {
		t.Error("a read left the stored key unsealed")
	}
}

// Sealing the same key twice gives two different values, so the table does
// not show that two rows hold the same key.
func TestSealingIsNotDeterministic(t *testing.T) {
	table := newMemoryKeys()
	keys := NewSealedOAuthSigningKeys(table, sealerWith(t, sealerKey))

	a, _ := keys.Add(localKey("kid-a", testPEM))
	b, _ := keys.Add(localKey("kid-b", testPEM))

	if table.rows[a.ID].PrivateKey == table.rows[b.ID].PrivateKey {
		t.Error("the same PEM sealed to the same value twice")
	}
}

// A database written before keys were sealed holds them in the clear. They
// are sealed at start-up, once, and the keys still work afterwards.
func TestKeysStoredInTheClearAreSealedOnce(t *testing.T) {
	table := newMemoryKeys()
	table.Add(localKey("old-1", testPEM))
	table.Add(localKey("old-2", testPEM))
	kms := localKey("kms-1", "")
	kms.Active = false
	table.Add(kms)

	keys := NewSealedOAuthSigningKeys(table, sealerWith(t, sealerKey))

	// Before the upgrade a clear key is still readable, so nothing breaks
	// between deploying this and the upgrade running.
	if key, err := keys.ReadByKeyID("old-1"); err != nil || key.PrivateKey != testPEM {
		t.Fatalf("a key in the clear could not be read: %v", err)
	}

	count, err := keys.SealStored()
	if err != nil || count != 2 {
		t.Fatalf("sealed %d, err = %v; want 2", count, err)
	}

	for id, row := range table.rows {
		switch row.KeyID {
		case "kms-1":
			if row.PrivateKey != "" {
				t.Errorf("row %d: a KMS row was given a private key", id)
			}
		default:
			if !isSealed(row.PrivateKey) {
				t.Errorf("row %d (%s) is still in the clear", id, row.KeyID)
			}
		}
	}

	if key, err := keys.ReadByKeyID("old-2"); err != nil || key.PrivateKey != testPEM {
		t.Errorf("after sealing, the key no longer reads back as its PEM: %v", err)
	}

	again, err := keys.SealStored()
	if err != nil || again != 0 {
		t.Errorf("a second run sealed %d, err = %v; want 0", again, err)
	}
}

// With a different secrets key the stored key cannot be opened, and that is
// an error, not an empty key that fails somewhere less obvious.
func TestAKeySealedWithAnotherSecretCannotBeOpened(t *testing.T) {
	table := newMemoryKeys()
	added, _ := NewSealedOAuthSigningKeys(table, sealerWith(t, sealerKey)).Add(localKey("kid-1", testPEM))

	key, err := NewSealedOAuthSigningKeys(table, sealerWith(t, otherKey)).ReadByID(added.ID)
	if err == nil {
		t.Fatalf("opened with the wrong secret: %.30q", key.PrivateKey)
	}
}

func TestAStoredValueThatIsNeitherSealedNorPEMIsAnError(t *testing.T) {
	table := newMemoryKeys()
	added, _ := table.Add(localKey("kid-1", "garbage"))

	_, err := NewSealedOAuthSigningKeys(table, sealerWith(t, sealerKey)).ReadByID(added.ID)
	if !errors.Is(err, ErrPrivateKeyUnreadable) {
		t.Errorf("err = %v, want ErrPrivateKeyUnreadable", err)
	}
}

func TestANotFoundPassesThrough(t *testing.T) {
	keys := NewSealedOAuthSigningKeys(newMemoryKeys(), sealerWith(t, sealerKey))

	if _, err := keys.ReadActiveSigningKey(); !errors.Is(err, entities.ErrSigningKeyNotFound) {
		t.Errorf("err = %v, want ErrSigningKeyNotFound", err)
	}
}
