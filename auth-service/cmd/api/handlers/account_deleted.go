package handlers

import (
	"log"
	"strings"

	"anahita-libs/event"
)

// ---------------------------------------------------------------------
// Account deletion — the credential half.
//
// person-service and group-service own the profile and mark it deleted;
// auth-service owns everything you can sign in with. Neither can do the other's
// half, and until now nobody did this one: deleting an account revoked nothing.
// A refresh token minted before the deletion kept working until it expired, the
// IdP cookie kept minting fresh ones from /oauth/authorize without prompting,
// and the passkeys stayed enrolled — so an account could be "deleted" and still
// be open to whoever already held a credential for it.
//
// The link is the event person-service has always published and nothing has
// ever consumed. Consuming it here keeps the two services apart: person-service
// does not need to know what a refresh token is, and this does not need to know
// what a profile is.
//
// Revocation is deliberately NOT deferred to the sweeper. The grace period
// exists so a person can come back, not so their old sessions can keep running
// for thirty days — restoring means signing in again, which is exactly the
// proof that should be required.
// ---------------------------------------------------------------------

type accountDeletedHandler struct {
	*Env
}

func newAccountDeletedHandler(env *Env) *accountDeletedHandler {
	return &accountDeletedHandler{env}
}

// HandleEvents reacts to node.actor.*.*.*.v1.
//
// Two actions matter, and they are the two halves of deletion:
//
//	deleted — scheduled. Revoke the credentials, tell the person, leave the
//	          account restorable.
//	purged  — the grace period expired and the sweeper has destroyed the
//	          content. Remove what auth-service still holds.
//
// Everything else is ignored rather than treated as an error: this is
// subscribed to the whole actor topic and most of what arrives is somebody
// editing their bio.
func (h *accountDeletedHandler) HandleEvents(payload *event.EventPayload) error {
	parts := strings.Split(payload.Name, ".")
	if len(parts) < 5 {
		return nil
	}

	if parts[4] == "purged" {
		return h.handlePurged(payload.ObjectID)
	}

	if parts[4] != "deleted" {
		return nil
	}

	// ObjectID is the actor that was deleted. Groups come through this topic
	// too and have no credentials, so a lookup miss is expected and not an
	// error worth returning — returning one would nack the message and have the
	// broker redeliver a group deletion forever.
	user, err := h.Repos.Users.ReadByID(payload.ObjectID)
	if err != nil || user == nil {
		log.Printf("account deleted: no credentials for actor %d, nothing to revoke",
			payload.ObjectID)
		return nil
	}

	h.revokeCredentials(user.ID)

	// Tell them, and tell them when. Deletion is gated on a step-up, so an
	// unexpected message here means somebody got past a passkey or the
	// password — and the grace period is the only window in which that is
	// still recoverable. Sent last, so it describes work already done.
	//
	// DeletedAt drives the date rather than time.Now(): the event may be
	// redelivered or handled late, and a mail naming a purge date a day out
	// from the real one is worse than no date at all.
	if user.DeletedAt != nil {
		deletedAt := *user.DeletedAt
		notifyUser := *user
		go func() {
			if err := h.Notif.AccountDelete.EmailScheduled(&notifyUser, deletedAt); err != nil {
				log.Println("account deleted: notification failed:", err)
			}
		}()
	}

	return nil
}

