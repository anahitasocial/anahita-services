package handlers

import (
	"log"
	"net/http"
	"strings"

	"auth/requests"

	"github.com/labstack/echo/v4"
)

type usernameChangeHandler struct {
	*Env
}

func newUsernameChangeHandler(env *Env) *usernameChangeHandler {
	return &usernameChangeHandler{env}
}

// ---------------------------------------------------------------------
// Username change — the only way an account's handle moves.
//
// ONE STEP, unlike the email change beside it, and the difference is
// deliberate rather than a corner cut.
//
// Email change needs the old inbox to approve because the change moves
// where password reset delivers: without that approval, anyone inside a
// live session could repoint recovery at themselves and own the account
// permanently. A username change moves no recovery path — reset still
// goes to the same address — so there is nothing here for the mailbox
// to approve. Sending a confirmation link would be friction guarding a
// threat that does not exist.
//
// What the handle IS worth guarding is real, though, which is why this
// is step-up gated rather than open:
//
//   - it is a sign-in identifier, resolved by ReadByEmailOrUsername
//   - it is the profile slug (nodes.alias), so it is how people are
//     recognised and linked to
//
// Passkeys and sessions are NOT revoked, unlike the email flow. Nothing
// here changes who can recover the account, so revoking would be
// disproportionate — the same trap the password change flow avoids by
// leaving passkeys alone. The notification email is the tripwire.
// ---------------------------------------------------------------------

// Edit changes the username.
//
//	PATCH /username/change
//
// Status codes:
//
//	200 — changed, with the stored handle
//	204 — the requested handle is the one already held; nothing done
//	400 — bad body, or the handle fails format validation
//	403 — no recent step-up; the client re-authenticates and retries
//	409 — the handle is taken
//	500 — database failure
//
// 409 is safe to be specific about here for the same reason it is on the
// email change: the caller has already proved a session AND a step-up,
// so this is not an enumeration oracle. It could not be one anyway —
// username availability is public by construction, since a profile URL
// either resolves or does not.
//
// No rate limit scope. clearStepUp runs on success, so each change costs
// a fresh re-authentication, and a rejected attempt reveals only what
// visiting a profile URL already reveals.
func (h *usernameChangeHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &requests.UsernameChangeEdit{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Trimmed, not lowercased. The stored handle keeps the case the
	// person chose — it is displayed — while every comparison below is
	// case-insensitive, so "Rastin" and "rastin" can never both exist.
	newUsername := strings.TrimSpace(req.Username)

	// Re-read rather than trusting the cached viewer: the access token
	// carries no profile data, so viewer.Username is empty here.
	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.requireStepUp(user.ID); err != nil {
		log.Println("username change:", err)
		return c.NoContent(reauthStatus(err))
	}

	// Asking for the handle you already hold is a no-op, not an error.
	//
	// Case-insensitive, so this also catches a request that differs only
	// in case — which IsUsername below would otherwise report as taken
	// (by the caller's own row), turning a harmless re-submission into a
	// 409. Note the consequence: this flow cannot be used to restyle
	// your own handle's capitalisation. That is a fair trade for not
	// having to special-case the caller inside the availability query.
	oldUsername := user.Username
	if strings.EqualFold(newUsername, oldUsername) {
		return c.NoContent(http.StatusNoContent)
	}

	isTaken, err := h.Repos.Users.IsUsername(newUsername)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if isTaken {
		return c.NoContent(http.StatusConflict)
	}

	// The handle first, through the graph, then the credential row.
	//
	// These are the same value — the sign-in identifier and the profile slug —
	// and must not diverge, but they now live in two services and cannot share
	// a transaction. The graph goes first because it owns the actor namespace
	// and is the one that can refuse: a handle already taken is rejected there,
	// and nothing has been written yet when it is.
	//
	// The reverse order would write the credential row, have the graph refuse,
	// and leave the person signing in under a name their profile does not use.
	actor, err := h.Srv.ActorsGRPC.Read(user.ID, systemViewer())
	if err != nil || actor == nil {
		log.Println("username change: could not read the actor:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	actor.Alias = newUsername
	if _, err := h.Srv.ActorsGRPC.Edit(actor, systemViewer()); err != nil {
		// The namespace rejected it — almost always because another actor
		// already holds the handle, which is the caller's 409, not a 500.
		log.Println("username change: the graph refused the handle:", err)
		return c.NoContent(http.StatusConflict)
	}

	if err := h.Repos.Users.EditUsername(newUsername, user.ID); err != nil {
		log.Println("username change: update failed:", err)
		// The handle moved but the credential row did not: put the alias back,
		// or the person signs in under a name their profile no longer uses.
		actor.Alias = user.Username
		if _, revertErr := h.Srv.ActorsGRPC.Edit(actor, systemViewer()); revertErr != nil {
			log.Printf("username change: actor %d left on handle %q while credentials hold %q: %v",
				user.ID, newUsername, user.Username, revertErr)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	user.Username = newUsername

	// One step-up authorises one credential change.
	h.clearStepUp(user.ID)

	log.Printf("username change: %q -> %q (person=%d)", oldUsername, newUsername, user.ID)

	// Fire-and-forget notification, last, so it describes work that has
	// already happened. This is the only way an impersonation attempt
	// surfaces, since nothing else about the account visibly changes.
	go func() {
		if err := h.Notif.UsernameChange.EmailChanged(user, oldUsername); err != nil {
			log.Println(err)
		}
	}()

	// The handle back, so the SPA can update the store and the profile
	// URL it renders without a second round trip. Returns what was
	// STORED rather than what was sent — they differ by the trim.
	return c.JSON(http.StatusOK, map[string]string{
		"username": user.Username,
	})
}
