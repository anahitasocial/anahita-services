package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"auth/data/entities"
	"auth/requests"

	"anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// suggestedUsername derives a username to PREFILL the form with, from the
// invited address.
//
// A suggestion, not an assignment. It used to be the actual username, never
// shown: two people invited as alice@ at different domains both got "alice",
// the second was refused with "could not create your account, please try
// again", and trying again derived the same name. A permanent failure with
// nothing the person could do about it, and no hint of what was wrong.
//
// It is also the account's address on the network, which is not something to
// choose for somebody without telling them. Now they see it, can change it,
// and are told whether it is free before they submit — while the common case
// stays one glance and a click.
var inviteAliasStrip = regexp.MustCompile(`[^a-z0-9_-]+`)

func suggestedUsername(email string) string {
	local, _, _ := strings.Cut(email, "@")
	return inviteAliasStrip.ReplaceAllString(strings.ToLower(local), "")
}

// ---------------------------------------------------------------------
// Invite-accept flow (unauthenticated, token-gated, server-rendered)
// ---------------------------------------------------------------------

// AcceptForm validates the invite token from the URL and renders the
// registration form. Invalid, expired, or already-used tokens get a
// friendly error page instead of the form.
//
// The form displays the invited email as read-only context — the user
// fills in their name and password, and accepts the terms.
//
// The versions in force ride as hidden fields so the submission records what
// the person was actually shown. Accept refuses a submission whose versions
// have moved on rather than recording an acceptance of text nobody read.
//
//	GET /invites/accept?token=...
func (h *inviteHandler) AcceptForm(c echo.Context) error {
	// `closed` suspends outstanding invitations, which is the one thing it
	// does that `invite` does not.
	//
	// Checked on BOTH the form and the submission. Checking only the form
	// would leave a page somebody already had open as a working way in, which
	// is precisely the case a lockdown is for.
	//
	// The token is untouched — not consumed, not marked — so lifting the
	// lockdown brings the invitation back rather than having quietly
	// destroyed it while nobody was looking.
	if !h.Config.AllowsInvitations() {
		return h.renderError(c, "Invitations Are Paused",
			"This site is not accepting new members right now. Your invitation "+
				"has not been used, so it will still work once invitations "+
				"resume. Please contact whoever invited you.")
	}

	tokenBody := c.QueryParam("token")
	if tokenBody == "" {
		return h.renderError(c, "Invalid Link",
			"This invite link is not valid. Please contact the person who invited you for a new one.")
	}

	token, err := h.Repos.Tokens.ReadByBody(tokenBody)
	if err != nil {
		log.Println(err)
		return h.renderError(c, "Invalid Link",
			"This invite link is not valid. Please contact the person who invited you for a new one.")
	}

	if token.Type != constants.TokenTypeInvite {
		return h.renderError(c, "Invalid Link",
			"This link is not valid for accepting an invitation.")
	}

	switch token.Status {
	case constants.TokenStatusUsed:
		return h.renderError(c, "Invitation Already Accepted",
			"This invitation has already been accepted. Try signing in instead, or contact your administrator if you can't access your account.")
	case constants.TokenStatusExpired:
		return h.renderError(c, "Invitation Expired",
			"This invitation has expired. Please ask the person who invited you to send a new one.")
	}

	return h.renderTemplate(c, "invite_accept.html",
		h.inviteAcceptContext(token.Body, token.RecipientEmail, "", ""))
}

// inviteAcceptContext builds the form's context in one place, so a re-render
// after an error cannot quietly lose a field the form needs.
//
// That has happened here before: a re-render that dropped the agreement
// versions produced a form whose hidden fields were empty, and the next
// submission failed for a reason the person could not see. Every render goes
// through this now.
func (h *inviteHandler) inviteAcceptContext(
	token, email, username, errMsg string,
) map[string]any {
	// The suggestion is only offered when it is actually free. Prefilling a
	// username that is already taken invites somebody to submit it and be
	// refused, which is the failure this replaced.
	if username == "" {
		suggestion := suggestedUsername(email)
		if len(suggestion) >= 3 {
			if taken, err := h.Repos.Users.IsUsername(suggestion); err == nil && !taken {
				username = suggestion
			}
		}
	}

	return map[string]any{
		"Token":    token,
		"Email":    email,
		"Username": username,
		"Error":    errMsg,
	}
}