// revokeCredentials removes everything the person could sign in or act with.
//
// Best-effort throughout, and it does not stop at the first failure: the
// profile is already marked deleted, so returning an error here would nack the
// message and leave the OTHER credentials in place while the broker retries.
// Getting three of four revoked beats getting none.
//
// The same three revocations password reset performs, for the same reason —
// none of them are derived from the account's existence, so all three survive a
// deletion untouched unless something removes them.
func (h *accountDeletedHandler) revokeCredentials(userID int64) {
	user, err := h.Repos.Users.ReadByID(userID)
	if err != nil || user == nil {
		log.Println("account deleted: credential lookup failed:", err)
		return
	}

	// Passkeys. There is no foreign key to people — the schema comment on
	// webauthn_credentials says deletion must call this explicitly — and
	// nothing ever did.
	if revoked, err := h.Repos.WebAuthnCredentials.DeleteByPersonID(user.ID); err != nil {
		log.Println("account deleted: passkey revocation failed:", err)
	} else if revoked > 0 {
		log.Printf("account deleted: revoked %d passkey(s) (person=%d)", revoked, user.ID)
	}

	// Refresh tokens, via the auth log's session index.
	authlogHandler := newAuthLogHandler(h.Env)
	if revoked, err := authlogHandler.DeleteAllSessions(user); err != nil {
		log.Println("account deleted: session revocation failed:", err)
	} else if revoked > 0 {
		log.Printf("account deleted: revoked %d session(s) (person=%d)", revoked, user.ID)
	}

	// The IdP cookie on the auth host. Revoking refresh tokens alone leaves
	// this, and /oauth/authorize would honour it and mint a fresh pair without
	// prompting — putting the holder straight back into an account that no
	// longer exists.
	oauthHandler := newOAuthHandler(h.Env)
	if revoked, err := oauthHandler.revokeAllIdPSessions(user); err != nil {
		log.Println("account deleted: IdP session revocation failed:", err)
	} else if revoked > 0 {
		log.Printf("account deleted: revoked %d IdP session(s) (person=%d)", revoked, user.ID)
	}

	// Any step-up marker. Small, but it would otherwise let a credential change
	// go through on an account being deleted.
	h.clearStepUp(user.ID)
}

// handlePurged removes what auth-service still holds for an account the sweeper
// has just erased.
//
// The credentials row goes last, and that ordering is load-bearing: the pending
// tokens are found by the email address ON that row, so removing it first would
// strand them — a live password-reset link, addressed to a person who no longer
// exists, sitting in a table nobody would think to look in.
//
// THE NODES ROW IS NOT TOUCHED HERE. graph-grpc has already stripped it to a
// tombstone, and it stays forever: it is what reserves the handle so nobody can
// inherit a departed member's identity, and what lets the actor URI answer
// 410 Gone to the ActivityPub instances that cached them.
//
// Best-effort throughout and never returns an error. A nack would have the
// broker redeliver a purge whose content is already destroyed, and each retry
// would find less to do while logging the same failure.
func (h *accountDeletedHandler) handlePurged(personID int64) error {
	user, err := h.Repos.Users.ReadByID(personID)
	if err != nil || user == nil {
		// Expected for groups, which come through the same topic and have no
		// credentials, and for a redelivered purge that already succeeded.
		log.Printf("account purged: no credentials for actor %d, nothing to remove", personID)
		return nil
	}

	// Belt and braces. These went when the account was first marked deleted;
	// re-running is cheap and covers a deletion that predates that consumer.
	h.revokeCredentials(user.ID)

	if removed, err := h.Repos.Tokens.DeleteByRecipientEmail(user.Email); err != nil {
		log.Println("account purged: token removal failed:", err)
	} else if removed > 0 {
		log.Printf("account purged: removed %d pending token(s) (person=%d)", removed, user.ID)
	}

	// IP addresses, cities and device fingerprints — a record of where somebody
	// has been, which has no reason to outlive them.
	if removed, err := h.Repos.AuthLogs.DeleteByPersonID(user.ID); err != nil {
		log.Println("account purged: auth log removal failed:", err)
	} else if removed > 0 {
		log.Printf("account purged: removed %d auth log(s) (person=%d)", removed, user.ID)
	}

	if err := h.Repos.Users.Delete(user.ID); err != nil {
		log.Println("account purged: credential removal failed:", err)
		return nil
	}

	log.Printf("account purged: credentials removed (person=%d)", user.ID)

	return nil
}
