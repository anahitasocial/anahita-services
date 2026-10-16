package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	libconstants "anahita-libs/constants"
	lib_entities "anahita-libs/data/entities"
	"anahita-libs/validators"

	"auth/cmd/api/permissions"
	"auth/data/repositories"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// GET /accounts. The query is tested against MySQL in the repository; what is
// pinned here is who may ask and how the query string becomes a filter.

type fakeAccounts struct {
	asked *repositories.AccountFilter
}

func (f *fakeAccounts) Browse(filter *repositories.AccountFilter) ([]*repositories.Account, error) {
	f.asked = filter
	return []*repositories.Account{{
		ID: 7, Alias: "ada", Name: "Ada", State: "active", Tier: "empty",
		CreatedAt: time.Now(), LastActiveAt: time.Now(),
	}}, nil
}

func (f *fakeAccounts) Count(filter *repositories.AccountFilter) (uint64, error) {
	return 1, nil
}

func browseAccounts(t *testing.T, personType string, scopes []string, query string) (*httptest.ResponseRecorder, *fakeAccounts) {
	t.Helper()

	te := newTestEnv()
	accounts := &fakeAccounts{}
	te.Repos.Accounts = accounts
	te.Perms = permissions.NewPermissions(&permissions.Env{Config: te.Config, Repos: te.Repos})

	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())

	req := httptest.NewRequest(http.MethodGet, "/accounts?"+query, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	viewer := lib_entities.NewViewer()
	viewer.ID = 42
	viewer.PersonType = personType
	viewer.Scopes = scopes
	c.Set(libconstants.ViewerContextKey, viewer)

	if err := newAccountsHandler(te.Env).Browse(c); err != nil {
		t.Fatalf("browse: %v", err)
	}
	answers.Handler(newAccountsHandler(te.Env).Browse, rec)
	return rec, accounts
}

var readScope = []string{libconstants.OAuthScopeActorsRead}

func TestAccountsAreForSuperAdministratorsOnly(t *testing.T) {
	for _, tc := range []struct {
		name       string
		personType string
		scopes     []string
		want       int
	}{
		{"an administrator", libconstants.PersonTypeAdmin, readScope, http.StatusForbidden},
		{"a member", libconstants.PersonTypeRegistered, readScope, http.StatusForbidden},
		{"a super administrator without the scope", libconstants.PersonTypeSuperAdmin, nil, http.StatusForbidden},
		{"a super administrator", libconstants.PersonTypeSuperAdmin, readScope, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, accounts := browseAccounts(t, tc.personType, tc.scopes, "")

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if (accounts.asked != nil) != (tc.want == http.StatusOK) {
				t.Error("the list was read for somebody who may not see it")
			}
		})
	}
}

func TestAccountsQueryBecomesAFilter(t *testing.T) {
	rec, accounts := browseAccounts(t, libconstants.PersonTypeSuperAdmin, readScope,
		"tier=empty&email_verified=false&onboarded=false&content_max=0&last_active_before=2024-01-01&has_admin=false&sort=name&dir=desc&limit=500")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", rec.Code, rec.Body.String())
	}

	filter := accounts.asked
	if filter.Kind != repositories.AccountKindPerson || filter.Tier != "empty" ||
		!filter.OnlyUnverified || !filter.OnlyNotOnboarded || filter.ContentMax != 0 ||
		filter.Sort != "name" || !filter.Descending || filter.Limit != 500 {
		t.Errorf("the filter: %+v", filter)
	}
	if filter.LastActiveBefore == nil || filter.LastActiveBefore.Format("2006-01-02") != "2024-01-01" {
		t.Errorf("last active before: %v", filter.LastActiveBefore)
	}

	// has_admin is a question about groups, and is ignored for people.
	if filter.OnlyWithoutAdmin {
		t.Error("the people list was filtered on having an administrator")
	}

	// Two years, unless the installation says otherwise.
	if days := time.Since(filter.DormantBefore).Hours() / 24; days < 729 || days > 731 {
		t.Errorf("the dormant line is %.0f days back, want 730", days)
	}

	body := rec.Body.String()
	for _, want := range []string{`"kind":"person"`, `"email_verified":false`, `"total":1`} {
		if !strings.Contains(body, want) {
			t.Errorf("the answer has no %s: %s", want, body)
		}
	}
	if strings.Contains(body, "admin_count") {
		t.Errorf("a person carries a group's field: %s", body)
	}
}

func TestAccountsDefaultsAndGroups(t *testing.T) {
	rec, accounts := browseAccounts(t, libconstants.PersonTypeSuperAdmin, readScope,
		"kind=group&has_admin=false&email_verified=false")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	filter := accounts.asked
	if filter.Kind != repositories.AccountKindGroup || !filter.OnlyWithoutAdmin || filter.OnlyUnverified {
		t.Errorf("the filter: %+v", filter)
	}

	// Not asking about content is not the same as asking for none.
	if filter.ContentMax >= 0 {
		t.Errorf("content_max defaulted to %d, which is a filter", filter.ContentMax)
	}
	if filter.Limit != accountsDefaultLimit {
		t.Errorf("limit defaulted to %d", filter.Limit)
	}

	if !strings.Contains(rec.Body.String(), `"admin_count":0`) || strings.Contains(rec.Body.String(), "email_verified") {
		t.Errorf("a group's row: %s", rec.Body.String())
	}
}

func TestAccountsRefuseWhatTheyDoNotUnderstand(t *testing.T) {
	for _, query := range []string{
		"kind=hashtag",
		"tier=sleepy",
		"last_active_before=yesterday",
		"email_verified=true",
		"content_max=-1",
		"sort=password",
		"limit=501",
	} {
		t.Run(query, func(t *testing.T) {
			rec, accounts := browseAccounts(t, libconstants.PersonTypeSuperAdmin, readScope, query)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if accounts.asked != nil {
				t.Error("the list was read anyway")
			}
		})
	}
}
