package handlers

import (
	"auth/data/entities"
	"fmt"
	"log"

	libs_constants "anahita-libs/constants"
	"anahita-libs/sessions"

	"github.com/labstack/echo/v4"
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
// sign in again regardless. Authenticated callers, who are running
// inside a session they mean to keep, want revokeIdPSessionsExcept.
func (h *oauthHandler) revokeAllIdPSessions(user *entities.User) (int, error) {
	return h.revokeIdPSessions(user, "")
}

// revokeIdPSessionsExcept deletes every IdP session belonging to a
// person apart from the one this request arrived on.
//
// The authenticated counterpart to revokeAllIdPSessions, for flows
// that run inside a session worth keeping — an authenticated password
// change signs the person out everywhere else, not everywhere.
//
// The session to spare is identified from the person_session cookie on
// this request. WHEN THAT COOKIE IS ABSENT THIS REVOKES EVERYTHING,
// which is the common case rather than an edge one: the SPA reaches
// this endpoint through the BFF with an X-OAuth-Token, and the auth
// host's cookie does not travel on that hop.
//
// That fallback is the deliberate choice of the two available. Sparing
// nothing costs the person a fresh sign-in the next time they pass
// through /oauth/authorize — their BFF session and its refresh token
// are untouched here, so nothing breaks under them in the meantime.
// Sparing everything, which is the only alternative when the acting
// session cannot be named, would leave an attacker's IdP cookie live
// through the exact request meant to lock them out.
func (h *oauthHandler) revokeIdPSessionsExcept(
	c echo.Context,
	user *entities.User,
) (int, error) {
	// Errors are swallowed on purpose: no cookie and an unreadable
	// cookie lead to the same place, which is that this request cannot
	// name its own IdP session and therefore spares none.
	currentSessionID, _ := h.Session.GetViewerSessionID(c, libs_constants.UserSessionKey)

	return h.revokeIdPSessions(user, currentSessionID)
}

// revokeIdPSessions is the shared implementation. exceptSessionID is
// the store key to keep, or "" to keep none.
func (h *oauthHandler) revokeIdPSessions(
	user *entities.User,
	exceptSessionID string,
) (int, error) {
	if user == nil {
		return 0, nil
	}

	indexKey := personSessionIndexKey(user.ID)

	sessionIDs, err := h.Session.SMembers(indexKey)
	if err != nil {
		return 0, fmt.Errorf("read session index: %w", err)
	}

	doomed := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if exceptSessionID != "" && sessionID == exceptSessionID {
			continue
		}
		doomed = append(doomed, sessionID)
	}

	if len(doomed) == 0 {
		// Nothing to revoke. The index key goes only when it is being
		// emptied — dropping it while a spared session is still listed
		// would leave that session live and unreachable by any future
		// revocation, which is worse than a set that never shrinks.
		if exceptSessionID == "" || len(sessionIDs) == 0 {
			_ = h.Session.Del(indexKey)
		}
		return 0, nil
	}

	// One round trip for all of them. Some will already have expired
	// on their own — deleting a missing key is a no-op, so the count
	// returned here is sessions that *could* have been live rather
	// than a precise figure. Good enough for a log line, and not used
	// for anything else.
	if err := h.Session.Del(doomed...); err != nil {
		return 0, fmt.Errorf("delete sessions: %w", err)
	}

	if exceptSessionID == "" {
		_ = h.Session.Del(indexKey)
	} else {
		// Rebuild the index around the survivor rather than dropping
		// it. SAdd re-applies the TTL, so the spared session stays
		// revocable later; the members just deleted are left behind
		// and are harmless, since a stale ID deletes to a no-op.
		if err := h.Session.SAdd(indexKey, exceptSessionID, sessions.SessionTimeout); err != nil {
			log.Printf("session index: failed to reindex spared session (user=%d): %v",
				user.ID, err)
		}
	}

	return len(doomed), nil
}
