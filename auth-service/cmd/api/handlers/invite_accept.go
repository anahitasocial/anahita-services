package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"auth/data/entities"
	"auth/requests"

	"anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// ---------------------------------------------------------------------
// Invite-accept flow (unauthenticated, token-gated, server-rendered)
// ---------------------------------------------------------------------

// AcceptForm validates the invite token from the URL and renders the
// registration form. Invalid, expired, or already-used tokens get a
// friendly error page instead of the form.
//
// The form displays the invited email as read-only context — the user
// fills in their name and password.
// ToS/Privacy agreement is NOT collected here; that happens in the
// SPA after auto-login via the AgreementsPage interstitial gated by
// the SPA's AuthenticatedRoute.
//
//	GET /invites/accept?token=...
func (h *inviteHandler) AcceptForm(c echo.Context) error {
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
		"Token": token.Body,
		"Email": token.RecipientEmail,
	})
}

// Accept processes the invite-acceptance submission. On success:
//
//   - Creates the user record (registered type, no agreements yet)
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
	req := &requests.InviteAccept{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "invite_accept.html", map[string]any{
			"Token": req.Token,
			"Error": "Please fill in your name and a password of at least 15 characters.",
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
	exists, err := h.Repos.Users.ReadByEmail(token.RecipientEmail)
	if err != nil {
		log.Println(err)
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
		inviter.GivenName = "A team"
		inviter.FamilyName = "administrator"
	}

	user := entities.NewUser()
	user.Email = token.RecipientEmail
	user.Type = constants.PersonTypeRegistered
	user.Enabled = true
	user.GivenName = req.GivenName
	user.FamilyName = req.FamilyName
	user.CreationTime = time.Now()
	// ToS/Privacy agreement intentionally NOT set here — the SPA's
	// AuthenticatedRoute will bounce the user to AgreementsPage on
	// first sign-in to collect agreement (with the current version)
	// before granting access to the app.

	if err := user.SetPassword(req.Password); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "invite_accept.html", map[string]any{
			"Token": req.Token,
			"Email": token.RecipientEmail,
			"Error": "Could not create your account. Please try again.",
		})
	}

	user, err = h.Repos.Users.Add(user)
	if err != nil {
		log.Println(err)
		return h.renderTemplate(c, "invite_accept.html", map[string]any{
			"Token": req.Token,
			"Email": token.RecipientEmail,
			"Error": "Could not create your account. Please try again.",
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
	// and completes the OAuth flow silently. The SPA's
	// AuthenticatedRoute then sees no ToS/Privacy agreement and
	// routes to AgreementsPage.
	return c.Redirect(http.StatusSeeOther, h.Config.GetCompleteDomainName())
}
