package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

const sessionKeySignupCSRF = "signup_csrf_token"

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

	return h.renderTemplate(c, "signup.html", map[string]any{
		"CSRFToken": token,
	})
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

	if err := c.Validate(req); err != nil {
		log.Println("signup:", err)
		return h.renderSignupError(c,
			"Please enter a valid email address, a handle of at least 3 "+
				"characters, and a password of at least 15 characters.")
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
		log.Println("signup: handle check failed:", err)
		return h.renderSignupError(c, "Could not create your account. Please try again.")
	} else if taken {
		return h.renderSignupError(c, "That handle is already taken.")
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
			return h.renderSignupError(c, "That handle is already taken.")
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

	// Welcome email, fire-and-forget after the row is committed so it
	// describes an account that exists.
	notifyUser := *user
	go func() {
		if err := h.Notif.Signup.EmailSignup(&notifyUser); err != nil {
			log.Println("signup: welcome email failed:", err)
		}
	}()

	// Signed in, in the same step. Same ending invite-accept already has: the
	// IdP session is written here, the browser goes to the app, and the BFF
	// completes the OAuth flow against the session it finds.
	if err := h.writeSignupSession(c, user); err != nil {
		log.Println("signup:", err)
		return h.renderError(c, "Account Created",
			"Your account was created but we could not sign you in "+
				"automatically. Please sign in.")
	}

	return c.Redirect(http.StatusSeeOther, h.Config.GetFrontendURL())
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

	return h.renderTemplate(c, "signup.html", map[string]any{
		"CSRFToken": token,
		"Error":     message,
	})
}
