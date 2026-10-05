package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	libconstants "anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"
	"anahita-libs/policy"

	"auth/cmd/api/permissions"
	"auth/data/repositories"

	"github.com/labstack/echo/v4"
)

// NodeInfo is how the web app learns what kind of site this is: whether a
// visitor can read it, and whether anything new may be public. On a
// members-only site it is one of the few things that still answers, so the
// two fields have to be there and have to say what the services enforce.

type usersWithUsage struct {
	repositories.UsersInterface
}

func (usersWithUsage) CountInstanceUsage() (*repositories.InstanceUsage, error) {
	return &repositories.InstanceUsage{}, nil
}

func nodeInfoMetadata(t *testing.T) map[string]any {
	t.Helper()

	te := newTestEnv()
	te.Repos.Users = usersWithUsage{}

	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/nodeinfo/2.1", nil), rec)

	if err := newNodeInfoHandler(te.Env).Read(c); err != nil {
		t.Fatalf("nodeinfo: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("nodeinfo answered %d: %s", rec.Code, rec.Body.String())
	}

	var document struct {
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatalf("nodeinfo is not JSON: %s", rec.Body.String())
	}
	return document.Metadata
}

func TestNodeInfoSaysWhatVisitorsMayReadAndWhatMayBePublic(t *testing.T) {
	metadata := nodeInfoMetadata(t)
	if metadata["readAccess"] != "public" || metadata["minContentAccess"] != "public" {
		t.Errorf("an open installation reports readAccess=%v minContentAccess=%v",
			metadata["readAccess"], metadata["minContentAccess"])
	}

	for _, readAccess := range []string{policy.ReadPreview, policy.ReadRegistered} {
		restoreRead := policy.SetReadAccessForTest(readAccess)
		restoreContent := policy.SetForTest("registered")

		metadata = nodeInfoMetadata(t)
		if metadata["readAccess"] != readAccess || metadata["minContentAccess"] != "registered" {
			t.Errorf("set to %s and registered, it reports readAccess=%v minContentAccess=%v",
				readAccess, metadata["readAccess"], metadata["minContentAccess"])
		}

		restoreContent()
		restoreRead()
	}
}

// How much is public is a super administrator's to see, like the accounts
// list it sits beside.
func TestPublicContentCountsAreForSuperAdministratorsOnly(t *testing.T) {
	for _, tc := range []struct {
		personType string
		want       int
	}{
		{libconstants.PersonTypeRegistered, http.StatusForbidden},
		{libconstants.PersonTypeAdmin, http.StatusForbidden},
		{libconstants.PersonTypeSuperAdmin, http.StatusOK},
	} {
		te := newTestEnv()
		te.Srv.ActorsGRPC = &fakePublicContent{}
		te.Perms = permissions.NewPermissions(&permissions.Env{Config: te.Config, Repos: te.Repos})

		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/accounts/public-content", nil), rec)

		viewer := lib_entities.NewViewer()
		viewer.ID = 42
		viewer.PersonType = tc.personType
		viewer.Scopes = []string{libconstants.OAuthScopeActorsRead}
		c.Set(libconstants.ViewerContextKey, viewer)

		te.Srv.ActorsGRPC.(*fakePublicContent).counts = publicCounts(2, 1, 4)

		if err := newAccountsHandler(te.Env).PublicContent(c); err != nil {
			t.Fatalf("public content: %v", err)
		}
		if rec.Code != tc.want {
			t.Errorf("%s: answered %d, want %d", tc.personType, rec.Code, tc.want)
		}
		if tc.want != http.StatusOK {
			continue
		}

		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["total"] != float64(7) || body["public_allowed"] != true {
			t.Errorf("the counts: %v", body)
		}
	}
}
