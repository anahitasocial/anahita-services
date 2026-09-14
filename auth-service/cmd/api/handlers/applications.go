package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"

	"auth/data/entities"

	"github.com/labstack/echo/v4"
)

// The approval queue's administration surface.
//
// JSON, and administrator-only. The SCREEN that consumes it belongs with the
// other settings work and is not here; these endpoints are what it will call,
// and what makes `REGISTRATION_MODE=approval` usable in the meantime.

type applicationsHandler struct {
	*Env
}

func newApplicationsHandler(env *Env) *applicationsHandler {
	return &applicationsHandler{env}
}

// applicationResponse is what an administrator sees.
//
// NO PASSWORD HASH, and no token. The hash is in the row because approval
// needs it, and it leaves the building for nobody — an approval queue that
// serialises credentials is a far worse thing to leak than a list of people
// who would like to join.
type applicationResponse struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	WebsiteURL string `json:"website_url"`
	Status     string `json:"status"`
	InvitedBy  int64  `json:"invited_by,omitempty"`
	CreatedIP  string `json:"created_ip"`
	CreatedAt  string `json:"created_at"`

	// UsernameTaken says whether the name they asked for is still free.
	//
	// Computed at read time rather than stored, because it changes underneath
	// the queue: somebody else may take it while an application waits. An
	// administrator deciding without this finds out by having the approval
	// fail.
	UsernameTaken bool `json:"username_taken"`
}

func (h *applicationsHandler) mapApplication(
	app *entities.RegistrationApplication,
) applicationResponse {
	taken, err := h.Repos.Users.IsUsername(app.Username)
	if err != nil {
		log.Println("applications: username check failed:", err)
	}

	return applicationResponse{
		ID:            app.ID,
		Username:      app.Username,
		Email:         app.Email,
		Name:          app.Name,
		Body:          app.Body,
		WebsiteURL:    app.WebsiteURL,
		Status:        app.Status,
		InvitedBy:     app.InvitedBy,
		CreatedIP:     app.CreatedIP,
		CreatedAt:     app.CreatedAt.Format(time.RFC3339),
		UsernameTaken: taken,
	}
}

