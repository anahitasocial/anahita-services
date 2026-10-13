package handlers

import (
	"crypto/rsa"
	"log"
	"net/http"
	"sync"

	"activitypub/ap"
	"activitypub/keys"

	"github.com/labstack/echo/v4"
)

// The installation itself, as an actor.
//
// Many servers answer nobody who does not say who is asking: a request for
// somebody's description has to be signed. When this installation looks
// somebody up, or checks a signature, it is not doing so for any one person
// here, so it signs as itself. This is who that is, with the key those
// servers fetch to check.
//
// It has no node: nobody follows it and it posts nothing. Its key is kept
// like a person's, under the number no node has.
const instanceNodeID = 0

// InstanceActor describes the installation to a server that wants to check
// a request it signed.
func (h *Handlers) InstanceActor(c echo.Context) error {
	if ok, _ := h.siteFederates(); !ok {
		return c.NoContent(http.StatusNotFound)
	}

	publicKey, err := h.publicKeyOf(instanceNodeID)
	if err != nil {
		log.Println("activitypub key:", err)
		return c.NoContent(http.StatusServiceUnavailable)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=180")
	return activityJSON(c, ap.NewInstanceActor(h.addresses, publicKey))
}

// signing is the private halves that have been read, so that each is
// decrypted once and not at every request.
type signing struct {
	mu   sync.Mutex
	keys map[int64]*rsa.PrivateKey
}

// signerFor is the key a node signs with, and what that key is called.
func (h *Handlers) signerFor(nodeID int64, keyID string) (string, *rsa.PrivateKey, error) {
	h.signing.mu.Lock()
	defer h.signing.mu.Unlock()

	if key := h.signing.keys[nodeID]; key != nil {
		return keyID, key, nil
	}

	// Made if there is none: the installation's own is first wanted here.
	if _, err := h.publicKeyOf(nodeID); err != nil {
		return "", nil, err
	}
	kept, err := h.Keys.Active(nodeID)
	if err != nil || kept == nil {
		return "", nil, err
	}
	privatePEM, err := h.Crypt.Decrypt(kept.PrivateEncrypted)
	if err != nil {
		return "", nil, err
	}
	key, err := keys.ParsePrivate(privatePEM)
	if err != nil {
		return "", nil, err
	}

	if h.signing.keys == nil {
		h.signing.keys = map[int64]*rsa.PrivateKey{}
	}
	h.signing.keys[nodeID] = key
	return keyID, key, nil
}

// SignAsInstance is who a fetch is signed as.
func (h *Handlers) SignAsInstance() (string, *rsa.PrivateKey, error) {
	return h.signerFor(instanceNodeID, h.addresses.InstanceKey())
}
