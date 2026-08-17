package handlers

import (
	"auth/data/entities"
	"fmt"
	"log"

	"anahita-libs/sessions"
)

// personSessionIndexPrefix namespaces the per-person index of IdP
// session IDs.
//
// The session store keys sessions by a hash of a random string, which
// carries no information about who they belong to and cannot be
// scanned by owner. This index is what makes "sign this person out
// everywhere" answerable at all.
const personSessionIndexPrefix = "person_sessions:"

// personSessionIndexKey returns the index key for one person.
//
// Keyed by PersonID rather than UUID only because that is what every
// call site already has in hand. Either works; what matters is that
// the write side in indexIdPSession and the read side in
// revokeAllIdPSessions never disagree, which is why both go through
// this function.
func personSessionIndexKey(userID int64) string {
	return fmt.Sprintf("%s%d", personSessionIndexPrefix, userID)
}

// indexIdPSession records a freshly created IdP session against its
// owner.
//
// Call this immediately after Session.Add for PersonSessionKey — the
// return value of that call is the store's own key for the session,
// which is precisely what has to be indexed. Passing anything else
// produces an index that looks populated and revokes nothing.
//
// Best-effort. A failure here costs the ability to remotely revoke
// this one session later; failing the sign-in over it would cost the
// person their login now. Logged loudly because a silently empty
// index turns password reset into theatre.
func (h *oauthHandler) indexIdPSession(userID int64, sessionID string) {
	if sessionID == "" {
		log.Printf("session index: empty session id, not indexing (user=%d)", userID)
		return
	}

	err := h.Session.SAdd(
		personSessionIndexKey(userID),
		sessionID,
		sessions.SessionTimeout,
	)
	if err != nil {
		log.Printf("session index: failed to index IdP session (user=%d): %v",
			userID, err)
	}
}

// revokeAllIdPSessions deletes every IdP session belonging to a
// person, then clears the index.
//
// This is the half of account recovery that revoking refresh tokens
// does not cover. An attacker whose refresh token is revoked simply
// hits /oauth/authorize again; ReadAuthorize finds their person_session
// cookie still valid, mints a fresh authorization code without
// prompting, and they are back in with a new token pair — and
// recordAuthLogAfterIssuance even suppresses the notification email,
// because a recent auth log exists for that client. Without this, a
// password reset locks an attacker out for roughly as long as it takes
// their SPA to retry.
//
// Returns the number of sessions dropped. Zero is a success: someone
// with no live sessions and someone whose index has aged out are the
// same case, and neither is a failure.
//
// The person's own cookie is invalidated too, which is intended — the
// reset flow deliberately does not auto-login, so they were going to
// sign in again regardless.
func (h *oauthHandler) revokeAllIdPSessions(user *entities.User) (int, error) {
	if user == nil {
		return 0, nil
	}

	indexKey := personSessionIndexKey(user.ID)

	sessionIDs, err := h.Session.SMembers(indexKey)
	if err != nil {
		return 0, fmt.Errorf("read session index: %w", err)
	}

	if len(sessionIDs) == 0 {
		// Still drop the index key: it may hold only stale members,
		// and leaving it costs a set that never shrinks.
		_ = h.Session.Del(indexKey)
		return 0, nil
	}

	// One round trip for all of them. Some will already have expired
	// on their own — deleting a missing key is a no-op, so the count
	// returned here is sessions that *could* have been live rather
	// than a precise figure. Good enough for a log line, and not used
	// for anything else.
	if err := h.Session.Del(sessionIDs...); err != nil {
		return 0, fmt.Errorf("delete sessions: %w", err)
	}

	_ = h.Session.Del(indexKey)

	return len(sessionIDs), nil
}
