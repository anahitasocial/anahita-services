package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	libs_responses "anahita-libs/responses"

	"auth/data/entities"
	"auth/requests"

	"github.com/labstack/echo/v4"
)

// Accepting the Terms of Service and the Privacy Policy.
//
// The web app decides when to ask: it holds the version of each document in
// statics/legal/index.js and compares it against what the viewer has accepted,
// sending anybody behind to the agreements page. These endpoints are what that
// page calls, one per document.
//
// They act on THE SIGNED-IN VIEWER and take no person id. There is nobody else
// whose acceptance anybody should be able to record, so there is no id to
// validate and no way to get it wrong — accepting terms on another person's
// behalf is not something to guard against, it is something to make
// unexpressible.

type agreementsHandler struct {
	*Env
}

func newAgreementsHandler(env *Env) *agreementsHandler {
	return &agreementsHandler{env}
}

// consentItem is one recorded acceptance, as a client sees it.
//
// Version, when, and whether — and NOT the audit trio the row also
// holds. IP, user agent and OS are there for "who accepted this, and
// from where", a question the entity comment says is asked by somebody
// looking at a row rather than by code. An administration screen is not
// that; putting a person's IP address on one would be a privacy
// expansion nobody asked for, arriving by accident because the column
// happened to be adjacent.
type consentItem struct {
	// Whether they have accepted it.
	Accepted bool `json:"accepted"`
	// Which version they accepted: `1.0`.
	Version string `json:"version"`
	// When. Null when they never have.
	AcceptedAt *string `json:"accepted_at"`
}

// Agreements is what somebody has accepted of the two documents.
type Agreements struct {
	// The Terms of Service.
	Tos consentItem `json:"tos"`
	// The Privacy Policy.
	Privacy consentItem `json:"privacy"`
}

// Accepted is an acceptance recorded.
type Accepted struct {
	// Always true.
	Accepted bool `json:"accepted"`
	// The version accepted.
	Version string `json:"version"`
}

func mapConsent(c *entities.Consent) consentItem {
	item := consentItem{
		Accepted: c.Accepted,
		Version:  c.Version,
	}

	if c.AcceptedAt != nil {
		at := c.AcceptedAt.Format(time.RFC3339)
		item.AcceptedAt = &at
	}

	return item
}

// Read returns what one person has accepted.
//
//	GET /agreements/:person_id
//
// Takes an id, unlike the two PATCH endpoints below, and the asymmetry
// is deliberate. Recording an acceptance on somebody else's behalf is
// meaningless, so those take no id and cannot express it. READING one is
// meaningful for two different people: its subject, and an administrator
// working out whether somebody is behind on the current terms.
//
// 200: { "data": { "tos": {...}, "privacy": {...} } }
// 403: not yourself and not an administrator
// 404: no such person
func (h *agreementsHandler) Read(c echo.Context) error {
	viewer := getViewer(c)

	personID, err := strconv.ParseInt(c.Param("person_id"), 10, 64)
	if err != nil || personID <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(personID)
	if err != nil || user == nil {
		log.Println("agreements read: person lookup:", err)
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Perms.Agreement.CanRead(viewer, user); err != nil {
		log.Println("agreements read:", err)
		return c.NoContent(http.StatusForbidden)
	}

	return c.JSON(http.StatusOK, libs_responses.Item[Agreements]{Data: Agreements{
		Tos:     mapConsent(&user.TermsOfService),
		Privacy: mapConsent(&user.PrivacyPolicy),
	}})
}

// EditTos records acceptance of the Terms of Service.
//
//	PATCH /agreements/tos
func (h *agreementsHandler) EditTos(c echo.Context) error {
	return h.accept(c, "tos",
		func(u *entities.User) *entities.Consent { return &u.TermsOfService },
		h.Repos.Users.EditTosConsent)
}

// EditPrivacy records acceptance of the Privacy Policy.
//
//	PATCH /agreements/privacy
func (h *agreementsHandler) EditPrivacy(c echo.Context) error {
	return h.accept(c, "privacy",
		func(u *entities.User) *entities.Consent { return &u.PrivacyPolicy },
		h.Repos.Users.EditPrivacyConsent)
}

// accept is the shared body of both endpoints.
//
// Shared so the two documents cannot drift in what they record or how they
// refuse — one of them quietly not stamping an IP address would only be found
// by somebody reading rows.
func (h *agreementsHandler) accept(
	c echo.Context,
	document string,
	consentOf func(*entities.User) *entities.Consent,
	write func(int64, *entities.Consent) error,
) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 || viewer.IsGuest() {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &requests.AgreementAccept{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		return refuse(c, http.StatusBadRequest, "version_malformed", ErrAgreementVersionMalformed.Error())
	}

	// Accepted must be true. An endpoint that records `accepted: false` has a
	// meaning — withdrawing consent — that nothing in the product offers, and
	// withdrawal would need to answer "what happens to the account now?" before
	// it could be allowed. Refused rather than half-supported.
	if !req.Accepted {
		return refuse(c, http.StatusBadRequest, "not_accepted",
			"This endpoint records an acceptance. Withdrawing one is not supported.")
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Printf("agreements %s: could not read the viewer (person=%d): %v",
			document, viewer.ID, err)
		return c.NoContent(http.StatusInternalServerError)
	}

	consent := consentOf(user)

	if err := acceptInto(c, consent, req.Version); err != nil {
		if errors.Is(err, ErrAgreementVersionMalformed) {
			return refuse(c, http.StatusBadRequest, "version_malformed", err.Error())
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := write(user.ID, consent); err != nil {
		log.Printf("agreements %s: could not record (person=%d): %v",
			document, user.ID, err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &Accepted{Accepted: true, Version: consent.Version})
}
