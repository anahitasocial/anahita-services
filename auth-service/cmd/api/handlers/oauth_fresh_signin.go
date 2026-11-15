package handlers

import (
	"fmt"
	"log"
	"time"
)

// What the "New sign-in" email announces: somebody gave a password, a code
// or a passkey. Not every code exchange is that. A browser whose sign-in
// session is still alive is given a new authorization code without being
// asked for anything, and an email for that tells the person of a sign-in
// that did not happen; a timer measured from the last sign-in missed it as
// soon as an address or a browser changed.
//
// So a sign-in marks the person, for a little while, and the code exchange
// that follows consumes the mark: it emails only if the mark was there.

// freshSignInPrefix namespaces the marks in the session store.
const freshSignInPrefix = "auth:fresh_signin:"

// freshSignInTTL is how long a sign-in waits for its code exchange: the
// redirect back to the app takes a moment, and ten minutes covers a slow
// one.
const freshSignInTTL = 10 * time.Minute

func freshSignInKey(userID int64) string {
	return fmt.Sprintf("%s%d", freshSignInPrefix, userID)
}

// markFreshSignIn records that the person just signed in. Best-effort: a
// failure costs one email, never the sign-in.
func (h *oauthHandler) markFreshSignIn(userID int64) {
	if h.Session == nil {
		return
	}
	if _, err := h.Session.SetNX(freshSignInKey(userID), "1", freshSignInTTL); err != nil {
		log.Printf("sign-in mark: could not set it (user=%d): %v", userID, err)
	}
}

// consumeFreshSignIn reports whether the person signed in just now, and
// takes the mark away, so that one sign-in sends one email. It fails open:
// when the store cannot be asked, the email is sent, an extra one costing
// less than a sign-in nobody is told of.
func (h *oauthHandler) consumeFreshSignIn(userID int64) bool {
	if h.Session == nil {
		return true
	}
	key := freshSignInKey(userID)
	fresh, err := h.Session.Exists(key)
	if err != nil {
		log.Printf("sign-in mark: could not read it (user=%d), sending the email: %v", userID, err)
		return true
	}
	if fresh {
		if err := h.Session.Del(key); err != nil {
			log.Printf("sign-in mark: could not remove it (user=%d): %v", userID, err)
		}
	}
	return fresh
}
