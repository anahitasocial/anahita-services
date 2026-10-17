package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"auth/cmd/api/permissions"
	"auth/data/entities"
	"auth/data/repositories"

	libconstants "anahita-libs/constants"
	libentities "anahita-libs/data/entities"
)

// DELETE /clients/:id. The schema has no foreign keys, so nothing cascades:
// a client's tokens are revoked and its codes deleted by the handler, or they
// outlive it and work again for a client made later under the same client_id.

// calls records what the client's repositories were asked, in order.
type calls []string

type fakeClients struct {
	repositories.OAuthClientsInterface
	byID map[int64]*entities.OAuthClient
	log  *calls
}

func (f *fakeClients) ReadByID(id int64) (*entities.OAuthClient, error) {
	if c, ok := f.byID[id]; ok {
		return c, nil
	}
	return nil, errors.New("no client")
}

func (f *fakeClients) Delete(id int64) error {
	*f.log = append(*f.log, "delete client "+strconv.FormatInt(id, 10))
	return nil
}

type fakeAuthorizations struct {
	repositories.OAuthAuthorizationsInterface
	log *calls
}

func (f *fakeAuthorizations) DeleteByClientID(clientID string) error {
	*f.log = append(*f.log, "delete codes "+clientID)
	return nil
}

type revokingRefreshTokens struct {
	repositories.OAuthRefreshTokensInterface
	log *calls
	err error
}

func (f *revokingRefreshTokens) RevokeByClientID(clientID string) error {
	if f.err != nil {
		return f.err
	}
	*f.log = append(*f.log, "revoke tokens "+clientID)
	return nil
}

func deleteClientEnv(t *testing.T, refreshErr error) (*testEnv, *calls) {
	t.Helper()
	log := &calls{}
	te := newTestEnv()
	te.Repos.OAuthClients = &fakeClients{
		byID: map[int64]*entities.OAuthClient{5: {ID: 5, ClientID: "a-reader"}},
		log:  log,
	}
	te.Repos.OAuthAuthorizations = &fakeAuthorizations{log: log}
	te.Repos.OAuthRefreshTokens = &revokingRefreshTokens{log: log, err: refreshErr}
	te.Perms = permissions.NewPermissions(&permissions.Env{Config: te.Config, Repos: te.Repos})
	if err := te.Env.markStepUp(1); err != nil {
		t.Fatalf("mark step-up: %v", err)
	}
	return te, log
}

func deleteClient(t *testing.T, te *testEnv) *httptest.ResponseRecorder {
	t.Helper()
	viewer := libentities.NewViewer()
	viewer.ID = 1
	viewer.Authenticated = true
	viewer.PersonType = libconstants.PersonTypeSuperAdmin
	viewer.Scopes = []string{libconstants.OAuthScopeOAuthWrite}

	req := httptest.NewRequest(http.MethodDelete, "/clients/5", nil)
	rec := httptest.NewRecorder()
	c := newTestEcho().NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("5")
	c.Set(libconstants.ViewerContextKey, viewer)

	if err := newOAuthHandler(te.Env).DeleteClient(c); err != nil {
		t.Fatalf("delete: %v", err)
	}
	return rec
}

func TestDeletingAClientRevokesItsTokensFirst(t *testing.T) {
	te, log := deleteClientEnv(t, nil)

	rec := deleteClient(t, te)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	want := calls{"revoke tokens a-reader", "delete codes a-reader", "delete client 5"}
	if len(*log) != len(want) {
		t.Fatalf("calls = %v, want %v", *log, want)
	}
	for i := range want {
		if (*log)[i] != want[i] {
			t.Fatalf("calls = %v, want %v", *log, want)
		}
	}
}

// When the tokens cannot be revoked, the client stays, so that deleting it
// again finishes the job instead of leaving tokens nobody can find a client for.
func TestAClientWhoseTokensCannotBeRevokedIsKept(t *testing.T) {
	te, log := deleteClientEnv(t, errStoreDown)

	rec := deleteClient(t, te)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if len(*log) != 0 {
		t.Errorf("calls = %v, want none", *log)
	}
}
