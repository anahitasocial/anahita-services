package handlers

import (
	"anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"
	"auth/data/entities"
	"auth/requests"
	"log"
	"net/http"
	"strings"
	"time"

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

// Signup creates a new user
func (h *signupHandler) Add(c echo.Context) error {
	var (
		req    = requests.NewSignupRequest().Add
		viewer = getViewer(c)
	)

	ok, err := h.Perms.Signup.CanAdd(viewer, nil)
	if !ok || err != nil {
		return c.JSON(http.StatusForbidden, err)
	}

	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	// making sure that the user doesn't already exist

	// Chcking if the user already exists by email
	emailUser, _ := h.Repos.Users.ReadByEmail(req.Email)
	if emailUser != nil {
		return c.JSON(http.StatusConflict, "A user with this email already exists")
	}

	// Checking if the user already exists by username
	usernameUser, _ := h.Repos.Users.ReadByUsername(req.Username)
	if usernameUser != nil {
		return c.JSON(http.StatusConflict, "A user with this username already exists")
	}

	// Creating the user
	user := entities.NewUser()
	user.PersonType = constants.PersonTypeRegistered
	user.Email = req.Email
	user.Alias = req.Username

	if err := user.SetPassword(req.Password); err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	if err := user.Validate(); err != nil {
		return c.JSON(http.StatusForbidden, err)
	}

	// The actor node first, through the graph.
	//
	// This used to be an INSERT INTO nodes inside the repository. Going
	// through ActorsGRPC is what applies the type's default features,
	// enforces the actor handle namespace, and publishes the "actor added"
	// event that search and feed consume — none of which a direct insert did.
	actor := lib_entities.NewAnahitaNode()
	actor.Type = constants.NodeActorPersonEntityType
	actor.Service = constants.PersonService
	actor.Alias = req.Username
	actor.Name = req.Username
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
		// Logged before it is mapped. A handle already taken is the expected
		// failure and a 409 is right for it, but everything else is a fault on
		// this side — and answering 409 for all of them tells the caller to
		// pick another name for a problem another name will not fix.
		log.Println("signup: could not create the actor:", err)
		if isDuplicateHandle(err) {
			return c.JSON(http.StatusConflict, "A user with this username already exists")
		}
		return c.JSON(http.StatusInternalServerError, "Could not create your account. Please try again.")
	}

	user.ID = created.ID

	user, err = h.Repos.Users.Add(user)
	if err != nil {
		// Compensating delete. The node and the credential row are two writes
		// to two services and cannot share a transaction, so a failure on the
		// second leg has to unwind the first — otherwise the handle is taken
		// by an account nobody can sign into and nobody can remove.
		if _, purgeErr := h.Srv.ActorsGRPC.Purge(created.ID, signupViewer()); purgeErr != nil {
			log.Printf("signup: orphaned actor %d after credential write failed: %v (purge also failed: %v)", created.ID, err, purgeErr)
		}
		return c.JSON(http.StatusInternalServerError, err)
	}

	// Welcome email. The notifier and its copy have existed since the service
	// was written and no handler ever called them, so signing up has always
	// been silent — the first mail anybody received was whatever they
	// triggered next.
	//
	// Fire-and-forget after the row is committed, so it describes an account
	// that exists. A copy is captured because the response serialises the same
	// pointer.
	notifyUser := *user
	go func() {
		if err := h.Notif.Signup.EmailSignup(&notifyUser); err != nil {
			log.Println("signup: welcome email failed:", err)
		}
	}()

	return c.JSON(http.StatusCreated, user)
}

func (h *signupHandler) Confirm(c echo.Context) error {
	return c.NoContent(http.StatusAccepted)
}
