package handlers

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"

	"auth/data/entities"
	"auth/requests"

	"github.com/labstack/echo/v4"
)

// isDuplicateHandle reports whether the graph refused an actor because its
// alias is already taken.
//
// gRPC flattens the driver error to a string, so this matches the unique index
// by name — uq_unique_alias is the actor handle namespace, and nothing else on
// the nodes table carries a unique constraint.
func isDuplicateHandle(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "uq_unique_alias") || strings.Contains(msg, "duplicate entry")
}

// signupViewer is the viewer a public signup acts as.
//
// Signup has no authenticated caller by definition, and ActorService.Add does
// not gate on the viewer — it uses it to attribute the event it publishes. A
// guest viewer says exactly that, rather than borrowing super-administrator
// authority for a request that has none.
func signupViewer() *lib_entities.Viewer {
	return lib_entities.NewViewer()
}

type signupHandler struct {
	*Env
}

// newSignupHandler creates a new Signup handler
func newSignupHandler(env *Env) *signupHandler {
	return &signupHandler{
		env,
	}
}

// ---------------------------------------------------------------------
// Signup (unauthenticated, server-rendered)
// ---------------------------------------------------------------------
//
// Server-rendered, beside the other credential flows, rather than a JSON
// endpoint the web app posts to. The JSON endpoint is gone with it.
//
// Four reasons, in the order they matter:
//
//   - REACT_APP_SIGNUP_CLOSED hid a tab while POST /signups stayed open. A
//     client cannot enforce whether a site takes registrations.
//   - The password stops passing through the web app's dependency tree. These
//     pages have no npm packages behind them; the app has hundreds.
//   - Honeypot, submission timing and a CSRF-bound form all want a server
//     page. In a React bundle the attacker reads the challenge.
//   - Signing up can end signed in, in one step, because the IdP session is
//     written here. It used to sign you up and then ask you to sign in.

const (
	sessionKeySignupCSRF = "signup_csrf_token"

	// sessionKeySignupRendered is when the form was handed out, so the
	// submission can be timed.
	sessionKeySignupRendered = "signup_rendered_at"
)

// minimumSignupSeconds is how quickly a submission is assumed to be scripted.
//
// Two, not the five or ten often suggested. Somebody with a password manager
// clicks the username field, has all three filled for them, and submits — that
// is a real person and it can take under three seconds. A threshold that
// catches them is worse than one a fast bot slips under, because the bot will
// simply wait and the person cannot.
//
// It is a floor on machine speed, not a measure of human care.
const minimumSignupSeconds = 2

// honeypotField is a field no person can see and no person should fill.
//
// Named for something a naive bot expects on a signup form and fills because
// it is there. Deliberately NOT "website" or "url", which password managers
// and browser autofill will happily complete on a real person's behalf — a
// honeypot that catches the people using a password manager is a trap for the
// wrong species.
const honeypotField = "company_role"

// Form renders the signup page.
//
//	GET /signup
func (h *signupHandler) Form(c echo.Context) error {
	if refusal := h.registrationRefusal(c); refusal != nil {
		return refusal
	}

	token, err := generateCSRFToken()
	if err != nil {
		log.Println("signup: could not generate a CSRF token:", err)
		return h.renderError(c, "Something Went Wrong",
			"We could not start the sign-up form. Please try again.")
	}

	if _, err := h.Session.Add(c, sessionKeySignupCSRF, token); err != nil {
		log.Println("signup: could not store the CSRF token:", err)
		return h.renderError(c, "Something Went Wrong",
			"We could not start the sign-up form. Please try again.")
	}

	// When the form was handed out, kept server-side.
	//
	// Not a hidden field: a timestamp the client holds is a timestamp the
	// client chooses, and a bot would simply send one from two seconds ago.
	// The session already exists for the CSRF token, so this costs nothing.
	if _, err := h.Session.Add(c, sessionKeySignupRendered,
		strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		log.Println("signup: could not store the render time:", err)
	}

	return h.renderTemplate(c, "signup.html", map[string]any{
		"CSRFToken":     token,
		"HoneypotField": honeypotField,
	})
}

