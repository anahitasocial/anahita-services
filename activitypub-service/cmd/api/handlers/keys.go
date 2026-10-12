package handlers

import (
	"errors"

	"activitypub/data/repositories"
	"activitypub/keys"
)

// publicKeyOf is the public half of the key a person signs with, made the
// first time it is asked for.
//
// Two requests may both find no key and both make one. The table lets one
// in; the other reads that one. So whoever asks first, everybody is told
// the same key, which matters: a server that was told one key will refuse
// what is signed with another.
func (h *Handlers) publicKeyOf(nodeID int64) (string, error) {
	key, err := h.Keys.Active(nodeID)
	if err != nil {
		return "", err
	}
	if key != nil {
		return key.PublicKey, nil
	}

	pair, err := keys.Generate()
	if err != nil {
		return "", err
	}
	encrypted, err := h.Crypt.Encrypt(pair.PrivatePEM)
	if err != nil {
		return "", err
	}

	err = h.Keys.Add(&repositories.Key{
		NodeID:           nodeID,
		Algorithm:        keys.Algorithm,
		PublicKey:        pair.PublicPEM,
		PrivateEncrypted: encrypted,
	})
	if errors.Is(err, repositories.ErrKeyExists) {
		key, err = h.Keys.Active(nodeID)
		if err != nil {
			return "", err
		}
		if key == nil {
			return "", repositories.ErrKeyExists
		}
		return key.PublicKey, nil
	}
	if err != nil {
		return "", err
	}

	return pair.PublicPEM, nil
}
