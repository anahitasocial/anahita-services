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

// inviteAlias derives an actor handle from the invited address.
//
// Invite-accept collects a name and a password but never a username, while
// every actor needs a handle — so it comes from the email local part, lowercased
// and stripped of anything that is not a handle character.
//
// Not unique on its own: two invitations to alice@a.com and alice@b.com both
// want "alice". The graph's actor namespace rejects the second, and the caller
// surfaces that rather than guessing a suffix — a handle is a person's name on
// the network and should not be silently chosen for them.
var inviteAliasStrip = regexp.MustCompile(`[^a-z0-9_-]+`)

func inviteAlias(email string) string {
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

	return h.renderTemplate(c, "invite_accept.html", map[string]any{
		"Token":          token.Body,
		"Email":          token.RecipientEmail,
		"TosVersion":     h.Config.GetTosVersion(),
		"PrivacyVersion": h.Config.GetPrivacyVersion(),
	})
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
		return h.renderTemplate(c, "invite_accept.html", map[string]any{
			"Token":          req.Token,
			"Error":          "Please fill in your name, choose a password of at least 15 characters, and accept the Terms of Service and Privacy Policy.",
			"TosVersion":     h.Config.GetTosVersion(),
			"PrivacyVersion": h.Config.GetPrivacyVersion(),
		})
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
	user.CreationTime = time.Now()

	// Agreement is captured HERE, at account creation.
	//
	// It used to say it happened in the SPA, on an AgreementsPage gated by
	// AuthenticatedRoute. That component was never written and there was no
	// column to write to, so acceptance was recorded nowhere at all.
	if err := h.recordAgreements(c, user, req.TosVersion, req.PpVersion); err != nil {
		return h.renderTemplate(c, "invite_accept.html", map[string]any{
			"Token":          req.Token,
			"Email":          token.RecipientEmail,
			"TosVersion":     h.Config.GetTosVersion(),
			"PrivacyVersion": h.Config.GetPrivacyVersion(),
			"Error":          "The terms have been updated since this page was opened. Please read them again.",
		})
	}

	if err := user.SetPassword(req.Password); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "invite_accept.html", map[string]any{
			"Token":          req.Token,
			"Email":          token.RecipientEmail,
			"Error":          "Could not create your account. Please try again.",
			"TosVersion":     h.Config.GetTosVersion(),
			"PrivacyVersion": h.Config.GetPrivacyVersion(),
		})
	}

	// The actor node first, through the graph — see signup.go for why this is
	// no longer a direct insert.
	//
	// The handle comes from the email local part, since invite-accept collects
	// a name and a password but never a username.
	actor := libs_entities.NewAnahitaNode()
	actor.Type = constants.NodeActorPersonEntityType
	actor.Service = constants.PersonService
	actor.Alias = inviteAlias(token.RecipientEmail)
	actor.Name = strings.TrimSpace(req.Name)
	actor.Access = constants.AnahitaAccessPublic
	actor.PersonType = constants.PersonTypeRegistered
	actor.Enabled = true
	// See signup.go: NewAnahitaNode leaves these at Go's zero time, which a
	// MySQL DATETIME cannot store.
	actor.CreationTime = time.Now()
	actor.UpdateTime = time.Now()

	createdActor, err := h.Srv.ActorsGRPC.Add(actor, signupViewer())
	if err != nil {
		log.Println("invite accept: could not create the actor:", err)
		return h.renderTemplate(c, "invite_accept.html", map[string]any{
			"Token":          req.Token,
			"Email":          token.RecipientEmail,
			"Error":          "Could not create your account. Please try again.",
			"TosVersion":     h.Config.GetTosVersion(),
			"PrivacyVersion": h.Config.GetPrivacyVersion(),
		})
	}

	user.ID = createdActor.ID
	user.Alias = createdActor.Alias
	// See signup.go: Go's zero time is '0000-00-00' to MySQL, which strict mode
	// rejects.
	user.UpdateTime = time.Now()

	user, err = h.Repos.Users.Add(user)
	if err != nil {
		log.Println(err)
		// Compensating delete, as in signup: two writes across two services
		// with no transaction between them, so the node has to be unwound or
		// it holds a handle nobody can sign into.
		if _, purgeErr := h.Srv.ActorsGRPC.Purge(createdActor.ID, signupViewer()); purgeErr != nil {
			log.Printf("invite accept: orphaned actor %d: %v (purge also failed: %v)", createdActor.ID, err, purgeErr)
		}
		return h.renderTemplate(c, "invite_accept.html", map[string]any{
			"Token":          req.Token,
			"Email":          token.RecipientEmail,
			"Error":          "Could not create your account. Please try again.",
			"TosVersion":     h.Config.GetTosVersion(),
			"PrivacyVersion": h.Config.GetPrivacyVersion(),
		})
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