// looksAutomated reports whether a submission was not made by a person at the
// form, and says which signal fired.
//
// Two signals, both free and neither of them a puzzle: a field no person can
// see, and a submission faster than a person can type. Together they stop the
// naive majority; neither pretends to stop a determined one, which is what the
// registration modes are for.
func (h *signupHandler) looksAutomated(c echo.Context) (bool, string) {
	// The honeypot. Anything at all in it, including whitespace.
	if value := strings.TrimSpace(c.FormValue(honeypotField)); value != "" {
		return true, "honeypot"
	}

	renderedAt, err := h.Session.Read(c, sessionKeySignupRendered)
	if err != nil || renderedAt == "" {
		// No render time means the session was lost or the POST never went
		// through the form. Not treated as automated on its own: the CSRF
		// check already refuses a submission with no session behind it, and
		// double-refusing here would turn an expired cookie into a silent
		// failure.
		return false, ""
	}

	seconds, err := strconv.ParseInt(renderedAt, 10, 64)
	if err != nil {
		return false, ""
	}

	if time.Now().Unix()-seconds < minimumSignupSeconds {
		return true, "too fast"
	}

	return false, ""
}

// registrationRefusal returns a rendered refusal when this installation does
// not take self-serve registrations, and nil when it does.
//
// Checked on the form AND on the submission. A page somebody already had open
// is otherwise a working way in after the mode changes, which is the case a
// lockdown exists for.
func (h *signupHandler) registrationRefusal(c echo.Context) error {
	if h.Config.AllowsSelfServeRegistration() {
		return nil
	}

	// What to say depends on whether an invitation would still work, and the
	// difference matters to the reader: one is "find somebody to invite you",
	// the other is "there is nothing you can do today".
	if h.Config.AllowsInvitations() {
		return h.renderError(c, "Invitation Only",
			"This site is not open for registration. You need an invitation "+
				"from an existing member to join.")
	}

	return h.renderError(c, "Registration Closed",
		"This site is not accepting new members right now.")
}

