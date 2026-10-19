package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	libconstants "anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"
	libservices "anahita-libs/data/services"
	"anahita-libs/validators"

	"auth/cmd/api/permissions"
	"auth/data/entities"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PATCH /accounts/:id/person-type. The change itself, and the refusal of
// one's own, are tested in graph-grpc-service, and the rule of who may make
// which change in anahita-libs; what is pinned here is what stands in front.

type fakePersonTypes struct {
	libservices.ActorsGRPCInterface

	err   error
	asked []string
}

func (f *fakePersonTypes) SetPersonType(id int64, personType string, viewer *lib_entities.Viewer) (bool, string, error) {
	f.asked = append(f.asked, personType)
	if f.err != nil {
		return false, "", f.err
	}
	return true, "registered", nil
}

const typingViewer = 7

// The people whose types are asked about: Odo registered, Kira an
// administrator, Sisko a super administrator.
const (
	odo   = 2
	kira  = 3
	sisko = 4
)

func person(id int64, personType string) *entities.User {
	u := &entities.User{Email: strconv.FormatInt(id, 10) + "@example.org"}
	u.ID = id
	u.PersonType = personType
	return u
}

func editPersonType(t *testing.T, personType string, proved bool, id int64, body string, graphErr error) (*httptest.ResponseRecorder, *fakePersonTypes, *testEnv) {
	t.Helper()

	te := newTestEnv(
		person(odo, libconstants.PersonTypeRegistered),
		person(kira, libconstants.PersonTypeAdmin),
		person(sisko, libconstants.PersonTypeSuperAdmin),
		person(typingViewer, personType),
	)
	graph := &fakePersonTypes{err: graphErr}
	te.Srv.ActorsGRPC = graph
	te.Perms = permissions.NewPermissions(&permissions.Env{Config: te.Config, Repos: te.Repos})
	if proved {
		if err := te.markStepUp(typingViewer); err != nil {
			t.Fatalf("marking the proof: %v", err)
		}
	}

	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())

	req := httptest.NewRequest(http.MethodPatch, "/accounts/"+strconv.FormatInt(id, 10)+"/person-type", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(strconv.FormatInt(id, 10))

	viewer := lib_entities.NewViewer()
	viewer.ID = typingViewer
	viewer.PersonType = personType
	viewer.Scopes = []string{libconstants.OAuthScopeActorsRead, libconstants.OAuthScopeActorsWrite}
	c.Set(libconstants.ViewerContextKey, viewer)

	if err := newAccountsHandler(te.Env).EditPersonType(c); err != nil {
		t.Fatalf("person type: %v", err)
	}
	return rec, graph, te
}

// Who may change a person's type, and to what (the user, 2026-10-18): an
// administrator makes administrators of registered people; the rest is a
// super administrator's; nobody changes their own; always after a proof.
func TestEditPersonType(t *testing.T) {
	const (
		toReg   = `{"person_type":"registered"}`
		toAdmin = `{"person_type":"administrator"}`
		toSuper = `{"person_type":"super-administrator"}`
	)
	reg := libconstants.PersonTypeRegistered
	admin := libconstants.PersonTypeAdmin
	super := libconstants.PersonTypeSuperAdmin

	for _, tc := range []struct {
		name       string
		personType string
		proved     bool
		id         int64
		body       string
		graphErr   error
		want       int
		wantBody   string
		asked      int
	}{
		{"somebody registered", reg, true, odo, toAdmin, nil,
			http.StatusForbidden, "", 0},
		{"an administrator, their own", admin, true, typingViewer, toSuper, nil,
			http.StatusConflict, "own_person_type", 0},
		{"an administrator makes an administrator", admin, true, odo, toAdmin, nil,
			http.StatusOK, `"previous":"registered"`, 1},
		{"an administrator makes a super administrator", admin, true, odo, toSuper, nil,
			http.StatusForbidden, "person_type_not_allowed", 0},
		{"an administrator demotes an administrator", admin, true, kira, toReg, nil,
			http.StatusForbidden, "person_type_not_allowed", 0},
		{"an administrator demotes a super administrator", admin, true, sisko, toAdmin, nil,
			http.StatusForbidden, "person_type_not_allowed", 0},
		{"a super administrator, their own, before any proof", super, false, typingViewer, toAdmin, nil,
			http.StatusConflict, "own_person_type", 0},
		{"a super administrator without a proof", super, false, kira, toSuper, nil,
			http.StatusForbidden, "step_up_required", 0},
		{"a super administrator makes a super administrator", super, true, kira, toSuper, nil,
			http.StatusOK, `"changed":true`, 1},
		{"a super administrator demotes one", super, true, sisko, toReg, nil,
			http.StatusOK, `"changed":true`, 1},
		{"the type they have already, no proof needed", super, false, kira, toAdmin, nil,
			http.StatusOK, `"changed":false`, 0},
		{"nobody with that id", super, true, 99, toAdmin, nil,
			http.StatusNotFound, "not_a_person", 0},
		{"a type there is not", super, true, odo, `{"person_type":"guest"}`, nil,
			http.StatusBadRequest, "", 0},
		{"no type", super, true, odo, `{}`, nil,
			http.StatusBadRequest, "", 0},
		{"not a person, says the graph", super, true, odo, toAdmin, status.Error(codes.NotFound, "not_a_person"),
			http.StatusNotFound, "not_a_person", 1},
		{"not allowed, says the graph", admin, true, odo, toAdmin, status.Error(codes.PermissionDenied, "person_type_not_allowed"),
			http.StatusForbidden, "person_type_not_allowed", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, graph, te := editPersonType(t, tc.personType, tc.proved, tc.id, tc.body, tc.graphErr)

			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("answered %d %s, want %d %s", rec.Code, rec.Body.String(), tc.want, tc.wantBody)
			}
			if len(graph.asked) != tc.asked {
				t.Errorf("the graph was asked %d time(s), want %d", len(graph.asked), tc.asked)
			}

			// One proof, one change; a refusal leaves it for the corrected
			// request.
			stillProved := te.requireStepUp(typingViewer) == nil
			changed := tc.want == http.StatusOK && tc.asked == 1
			if changed && stillProved {
				t.Error("the proof of identity was not spent")
			}
			if !changed && tc.proved && !stillProved {
				t.Error("a refused request spent the proof of identity")
			}
		})
	}
}
