package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	libconstants "anahita-libs/constants"
	"anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// PATCH /people/:id/featured is for super administrators. An administrator
// who is not one is refused before anything is read — featuring is a statement
// by the installation, not a moderation action.
func TestPersonSetFeaturedIsForSuperAdministratorsOnly(t *testing.T) {
	for _, personType := range []string{
		libconstants.PersonTypeAdmin,
		libconstants.PersonTypeRegistered,
		libconstants.PersonTypeGuest,
	} {
		t.Run(personType, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPatch, "/people/7/featured",
				strings.NewReader(`{"featured":true}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("id")
			c.SetParamValues("7")

			viewer := entities.NewViewer()
			viewer.ID = 42
			viewer.PersonType = personType
			c.Set(libconstants.ViewerContextKey, viewer)

			// No services at all: reaching one would panic, which is the
			// assertion that nothing was read or written.
			if err := NewPersonHandler(&Env{}).SetFeatured(c); err != nil {
				t.Fatalf("set featured: %v", err)
			}

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", rec.Code)
			}
		})
	}
}