// Add processes the signup form.
//
//	POST /signup
func (h *signupHandler) Add(c echo.Context) error {
	if refusal := h.registrationRefusal(c); refusal != nil {
		return refusal
	}

	viewer := getViewer(c)

	if ok, err := h.Perms.Signup.CanAdd(viewer, nil); !ok || err != nil {
		message := "You cannot sign up."
		if err != nil {
			message = err.Error()
		}
		return h.renderError(c, "Cannot Sign Up", message)
	}

	req := requests.NewSignupRequest().Add
	if err := c.Bind(req); err != nil {
		log.Println("signup:", err)
		return h.renderSignupError(c, "Please check the form and try again.")
	}

	// CSRF before validation, and before anything is read from the form.
	//
	// Consumed either way — a token is good for one submission. Leaving it
	// live after a failed attempt would let one stolen token be replayed for
	// as long as the session lasts.
	stored, err := h.Session.Read(c, sessionKeySignupCSRF)
	_ = h.Session.Delete(c, sessionKeySignupCSRF)

	if err != nil || stored == "" || subtle.ConstantTimeCompare(
		[]byte(stored), []byte(req.CSRFToken),
	) != 1 {
		log.Println("signup: CSRF token did not match")
		return h.renderError(c, "Session Expired",
			"This form has expired. Please start again.")
	}

	// The two automated signals, after CSRF and before anything is created.
	//
	// They answer DIFFERENTLY, and that is deliberate — their false positives
	// are not comparable.
	//
	// The honeypot answers with success and creates nothing. No person can
	// fill a field they cannot see, so a false positive is close to
	// impossible, and telling a bot which field gave it away just gets the
	// field skipped next time.
	//
	// The timing check answers with an error a person can act on. A false
	// positive IS possible here — somebody with a password manager is fast —
	// and answering with a silent success would leave a real person waiting
	// for an email that is never coming, with nothing on screen to suggest
	// anything went wrong. A bot learns that a floor exists and pays another
	// round trip; that is a much cheaper leak than losing a member.
	if automated, signal := h.looksAutomated(c); automated {
		log.Printf("signup: refused as automated (%s)", signal)

		if signal == "honeypot" {
			return h.renderTemplate(c, "signup_confirm_sent.html", map[string]any{
				"Email": req.Email,
			})
		}

		return h.renderSignupError(c,
			"That was submitted a little too quickly. Please try again.")
	}

	// Metered on the source address and on its subnet, before anything is
	// created and before any mail is sent. The subnet counter is the one that
	// sees a caller spending a fresh address per account.
	clientIP := getClientIP(c)
	if h.rateLimitExceeded(rateLimitScopeSignupIP, clientIP) ||
		h.rateLimitExceeded(rateLimitScopeSignupSubnet, signupSubnet(clientIP)) {
		log.Println("signup: rate limited")
		return h.renderSignupError(c,
			"Too many accounts have been created from your network recently. "+
				"Please try again later.")
	}

	if err := c.Validate(req); err != nil {
		log.Println("signup:", err)
		return h.renderSignupError(c, signupValidationMessage(req))
	}

	// Both taken-checks before anything is created. They are also enforced by
	// the database and by the graph's handle namespace; these exist to say
	// which field is the problem rather than surfacing a duplicate-key error.
	if taken, err := h.Repos.Users.IsEmail(req.Email); err != nil {
		log.Println("signup: email check failed:", err)
		return h.renderSignupError(c, "Could not create your account. Please try again.")
	} else if taken {
		return h.renderSignupError(c, "An account already exists for that email address.")
	}

	if taken, err := h.Repos.Users.IsUsername(req.Username); err != nil {
		log.Println("signup: username check failed:", err)
		return h.renderSignupError(c, "Could not create your account. Please try again.")
	} else if taken {
		return h.renderSignupError(c, "That username is already taken.")
	}

	user := entities.NewUser()
	user.PersonType = constants.PersonTypeRegistered
	user.Email = req.Email
	user.Alias = req.Username

	if err := user.SetPassword(req.Password); err != nil {
		log.Println("signup:", err)
		return h.renderSignupError(c, "Could not create your account. Please try again.")
	}

	if err := user.Validate(); err != nil {
		log.Println("signup:", err)
		return h.renderSignupError(c, "Could not create your account. Please try again.")
	}

	// The actor node first, through the graph.
	//
	// Going through ActorsGRPC is what applies the type's default features,
	// enforces the actor handle namespace, and publishes the "actor added"
	// event that search and feed consume — none of which a direct insert did.
	actor := lib_entities.NewAnahitaNode()
	actor.Type = constants.NodeActorPersonEntityType
	actor.Service = constants.PersonService
	actor.Alias = req.Username
	// Name is deliberately empty. Rendering falls back to the handle, which
	// is what both federation protocols do, and it leaves onboarding able to
	// tell "never set one" from "chose their handle as their name".
	actor.Access = constants.AnahitaAccessPublic
	actor.PersonType = constants.PersonTypeRegistered
	actor.Enabled = true
	// Set explicitly. NewAnahitaNode leaves these as Go's zero time — year 1 —
	// which a MySQL DATETIME cannot represent at all, so the insert fails under
	// the strict sql_mode rather than storing something odd.
	actor.CreationTime = time.Now()
	actor.UpdateTime = time.Now()

	created, err := h.Srv.ActorsGRPC.Add(actor, signupViewer())
	if err != nil {
		log.Println("signup: could not create the actor:", err)
		if isDuplicateHandle(err) {
			return h.renderSignupError(c, "That username is already taken.")
		}
		return h.renderSignupError(c, "Could not create your account. Please try again.")
	}

	user.ID = created.ID

	user, err = h.Repos.Users.Add(user)
	if err != nil {
		log.Println("signup:", err)
		// Compensating delete. The node and the credential row are two writes
		// to two services and cannot share a transaction, so a failure on the
		// second leg has to unwind the first — otherwise the handle is taken
		// by an account nobody can sign into and nobody can remove.
		if _, purgeErr := h.Srv.ActorsGRPC.Purge(created.ID, signupViewer()); purgeErr != nil {
			log.Printf("signup: orphaned actor %d after credential write failed: %v (purge also failed: %v)", created.ID, err, purgeErr)
		}
		return h.renderSignupError(c, "Could not create your account. Please try again.")
	}

	// Confirm the address before the account can be used.
	//
	// NOT signed in here, unlike invite-accept. An invited person proved
	// control of their address by following the link that brought them; a
	// self-serve signup has proved nothing, and signing them in would make
	// the confirmation step decorative — the thing it must never be, since it
	// is the cheapest bot filter there is and the address is what password
	// reset delivers to.
	if err := h.sendConfirmation(user); err != nil {
		log.Println("signup: could not send the confirmation:", err)
		// The account exists. Saying otherwise would send somebody back to a
		// form that will now refuse them for a duplicate.
		return h.renderError(c, "Check Your Email",
			"Your account was created, but we could not send the confirmation "+
				"email. Please use the resend link on the sign-in page.")
	}

	return h.renderTemplate(c, "signup_confirm_sent.html", map[string]any{
		"Email": user.Email,
	})
}

