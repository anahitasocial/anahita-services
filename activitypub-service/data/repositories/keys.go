package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Key is a signing key as it is kept: the public half as it is told to
// other servers, the private half encrypted.
type Key struct {
	ID               int64
	NodeID           int64
	Algorithm        string
	PublicKey        string
	PrivateEncrypted string
	CreatedAt        time.Time
}

// ErrKeyExists says a node already has a key in use. Two requests can both
// find none and both make one; the second to save is told this, and reads
// the first one's.
var ErrKeyExists = errors.New("the node has a key already")

// KeysInterface is the signing keys of the people here.
type KeysInterface interface {
	// Active is the key a node signs with now, or nil when it has none.
	Active(nodeID int64) (*Key, error)
	// Add keeps a new key for a node that has none in use.
	Add(key *Key) error
}

type Keys struct {
	db *sql.DB
}

func NewKeys(db *sql.DB) *Keys {
	return &Keys{db: db}
}

const readActiveKey = `
SELECT id, node_id, algorithm, public_key, private_key_encrypted, created_at
FROM actor_keys
WHERE node_id = ?
  AND revoked_at IS NULL`

func (repo *Keys) Active(nodeID int64) (*Key, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	key := &Key{}
	err := repo.db.QueryRowContext(ctx, readActiveKey, nodeID).Scan(
		&key.ID, &key.NodeID, &key.Algorithm, &key.PublicKey, &key.PrivateEncrypted, &key.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return key, nil
}

const addKey = `
INSERT INTO actor_keys (node_id, algorithm, public_key, private_key_encrypted, created_at)
VALUES (?, ?, ?, ?, ?)`

func (repo *Keys) Add(key *Key) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, addKey, key.NodeID, key.Algorithm, key.PublicKey, key.PrivateEncrypted, time.Now().UTC())
	if err != nil && strings.Contains(err.Error(), "Error 1062") {
		return ErrKeyExists
	}
	return err
}