// inviteValidationMessage says which field is wrong, for the same reason
// signupValidationMessage does: the shared validator has flattened the typed
// error to a string by the time a handler sees it, so there is nothing left to
// ask which field failed.
func inviteValidationMessage(req *requests.InviteAccept) string {
	username := strings.TrimSpace(req.Username)

	switch {
	case username == "":
		return "Please choose a username."
	case len(username) < 3:
		return "That username is too short — it needs at least 3 characters."
	case len(username) > 32:
		return "That username is too long — 32 characters at most."
	case !usernameCharacters.MatchString(username):
		return "A username can contain letters, numbers, hyphens and " +
			"underscores, and nothing else."
	}

	if len(req.Name) > 100 {
		return "That display name is too long — 100 characters at most."
	}

	if len(req.Password) < 15 {
		return "Your password needs to be at least 15 characters long."
	}
	if len(req.Password) > 150 {
		return "That password is too long — 150 characters at most."
	}

	return "Please check the form and try again."
}

// Accept processes the invite-acceptance submission. On success:
//
//   - Creates the user record (registered type, terms accepted)
//   - Marks the token used
//   - Builds and persists the IdP session (auto-login)
//   - Fires accept notifications/emails to org admins
//   - Redirects to the BFF, which completes the OAuth flow silently
//     using the freshly-minted IdP session
//
// Auto-login IS performed here, in contrast to password reset. The
// TOTP-bypass concern that drove password-reset to skip auto-login
// doesn't apply: a brand-new user has no TOTP enrolled yet, and they
// just demonstrated email control plus password knowledge in one
// step.
//
//	POST /invites/accept
func (h *inviteHandler) Accept(c echo.Context) error {
	// `closed` suspends outstanding invitations, which is the one thing it
	// does that `invite` does not.
	//
	// Checked on BOTH the form and the submission. Checking only the form
	// would leave a page somebody already had open as a working way in, which
	// is precisely the case a lockdown is for.
	//
	// The token is untouched — not consumed, not marked — so lifting the
	// lockdown brings the invitation back rather than having quietly
	// destroyed it while nobody was looking.
	if !h.Config.AllowsInvitations() {
		return h.renderError(c, "Invitations Are Paused",
			"This site is not accepting new members right now. Your invitation "+
				"has not been used, so it will still work once invitations "+
				"resume. Please contact whoever invited you.")
	}

	req := &requests.InviteAccept{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "invite_accept.html",
			h.inviteAcceptContext(req.Token, "", req.Username, inviteValidationMessage(req)))
	}

	token, err := h.Repos.Tokens.ReadByBody(req.Token)
	if err != nil {
		log.Println(err)
		return h.renderError(c, "Invalid Link",
			"This invite link is not valid. Please contact the person who invited you for a new one.")
	}

	if token.Type != constants.TokenTypeInvite {
		return h.renderError(c, "Invalid Link",
			"This link is not valid for accepting an invitation.")
	}

	if token.Status != constants.TokenStatusPending {
		return h.renderError(c, "Invitation No Longer Valid",
			"This invitation has been used or expired. Please ask the person who invited you to send a new one.")
	}

	// Race check: did someone else create an account with this email
	// between invite creation and acceptance? If so, refuse rather
	// than overwriting.
	// sql.ErrNoRows is the ANSWER here, not a failure: it means nobody holds
	// this address, which is the whole point of checking.
	//
	// Treating every error as fatal meant this branch fired on every
	// invitation, so accepting one has always ended on "Something Went Wrong"
	// — an error page for the successful case. Only a real fault stops it now.
	exists, err := h.Repos.Users.ReadByEmail(token.RecipientEmail)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("invite accept: existing-account check failed:", err)
		return h.renderError(c, "Something Went Wrong",
			"We couldn't verify your account right now. Please try again in a moment.")
	}
	if exists != nil {
		return h.renderError(c, "Account Already Exists",
			"An account already exists for this email address. Try signing in instead.")
	}

	// Fetch the inviter now so we can name them in the admin
	// broadcast email. If this fails, log and proceed with a stub —
	// the inviter's name in the email is nice-to-have, not critical
	// to the acceptance flow.
	inviter, err := h.Repos.Users.ReadByID(token.CreatedBy)
	if err != nil || inviter == nil {
		log.Printf("invite accept: could not load inviter (person=%d): %v", token.CreatedBy, err)
		inviter = entities.NewUser()
		inviter.Name = "A team administrator"
	}

	// Whose invitation was it? That decides whether this person joins now or
	// joins a queue.
	//
	// An ADMINISTRATOR's invitation is already the decision an approver would
	// be making — asking them to clear it from a list afterwards is asking the
	// same person to approve the same person twice. A MEMBER's is a
	// recommendation: they are vouching, not admitting, and under `approval`
	// an administrator still decides.
	//
	// Only reachable at all when INVITES_FROM allows members; otherwise every
	// invitation is an administrator's by construction.
	if h.Config.RequiresRegistrationApproval() && !inviter.IsAdminOrSuperAdmin() {
		return h.acceptIntoQueue(c, req, token, inviter)
	}

	user := entities.NewUser()
	user.Email = token.RecipientEmail
	// PersonType, not Type. `Type` is the NODE type — it wants
	// node.actor.person-service.person.v1 — and this assigned "registered" to
	// it, which Users.Add then silently overwrote. The effect was that an
	// invited person got an EMPTY usertype: not guest, not registered, so
	// every IsRegistered/IsGuest check answered false and the account had no
	// permissions at all.
	user.PersonType = constants.PersonTypeRegistered
	user.Enabled = true
	user.CreatedAt = time.Now()
	// Verified on arrival. This person reached the form by following a link
	// sent to that address and nowhere else, which is exactly what a
	// confirmation email asks them to do — asking again would be asking them
	// to prove the same thing twice.
	user.MarkEmailVerified()
	// Who invited them, for onboarding to show first. From the token and NOT
	// from inviter above: when that lookup fails it falls back to a stub with
	// id 0, which would record "nobody" for somebody who plainly was invited.
	user.InvitedBy = token.CreatedBy

	// Checked before anything is created, so the message can name the field.
	// The graph enforces it too — the check below catches the race — but a
	// duplicate-key error surfaced raw is what used to make this a dead end.
	if taken, err := h.Repos.Users.IsUsername(req.Username); err != nil {
		log.Println("invite accept: username check failed:", err)
		return h.renderTemplate(c, "invite_accept.html",
			h.inviteAcceptContext(req.Token, token.RecipientEmail, req.Username,
				"Could not create your account. Please try again."))
	} else if taken {
		return h.renderTemplate(c, "invite_accept.html",
			h.inviteAcceptContext(req.Token, token.RecipientEmail, req.Username,
				"That username is already taken."))
	}

	// No agreement capture. Acceptance is collected after signing in and
	// checked on every sign-in, so a revision re-prompts by itself — see the
	// note on InviteAccept. The form shows the terms and links them.

	if err := user.SetPassword(req.Password); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "invite_accept.html",
			h.inviteAcceptContext(req.Token, token.RecipientEmail, req.Username, "Could not create your account. Please try again."))
	}

	// The actor node first, through the graph — see signup.go for why this is
	// no longer a direct insert.
	//
	// The handle comes from the email local part, since invite-accept collects
	// a name and a password but never a username.
	actor := libs_entities.NewAnahitaNode()
	actor.Type = constants.NodeActorPersonEntityType
	actor.Service = constants.PersonService
	actor.Alias = strings.ToLower(strings.TrimSpace(req.Username))
	actor.Name = strings.TrimSpace(req.Name)
	actor.Access = constants.AnahitaAccessPublic
	actor.PersonType = constants.PersonTypeRegistered
	actor.Enabled = true
	// See signup.go: NewAnahitaNode leaves these at Go's zero time, which a
	// MySQL DATETIME cannot store.
	actor.CreatedAt = time.Now()
	actor.UpdatedAt = time.Now()

	createdActor, err := h.Srv.ActorsGRPC.Add(actor, signupViewer())
	if err != nil {
		log.Println("invite accept: could not create the actor:", err)

		// Somebody took the username between the check above and this call.
		// Rare, and the only honest thing to say about it is the same thing
		// the check says.
		message := "Could not create your account. Please try again."
		if isDuplicateHandle(err) {
			message = "That username is already taken."
		}

		return h.renderTemplate(c, "invite_accept.html",
			h.inviteAcceptContext(req.Token, token.RecipientEmail, req.Username, message))
	}

	user.ID = createdActor.ID
	user.Alias = createdActor.Alias
	// See signup.go: Go's zero time is '0000-00-00' to MySQL, which strict mode
	// rejects.
	user.UpdatedAt = time.Now()

	user, err = h.Repos.Users.Add(user)
	if err != nil {
		log.Println(err)
		// Compensating delete, as in signup: two writes across two services
		// with no transaction between them, so the node has to be unwound or
		// it holds a handle nobody can sign into.
		if purgeErr := h.purgeOrphanedActor(createdActor.ID); purgeErr != nil {
			log.Printf("invite accept: orphaned actor %d: %v (purge also failed: %v)", createdActor.ID, err, purgeErr)
		}
		return h.renderTemplate(c, "invite_accept.html",
			h.inviteAcceptContext(req.Token, token.RecipientEmail, req.Username, "Could not create your account. Please try again."))
	}

	// Mark the token used. If this fails after the person was
	// created, log loudly — the person exists but the token could
	// theoretically be reused. The status check on subsequent
	// attempts won't help here because the token is still pending;
	// the IsEmail check on next attempt will catch it though.
	token.IsUsed = true
	token.UpdatedAt = time.Now()
	token.UpdatedBy = user.ID
	if _, err := h.Repos.Tokens.Edit(token); err != nil {
		log.Printf("invite accept: failed to consume token after person creation (person=%d token=%d): %v",
			user.ID, token.ID, err)
	}

	// Settle every OTHER pending invitation to this address.
	//
	// Several people may invite the same person — nothing forbids it,
	// and the per-recipient rate limit meters it rather than blocking
	// it. Only the link that was clicked is consumed above, so without
	// this the rest sit as "waiting" in their senders' lists forever:
	// for somebody who has already joined, behind links that now fail
	// because the address has an account. Each sender sees them settle
	// as accepted, which is what happened.
	//
	// After the accepted token, not before. If this runs first and the
	// account creation then fails, the invitations are spent and
	// nobody can retry.
	//
	// Logged rather than fatal: the person has an account and is about
	// to be signed in, and failing the request here would report that
	// as an error. The cost of a missed sweep is a stale row, which
	// expires on its own.
	if _, err := h.Repos.Tokens.InvalidateOutstanding(
		constants.TokenTypeInvite, token.RecipientEmail,
	); err != nil {
		log.Printf("invite accept: could not settle sibling invitations for %s: %v",
			token.RecipientEmail, err)
	}

	// Auto-login: build the viewer and persist it to the IdP session.
	// Brand-new user, no TOTP, so it's safe to mint the session
	// directly without going through the TOTP step.
	//
	// TODO: extract this into a shared establishIdPSession helper
	// alongside readIdPViewer. Three callers now want this logic
	// (login finalizeLogin, the old signup register, and Accept).
	viewer := libs_entities.NewViewer()
	viewer.ID = user.ID
	viewer.Email = user.Email
	viewer.Type = user.Type
	viewer.Authenticated = true

	viewerJSON, err := json.Marshal(viewer)
	if err != nil {
		log.Println(err)
		return h.renderError(c, "Something Went Wrong",
			"Your account was created but we couldn't sign you in automatically. Please try signing in manually.")
	}

	if _, err := h.Session.Add(c, constants.UserSessionKey, string(viewerJSON)); err != nil {
		log.Println(err)
		return h.renderError(c, "Something Went Wrong",
			"Your account was created but we couldn't sign you in automatically. Please try signing in manually.")
	}

	// Fire-and-forget notifications.
	//
	// NotifyAccept publishes the in-app event; the notification-service
	// owns fan-out to admin dashboards from there.
	//
	// EmailAccept broadcasts to every admin and super-admin in the
	// org, naming both the joiner and the inviter. Sending failures
	// for individual admins do not stop the rest of the broadcast.
	go func() {
		if err := h.Notif.Invite.NotifyAccept(user); err != nil {
			log.Println(err)
		}
	}()
	go func() {
		if err := h.Notif.Invite.EmailAccept(user, inviter); err != nil {
			log.Println(err)
		}
	}()

	// Redirect to the BFF root. The BFF sees no oauth_tokens cookie,
	// initiates /oauth/authorize, finds the IdP session we just set,
	// and completes the OAuth flow silently. The terms were accepted on
	// the form above, so there is nothing left to collect before the app.
	return c.Redirect(http.StatusSeeOther, h.Config.GetFrontendURL())
}