// sendConfirmation issues a confirmation token and mails the link.
//
// Shared by signup and by the resend endpoint, so the two cannot diverge on
// the token type, its lifetime, or what happens to an earlier one.
func (h *signupHandler) sendConfirmation(user *entities.User) error {
	// Retire any earlier link first. Re-sending would otherwise leave one live
	// token per attempt, each good for a day — and a confirmation link is a
	// way into an account that has no password check in front of it.
	if _, err := h.Repos.Tokens.InvalidateOutstanding(
		constants.TokenTypeSignupConfirm, user.Email,
	); err != nil {
		log.Println("signup: retiring earlier confirmations failed:", err)
	}

	token := entities.NewToken()
	token.Type = constants.TokenTypeSignupConfirm
	token.Status = constants.TokenStatusPending
	token.RecipientEmail = user.Email
	token.ExpiredAfter = constants.TokenExpiryTime
	token.IsUsed = false
	token.CreatedAt = time.Now()
	token.CreatedBy = user.ID
	token.UpdatedAt = time.Now()
	token.UpdatedBy = user.ID

	if err := token.GenerateToken(); err != nil {
		return fmt.Errorf("generate the confirmation token: %w", err)
	}

	if _, err := h.Repos.Tokens.Add(token); err != nil {
		return fmt.Errorf("store the confirmation token: %w", err)
	}

	return h.Notif.Signup.EmailConfirm(user, token)
}

// writeSignupSession mints the IdP session for a freshly created account.
func (h *signupHandler) writeSignupSession(c echo.Context, user *entities.User) error {
	viewer := lib_entities.NewViewer()
	viewer.ID = user.ID
	viewer.Email = user.Email
	viewer.Type = user.Type
	viewer.Authenticated = true

	viewerJSON, err := json.Marshal(viewer)
	if err != nil {
		return fmt.Errorf("viewer marshal: %w", err)
	}

	if _, err := h.Session.Add(c, constants.UserSessionKey, string(viewerJSON)); err != nil {
		return fmt.Errorf("session write: %w", err)
	}

	return nil
}

// signupValidationMessage says which field is wrong.
//
// The fields are re-checked here rather than read out of the validation error,
// because the shared RequestValidator wraps failures as
// echo.NewHTTPError(400, err.Error()) — a STRING. The typed
// validator.ValidationErrors is gone by the time a handler sees it, so there
// is nothing left to ask which field failed. Same problem, and the same local
// answer, as the token check in password.go.
//
// One message rather than a list, and the first problem in the order the form
// is read. Three complaints at once about a form with three fields tells
// somebody their whole submission was wrong when one character was.
func signupValidationMessage(req *requests.SignupAddRequest) string {
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

	if !strings.Contains(req.Email, "@") || len(req.Email) < 3 {
		return "Please enter a valid email address."
	}
	if len(req.Email) > 64 {
		return "That email address is too long — 64 characters at most."
	}

	if len(req.Password) < 15 {
		return "Your password needs to be at least 15 characters long."
	}
	if len(req.Password) > 150 {
		return "That password is too long — 150 characters at most."
	}

	// Everything checked above passed, so the validator refused something this
	// function does not know about. Say so plainly rather than guessing.
	return "Please check the form and try again."
}

// usernameCharacters mirrors the `username` validator tag, so the message and
// the rule cannot disagree about what is allowed.
var usernameCharacters = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// renderSignupError re-renders the form with a message and a fresh CSRF token.
//
// Fresh because the submitted one was consumed. Without a new one the person
// corrects their password, submits, and is told the form expired.
func (h *signupHandler) renderSignupError(c echo.Context, message string) error {
	token, err := generateCSRFToken()
	if err != nil {
		log.Println("signup: could not regenerate the CSRF token:", err)
		return h.renderError(c, "Something Went Wrong", message)
	}

	if _, err := h.Session.Add(c, sessionKeySignupCSRF, token); err != nil {
		log.Println("signup: could not store the CSRF token:", err)
		return h.renderError(c, "Something Went Wrong", message)
	}

	// The render time is refreshed with the token. Without it a retry is timed
	// from the FIRST render, which would pass trivially — and a person
	// correcting one character would sail past a check that is supposed to
	// apply to every submission.
	if _, err := h.Session.Add(c, sessionKeySignupRendered,
		strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		log.Println("signup: could not store the render time:", err)
	}

	return h.renderTemplate(c, "signup.html", map[string]any{
		"CSRFToken":     token,
		"HoneypotField": honeypotField,
		"Error":         message,
	})
}

// ---------------------------------------------------------------------
// Email confirmation
// ---------------------------------------------------------------------