// Browse lists the queue, oldest first.
//
//	GET /applications
func (h *applicationsHandler) Browse(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.Invite.CanBrowse(viewer); err != nil {
		log.Println("applications browse:", err)
		return c.NoContent(http.StatusForbidden)
	}

	limit, _ := strconv.ParseUint(c.QueryParam("limit"), 10, 64)
	offset, _ := strconv.ParseUint(c.QueryParam("offset"), 10, 64)

	apps, err := h.Repos.Applications.BrowsePending(limit, offset)
	if err != nil {
		log.Println("applications browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	total, err := h.Repos.Applications.CountPending()
	if err != nil {
		log.Println("applications browse: count failed:", err)
	}

	items := make([]applicationResponse, 0, len(apps))
	for _, app := range apps {
		items = append(items, h.mapApplication(app))
	}

	return c.JSON(http.StatusOK, map[string]any{
		"applications": items,
		"total":        total,
	})
}

type decisionRequest struct {
	Note string `json:"note"`
}

// Approve creates the account somebody applied for.
//
//	POST /applications/:id/approve
func (h *applicationsHandler) Approve(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.Invite.CanAdd(viewer); err != nil {
		log.Println("applications approve:", err)
		return c.NoContent(http.StatusForbidden)
	}

	app, err := h.readPendingApplication(c)
	if err != nil {
		return err
	}

	req := &decisionRequest{}
	_ = c.Bind(req)

	// The username is checked again HERE, not only when they applied.
	//
	// An application can wait for days, and somebody else can take the name in
	// the meantime. Without this the approval fails on a duplicate-key error
	// from the graph, which tells the administrator nothing and leaves the
	// application in the queue looking untouched.
	if taken, err := h.Repos.Users.IsUsername(app.Username); err != nil {
		log.Println("applications approve: username check failed:", err)
		return c.NoContent(http.StatusInternalServerError)
	} else if taken {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "username_taken",
			"message": "The username on this application has been taken since " +
				"it was submitted. Ask the applicant to choose another, or " +
				"reject it.",
		})
	}

	// The account, through the same path signup uses: the actor first, then
	// the credential row, with a compensating purge if the second fails.
	actor := lib_entities.NewAnahitaNode()
	actor.Type = constants.NodeActorPersonEntityType
	actor.Service = constants.PersonService
	actor.Alias = app.Username
	actor.Name = app.Name
	actor.Body = app.Body
	actor.WebsiteURL = app.WebsiteURL
	actor.Access = constants.AnahitaAccessPublic
	actor.PersonType = constants.PersonTypeRegistered
	actor.Enabled = true
	actor.CreationTime = time.Now()
	actor.UpdateTime = time.Now()

	created, err := h.Srv.ActorsGRPC.Add(actor, signupViewer())
	if err != nil {
		log.Println("applications approve: could not create the actor:", err)
		if isDuplicateHandle(err) {
			return c.JSON(http.StatusConflict, map[string]string{
				"error":   "username_taken",
				"message": "That username was taken moments ago. Please try again.",
			})
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	user := entities.NewUser()
	user.ID = created.ID
	user.Alias = created.Alias
	user.Name = created.Name
	user.Email = app.Email
	user.Password = app.Password
	user.PersonType = constants.PersonTypeRegistered
	user.Enabled = true
	user.CreationTime = time.Now()
	user.UpdateTime = time.Now()
	// Verified already: confirming the address is what put this application in
	// front of an administrator in the first place.
	user.EmailVerifiedAt = app.EmailVerifiedAt

	if _, err := h.Repos.Users.Add(user); err != nil {
		log.Println("applications approve: could not create the account:", err)
		if _, purgeErr := h.Srv.ActorsGRPC.Purge(created.ID, signupViewer()); purgeErr != nil {
			log.Printf("applications approve: orphaned actor %d: %v (purge also failed: %v)",
				created.ID, err, purgeErr)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	// Recorded LAST. If this fails the account exists and the application
	// still says pending, which a second approval attempt turns into a
	// duplicate-username refusal — visible, and recoverable by rejecting the
	// stale entry. The other order would mark somebody approved and leave them
	// with no account at all.
	if err := h.Repos.Applications.Decide(
		app.ID, entities.ApplicationStatusApproved, viewer.ID, req.Note,
	); err != nil {
		log.Printf("applications approve: account %d created but the "+
			"application was not marked approved: %v", created.ID, err)
	}

	notifyApp := *app
	go func() {
		if err := h.Notif.Signup.EmailApplicationApproved(&notifyApp); err != nil {
			log.Println("applications approve: notification failed:", err)
		}
	}()

	return c.JSON(http.StatusOK, map[string]any{
		"status":    entities.ApplicationStatusApproved,
		"person_id": created.ID,
	})
}

// Reject refuses an application.
//
//	POST /applications/:id/reject
func (h *applicationsHandler) Reject(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.Invite.CanDelete(viewer); err != nil {
		log.Println("applications reject:", err)
		return c.NoContent(http.StatusForbidden)
	}

	app, err := h.readPendingApplication(c)
	if err != nil {
		return err
	}

	req := &decisionRequest{}
	_ = c.Bind(req)

	if err := h.Repos.Applications.Decide(
		app.ID, entities.ApplicationStatusRejected, viewer.ID, req.Note,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Somebody else decided it between the read and the write.
			return c.JSON(http.StatusConflict, map[string]string{
				"error":   "already_decided",
				"message": "Another administrator has already answered this application.",
			})
		}
		log.Println("applications reject:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	notifyApp := *app
	go func() {
		if err := h.Notif.Signup.EmailApplicationRejected(&notifyApp); err != nil {
			log.Println("applications reject: notification failed:", err)
		}
	}()

	return c.JSON(http.StatusOK, map[string]any{
		"status": entities.ApplicationStatusRejected,
	})
}

// readPendingApplication loads the application named in the path, or writes
// the refusal and returns an error for the caller to return.
func (h *applicationsHandler) readPendingApplication(
	c echo.Context,
) (*entities.RegistrationApplication, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, c.NoContent(http.StatusBadRequest)
	}

	app, err := h.Repos.Applications.ReadByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, c.NoContent(http.StatusNotFound)
		}
		log.Println("applications: read failed:", err)
		return nil, c.NoContent(http.StatusInternalServerError)
	}

	if app.IsDecided() {
		return nil, c.JSON(http.StatusConflict, map[string]string{
			"error":   "already_decided",
			"message": "This application has already been answered.",
		})
	}

	// An unverified application is not in the queue and must not be decidable
	// from it — approving one would create an account on an address nobody has
	// shown they can receive mail at, which is the whole thing verification is
	// for.
	if !app.IsEmailVerified() {
		return nil, c.JSON(http.StatusConflict, map[string]string{
			"error":   "not_verified",
			"message": "This applicant has not confirmed their email address yet.",
		})
	}

	return app, nil
}
