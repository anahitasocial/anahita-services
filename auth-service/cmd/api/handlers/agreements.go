package handlers

import (
	"errors"
	"log"
	"net/http"

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
		return c.JSON(http.StatusBadRequest, "Could not read the request.")
	}

	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrAgreementVersionMalformed.Error())
	}

	// Accepted must be true. An endpoint that records `accepted: false` has a
	// meaning — withdrawing consent — that nothing in the product offers, and
	// withdrawal would need to answer "what happens to the account now?" before
	// it could be allowed. Refused rather than half-supported.
	if !req.Accepted {
		return c.JSON(http.StatusBadRequest,
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
			return c.JSON(http.StatusBadRequest, err.Error())
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := write(user.ID, consent); err != nil {
		log.Printf("agreements %s: could not record (person=%d): %v",
			document, user.ID, err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"accepted": true,
		"version":  consent.Version,
	})
}