// Confirm spends a confirmation token and marks the address verified.
//
//	GET /signup/confirm?token=...
//
// A GET that changes state, which is a deliberate exception: the person
// arrives by clicking a link in their mail client, and there is no form to
// post from. The token is single-use and carries its own entropy, so the
// usual objection — that a GET can be triggered by anything that can make the
// browser fetch a URL — costs an attacker a token they would have to have
// stolen from the mailbox first, at which point they have the account anyway.
func (h *signupHandler) Confirm(c echo.Context) error {
	tokenBody := c.QueryParam("token")
	if tokenBody == "" {
		return h.renderError(c, "Invalid Link",
			"This confirmation link is not valid. Please request a new one "+
				"from the sign-in page.")
	}

	token, err := h.Repos.Tokens.ReadByBody(tokenBody)
	if err != nil {
		log.Println("signup confirm:", err)
		return h.renderError(c, "Invalid Link",
			"This confirmation link is not valid. Please request a new one "+
				"from the sign-in page.")
	}

	if token.Type != constants.TokenTypeSignupConfirm {
		return h.renderError(c, "Invalid Link",
			"This link is not valid for confirming an email address.")
	}

	user, err := h.Repos.Users.ReadByEmail(token.RecipientEmail)
	if err != nil || user == nil {
		log.Println("signup confirm: account lookup failed:", err)
		return h.renderError(c, "Account Not Found",
			"We could not find an account for this confirmation link.")
	}

	// Already done is a SUCCESS, not an error, and it is the common case:
	// mail clients prefetch links, people click twice, and a second visit
	// should land somewhere that says "you're confirmed" rather than
	// "this link has been used" — which reads as a failure for something
	// that worked.
	if user.IsEmailVerified() {
		return h.renderTemplate(c, "signup_confirm_success.html", map[string]any{
			"Email": user.Email,
		})
	}

	switch token.Status {
	case constants.TokenStatusUsed:
		return h.renderError(c, "Link Already Used",
			"This confirmation link has already been used. Please request a "+
				"new one from the sign-in page.")
	case constants.TokenStatusExpired:
		return h.renderError(c, "Link Expired",
			"This confirmation link has expired. Please request a new one "+
				"from the sign-in page.")
	}

	if err := h.Repos.Users.MarkEmailVerified(user.ID); err != nil {
		log.Println("signup confirm: could not mark verified:", err)
		return h.renderError(c, "Something Went Wrong",
			"We could not confirm your address. Please try the link again.")
	}

	// Consume the token. Logged rather than fatal if it fails: the address is
	// already verified, and refusing here would tell somebody their
	// confirmation had not worked when it had.
	token.IsUsed = true
	token.UpdatedAt = time.Now()
	token.UpdatedBy = user.ID
	if _, err := h.Repos.Tokens.Edit(token); err != nil {
		log.Printf("signup confirm: could not consume token %d: %v", token.ID, err)
	}

	// The welcome, now that there is something to welcome them to.
	welcomeUser := *user
	go func() {
		if err := h.Notif.Signup.EmailSignup(&welcomeUser); err != nil {
			log.Println("signup confirm: welcome email failed:", err)
		}
	}()

	return h.renderTemplate(c, "signup_confirm_success.html", map[string]any{
		"Email": user.Email,
	})
}

// ResendForm renders the "send me another confirmation" form.
//
//	GET /signup/confirm/resend
func (h *signupHandler) ResendForm(c echo.Context) error {
	return h.renderTemplate(c, "signup_confirm_resend.html", map[string]any{})
}

// Resend issues a fresh confirmation link.
//
//	POST /signup/confirm/resend
//
// Answers identically whether or not the address has an account, for the same
// reason forgot-password does: this is an unauthenticated endpoint that sends
// mail, so a visibly different response would make it a way to ask whether
// somebody is a member.
func (h *signupHandler) Resend(c echo.Context) error {
	req := &requests.PasswordForgot{}
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return h.renderTemplate(c, "signup_confirm_resend.html", map[string]any{
			"Error": "Please enter a valid email address.",
			"Email": req.Email,
		})
	}

	// Throttled before the lookup, so probing costs the same either way.
	throttled := h.rateLimitExceeded(rateLimitScopeSignupConfirm, req.Email) ||
		h.rateLimitExceeded(rateLimitScopePasswordIP, getClientIP(c))

	if !throttled {
		user, err := h.Repos.Users.ReadByEmail(req.Email)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Println("signup resend: lookup failed:", err)
		}

		// Nothing to do for an address that is already confirmed. Sending a
		// confirmation to somebody who does not need one is a way to have
		// this endpoint mail any member on demand.
		if user != nil && !user.IsEmailVerified() {
			if err := h.sendConfirmation(user); err != nil {
				log.Println("signup resend:", err)
			}
		}
	}

	return h.renderTemplate(c, "signup_confirm_sent.html", map[string]any{
		"Email": req.Email,
	})
}
