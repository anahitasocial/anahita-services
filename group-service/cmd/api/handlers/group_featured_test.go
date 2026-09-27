package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	libconstants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libservices "anahita-libs/data/services"
	"group/data/services"

	"github.com/labstack/echo/v4"
)

// PATCH /groups/:id/featured — what onboarding offers a new member to follow.
//
// The permission is the part worth pinning: featuring is a statement by the
// installation, so a site administrator who is not a super administrator is
// refused, and so is a group's own administrator.

type fakeFeaturingActors struct {
	libservices.ActorsGRPCInterface

	changed bool
	err     error

	calls    int
	featured bool
}

func (f *fakeFeaturingActors) ReadOfType(id int64, nodeType string, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	node := entities.NewAnahitaNode()
	node.ID = id
	node.Type = nodeType
	return node, nil
}

func (f *fakeFeaturingActors) SetFeatured(id int64, featured bool, viewer *entities.Viewer) (bool, error) {
	f.calls++
	f.featured = featured
	return f.changed, f.err
}

func setFeatured(t *testing.T, actors *fakeFeaturingActors, personType, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/groups/7/featured", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("7")

	viewer := entities.NewViewer()
	viewer.ID = 42
	viewer.PersonType = personType
	c.Set(libconstants.ViewerContextKey, viewer)

	h := NewGroupHandler(&Env{Srv: &services.Services{ActorsGRPC: actors}})
	if err := h.SetFeatured(c); err != nil {
		t.Fatalf("set featured: %v", err)
	}
	return rec
}

func TestSetFeaturedIsForSuperAdministratorsOnly(t *testing.T) {
	for _, personType := range []string{
		libconstants.PersonTypeAdmin,
		libconstants.PersonTypeRegistered,
		libconstants.PersonTypeGuest,
	} {
		t.Run(personType, func(t *testing.T) {
			actors := &fakeFeaturingActors{changed: true}
			rec := setFeatured(t, actors, personType, `{"featured":true}`)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", rec.Code)
			}
			if actors.calls != 0 {
				t.Error("the graph was asked to feature the group anyway")
			}
		})
	}
}

func TestSetFeaturedAnswers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		changed bool
		err     error
		want    int
	}{
		{"featured", `{"featured":true}`, true, nil, http.StatusOK},
		{"unfeatured", `{"featured":false}`, true, nil, http.StatusOK},
		{"already so", `{"featured":true}`, false, nil, http.StatusNoContent},
		{"disabled, archived or deleted", `{"featured":true}`, false,
			errors.New("rpc error: code = FailedPrecondition desc = not_featurable"),
			http.StatusConflict},
		{"graph down", `{"featured":true}`, false, errors.New("unavailable"),
			http.StatusInternalServerError},
		// Without the field there is no telling which way was meant, and
		// defaulting either way is a guess about an installation-wide list.
		{"no featured field", `{}`, false, nil, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actors := &fakeFeaturingActors{changed: tc.changed, err: tc.err}
			rec := setFeatured(t, actors, libconstants.PersonTypeSuperAdmin, tc.body)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d; body %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
