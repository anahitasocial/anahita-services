package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	libconstants "anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"
	libservices "anahita-libs/data/services"
	"anahita-libs/policy"
	"anahita-libs/proto/graphpb"
	"anahita-libs/validators"

	"auth/cmd/api/permissions"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// POST /accounts/make-registered. The change itself is tested against MySQL
// in graph-grpc-service; what is pinned here is everything that stands in
// front of it.

type fakePublicContent struct {
	libservices.ActorsGRPCInterface

	counts    *graphpb.PublicAccessCountsResponse
	converted int
}

func (f *fakePublicContent) PublicAccessCounts() (*graphpb.PublicAccessCountsResponse, error) {
	return f.counts, nil
}

func (f *fakePublicContent) MakePublicRegistered(viewer *lib_entities.Viewer) (*graphpb.PublicAccessCountsResponse, error) {
	f.converted++
	return f.counts, nil
}

const convertingViewer = 42

func makeRegistered(t *testing.T, personType string, proved bool, body string) (*httptest.ResponseRecorder, *fakePublicContent, *testEnv) {
	t.Helper()

	te := newTestEnv()
	graph := &fakePublicContent{counts: &graphpb.PublicAccessCountsResponse{People: 2, Groups: 1, Posts: 4}}
	te.Srv.ActorsGRPC = graph
	te.Perms = permissions.NewPermissions(&permissions.Env{Config: te.Config, Repos: te.Repos})
	if proved {
		if err := te.markStepUp(convertingViewer); err != nil {
			t.Fatalf("marking the proof: %v", err)
		}
	}

	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())

	req := httptest.NewRequest(http.MethodPost, "/accounts/make-registered", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	viewer := lib_entities.NewViewer()
	viewer.ID = convertingViewer
	viewer.PersonType = personType
	viewer.Scopes = []string{libconstants.OAuthScopeActorsRead, libconstants.OAuthScopeActorsWrite}
	c.Set(libconstants.ViewerContextKey, viewer)

	if err := newAccountsHandler(te.Env).MakeRegistered(c); err != nil {
		t.Fatalf("make registered: %v", err)
	}
	return rec, graph, te
}

// On an installation that still allows public content, converting would be
// undone by the next public post, and pressing it by mistake would hide an
// open site from its visitors.
func TestMakeRegisteredIsRefusedWhilePublicContentIsAllowed(t *testing.T) {
	rec, graph, _ := makeRegistered(t, libconstants.PersonTypeSuperAdmin, true, `{"confirm":"CONVERT 7"}`)

	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "public_content_allowed") {
		t.Errorf("answered %d %s, want 409 public_content_allowed", rec.Code, rec.Body.String())
	}
	if graph.converted != 0 {
		t.Error("the conversion ran anyway")
	}
}

func TestMakeRegisteredNeedsASuperAdministratorAProofAndTheCount(t *testing.T) {
	restore := policy.SetForTest("registered")
	defer restore()

	for _, tc := range []struct {
		name       string
		personType string
		proved     bool
		body       string
		want       int
		wantBody   string
		converted  int
	}{
		{"an administrator", libconstants.PersonTypeAdmin, true, `{"confirm":"CONVERT 7"}`,
			http.StatusForbidden, "", 0},
		{"no proof of identity", libconstants.PersonTypeSuperAdmin, false, `{"confirm":"CONVERT 7"}`,
			http.StatusForbidden, "step_up_required", 0},
		{"the wrong count", libconstants.PersonTypeSuperAdmin, true, `{"confirm":"CONVERT 6"}`,
			http.StatusBadRequest, "confirmation_mismatch", 0},
		{"no confirmation", libconstants.PersonTypeSuperAdmin, true, `{}`,
			http.StatusBadRequest, "", 0},
		{"proved and confirmed", libconstants.PersonTypeSuperAdmin, true, `{"confirm":"CONVERT 7"}`,
			http.StatusOK, `"total":7`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, graph, te := makeRegistered(t, tc.personType, tc.proved, tc.body)

			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("answered %d %s, want %d %s", rec.Code, rec.Body.String(), tc.want, tc.wantBody)
			}
			if graph.converted != tc.converted {
				t.Errorf("the conversion ran %d time(s), want %d", graph.converted, tc.converted)
			}

			// One proof, one conversion. A refusal leaves the proof for the
			// corrected request.
			stillProved := te.requireStepUp(convertingViewer) == nil
			if tc.converted == 1 && stillProved {
				t.Error("the proof of identity was not spent")
			}
			if tc.converted == 0 && tc.proved && !stillProved {
				t.Error("a refused request spent the proof of identity")
			}
		})
	}
}