// acceptIntoQueue turns an accepted member invitation into a signup request.
//
// Under `approval`, a member vouching for somebody is a recommendation rather
// than an admission — so the invitation gets them as far as the queue, with
// the voucher's name attached, and an administrator decides from there.
//
// A vouched request is a better thing to read than a cold one, and that is the
// reason to allow member invitations at all: the member does the finding, the
// administrator keeps the decision.
func (h *inviteHandler) acceptIntoQueue(
	c echo.Context,
	req *requests.InviteAccept,
	token *entities.Token,
	inviter *entities.User,
) error {
	sr := entities.NewSignupRequest()
	sr.Username = strings.ToLower(strings.TrimSpace(req.Username))
	sr.Email = token.RecipientEmail
	sr.Name = strings.TrimSpace(req.Name)
	// From the token, for the reason Accept gives: the inviter passed in may
	// be the id-0 stub a failed lookup falls back to, and approval copies this
	// onto the account.
	sr.InvitedBy = token.CreatedBy

	// Verified on arrival, and straight into the queue.
	//
	// They reached this form through a link sent to that address and nowhere
	// else, which is exactly what a confirmation email asks somebody to prove.
	// A cold request has to confirm first; this one already has.
	now := time.Now()
	sr.EmailVerifiedAt = &now
	sr.Status = entities.SignupRequestStatusPending

	source := newConsentSource(c)
	sr.CreatedIP = source.ip
	sr.CreatedUserAgent = source.userAgent

	if err := sr.SetPassword(req.Password); err != nil {
		log.Println("invite accept: could not hash the signup request password:", err)
		return h.renderTemplate(c, "invite_accept.html",
			h.inviteAcceptContext(req.Token, token.RecipientEmail, req.Username,
				"Could not send your request. Please try again."))
	}

	stored, err := h.Repos.SignupRequests.Add(sr)
	if err != nil {
		log.Println("invite accept: could not store the signup request:", err)
		return h.renderTemplate(c, "invite_accept.html",
			h.inviteAcceptContext(req.Token, token.RecipientEmail, req.Username,
				"Could not send your request. Please try again."))
	}

	// The invitation is spent either way. It brought them here, and leaving it
	// live would let one invitation produce a second request after this one is
	// decided.
	token.IsUsed = true
	token.UpdatedAt = time.Now()
	if _, err := h.Repos.Tokens.Edit(token); err != nil {
		log.Printf("invite accept: could not consume token %d: %v", token.ID, err)
	}

	notifyRequest := *stored
	go func() {
		if err := h.Notif.Signup.EmailSignupRequestReceived(&notifyRequest); err != nil {
			log.Println("invite accept: notifying administrators failed:", err)
		}
	}()

	return h.renderTemplate(c, "signup_request_sent.html", map[string]any{
		"Email":    stored.Email,
		"Verified": true,
	})
}
