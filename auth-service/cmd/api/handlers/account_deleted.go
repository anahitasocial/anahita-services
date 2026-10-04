package handlers

import (
	"log"
	"strings"

	an_constants "anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"
	"anahita-libs/event"

	"auth/constants"
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

	// Groups. Deleting one used to notify nobody at all — a group can have
	// several administrators, any one of them can delete it, and the others
	// would discover it by visiting a space that was gone.
	//
	// Handled here rather than in notification-service because this is the
	// only service that can both send mail and turn an actor id into an email
	// address. Giving notification-service a mail client would put one line
	// between every in-app notification and an email, which is the thing we
	// are deliberately not building.
	//
	// The routing key carries the entity: node.actor.group.group.deleted.v1.
	if parts[3] == "group" {
		switch parts[4] {
		case "deleted":
			h.notifyGroupAdmins(payload.ObjectID, false)
		case "restored":
			h.notifyGroupAdmins(payload.ObjectID, true)
		}
		return nil
	}

	if parts[4] != "deleted" {
		return nil
	}

	// ObjectID is the actor that was deleted. Anything without credentials
	// resolves to nothing here, which is expected and not an error worth
	// returning — returning one would nack the message and have the broker
	// redeliver it forever.
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

// systemViewer is the viewer used for auth-service's own internal reads.
//
// Not a person. This runs off a broker message with nobody signed in, and the
// read it authorises is for a group whose administrators are about to be told
// about it — the access check it would face as an anonymous viewer would fail
// on a group that has just been hidden.
func systemViewer() *lib_entities.Viewer {
	viewer := lib_entities.NewViewer()
	viewer.PersonType = an_constants.PersonTypeSuperAdmin
	return viewer
}

// notifyGroupAdmins mails every administrator of a group about its deletion or
// restoration.
//
// Never returns an error, and that is deliberate: this runs off a broker
// message, so an error nacks it and the group deletion is redelivered — which
// would re-send the mail on every redelivery. A notification that failed is
// worth a log line, not a retry loop aimed at somebody's inbox.
//
// Missing configuration is a no-op rather than a failure. GRAPH_SERVICE_ADDR
// is optional, and an installation that has not set it should lose this one
// notification quietly rather than log an error on every group deletion.
func (h *accountDeletedHandler) notifyGroupAdmins(groupID int64, restored bool) {
	if h.Srv.ActorAdminsGRPC == nil {
		return
	}

	admins, err := h.Srv.ActorAdminsGRPC.Browse(groupID)
	if err != nil {
		log.Printf("group lifecycle: cannot resolve administrators of %d: %v",
			groupID, err)
		return
	}
	if len(admins) == 0 {
		log.Printf("group lifecycle: group %d has no administrators to notify", groupID)
		return
	}

	// The group's own name and alias, for the subject line and the link.
	// "A group you administer was deleted" without saying which one is not a
	// usable notification.
	//
	// Read still resolves a group that has just been marked deleted — the soft
	// delete sets deleted_at and the read does not filter on it — which is
	// exactly the state the group is in when this runs.
	groupName, groupAlias := "", ""
	if h.Srv.ActorsGRPC != nil {
		if group, err := h.Srv.ActorsGRPC.Read(groupID, systemViewer()); err == nil && group != nil {
			groupName = group.Name
			groupAlias = group.Alias
		} else {
			log.Printf("group lifecycle: cannot read group %d: %v", groupID, err)
		}
	}
	if groupName == "" {
		// Rather than send a message with an empty pair of quotes in the
		// subject. Losing the name costs the notification most of its value,
		// and a group nobody can identify is not worth mailing about.
		log.Printf("group lifecycle: no name for group %d, not notifying", groupID)
		return
	}

	for _, admin := range admins {
		user, err := h.Repos.Users.ReadByID(admin.ID)
		if err != nil || user == nil {
			// An administrator with no credentials row: a deleted account that
			// still holds an admin edge. Skip rather than fail the rest.
			log.Printf("group lifecycle: no credentials for administrator %d",
				admin.ID)
			continue
		}
		if user.Email == "" {
			continue
		}

		notifyUser := *user
		go func() {
			var err error
			if restored {
				err = h.Notif.GroupLifecycle.EmailGroupRestored(
					&notifyUser, groupName, groupAlias)
			} else {
				err = h.Notif.GroupLifecycle.EmailGroupDeleted(
					&notifyUser, groupName, groupAlias)
			}
			if err != nil {
				log.Println("group lifecycle: notification failed:", err)
			}
		}()
	}
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
	authLogHandler := newAuthLogHandler(h.Env)
	if revoked, err := authLogHandler.DeleteAllSessions(user); err != nil {
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

	// Invitations this person sent that nobody took up. Each holds the invited
	// person's address, on behalf of somebody who is no longer here.
	if removed, err := h.Repos.Tokens.DeleteUnusedFrom(constants.TokenTypeInvite, user.ID); err != nil {
		log.Println("account purged: invitation removal failed:", err)
	} else if removed > 0 {
		log.Printf("account purged: removed %d unused invitation(s) (person=%d)", removed, user.ID)
	}

	// The request this account began as, where signing up needed approval. It
	// is kept after the decision and holds the address and a password hash.
	if removed, err := h.Repos.SignupRequests.DeleteFor(user.Email, user.Alias); err != nil {
		log.Println("account purged: signup request removal failed:", err)
	} else if removed > 0 {
		log.Printf("account purged: removed %d signup request(s) (person=%d)", removed, user.ID)
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
