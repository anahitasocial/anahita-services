package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// The versions of the Terms of Service and Privacy Policy currently in force.
//
// Public and unauthenticated, because the page that needs it is the signup
// page — nobody reading it has an account yet. It discloses two version
// strings and nothing else.
//
// It exists because signup records WHICH text somebody accepted, and the
// client has to know which that is to send it back. The alternative — letting
// the server fill in whatever is current — would record an acceptance of the
// version in force at the moment of submission rather than the one the person
// read, which is the same as not recording a version at all.
//
// The URL comes from configuration too, so the documents can be moved or
// hosted somewhere else without a code change. Nothing checks that the
// document at that URL really is the version named beside it — that is the
// operator's job, which is why the two settings sit together.

type agreementsHandler struct {
	*Env
}

func newAgreementsHandler(env *Env) *agreementsHandler {
	return &agreementsHandler{env}
}

type agreementResponse struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Read returns the current agreement versions.
//
//	GET /agreements
func (h *agreementsHandler) Read(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]agreementResponse{
		"tos": {
			Version: h.Config.GetTosVersion(),
			URL:     h.Config.GetTosURL(),
		},
		"privacy": {
			Version: h.Config.GetPrivacyVersion(),
			URL:     h.Config.GetPrivacyURL(),
		},
	})
}
