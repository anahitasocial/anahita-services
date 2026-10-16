package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"actor/data/services"
	libconstants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libservices "anahita-libs/data/services"
	"anahita-libs/proto/graphpb"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// POST /groups/purge-batch and DELETE /groups/:id/purge. What is worth pinning
// is what stands in front of the graph: who may ask, the proof of identity,
// and the typed confirmation.

type fakePurgingActors struct {
	libservices.ActorsGRPCInterface

	result *graphpb.ActorPurgeBatchResponse
	asked  []*libservices.ActorPurgeRequest
}

func (f *fakePurgingActors) RequestPurge(req *libservices.ActorPurgeRequest, viewer *entities.Viewer) (*graphpb.ActorPurgeBatchResponse, error) {
	f.asked = append(f.asked, req)
	return f.result, nil
}

// fakeProofs is the step-up store: one proof, which is spent by deleting it.
type fakeProofs struct {
	proved bool
}

func (f *fakeProofs) Exists(key string) (bool, error) { return f.proved, nil }

func (f *fakeProofs) Del(keys ...string) error {
	f.proved = false
	return nil
}

func purgeBatch(t *testing.T, actors *fakePurgingActors, proofs *fakeProofs, personType, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())

	req := httptest.NewRequest(http.MethodPost, "/groups/purge-batch", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	viewer := entities.NewViewer()
	viewer.ID = 42
	viewer.PersonType = personType
	c.Set(libconstants.ViewerContextKey, viewer)

	h := NewGroupHandler(&Env{Srv: &services.Services{ActorsGRPC: actors}, Session: proofs})
	if err := h.PurgeBatch(c); err != nil {
		t.Fatalf("purge batch: %v", err)
	}
	answers.Handler(h.PurgeBatch, rec)
	return rec
}

func queuedResult() *graphpb.ActorPurgeBatchResponse {
	return &graphpb.ActorPurgeBatchResponse{BatchId: "3f0e3a4e-8c1c-4b9e-9d0a-111111111111", Queued: 2}
}

func TestPurgeBatchIsForSuperAdministratorsOnly(t *testing.T) {
	for _, personType := range []string{
		libconstants.PersonTypeAdmin,
		libconstants.PersonTypeRegistered,
		libconstants.PersonTypeGuest,
	} {
		t.Run(personType, func(t *testing.T) {
			actors := &fakePurgingActors{result: queuedResult()}
			rec := purgeBatch(t, actors, &fakeProofs{proved: true}, personType,
				`{"ids":[7,8],"confirm":"PURGE 2"}`)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", rec.Code)
			}
			if len(actors.asked) != 0 {
				t.Error("the graph was asked to purge anyway")
			}
		})
	}
}

func TestPurgeBatchNeedsAProofAndTheTypedCount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		proved bool
		body   string
		want   int
		asked  bool
	}{
		{"no proof of identity", false, `{"ids":[7,8],"confirm":"PURGE 2"}`, http.StatusForbidden, false},
		{"the count is wrong", true, `{"ids":[7,8],"confirm":"PURGE 3"}`, http.StatusBadRequest, false},
		{"no confirmation", true, `{"ids":[7,8]}`, http.StatusBadRequest, false},
		{"nothing named", true, `{"ids":[],"confirm":"PURGE 0"}`, http.StatusBadRequest, false},
		{"proved and confirmed", true, `{"ids":[7,8],"confirm":"PURGE 2"}`, http.StatusAccepted, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actors := &fakePurgingActors{result: queuedResult()}
			proofs := &fakeProofs{proved: tc.proved}
			rec := purgeBatch(t, actors, proofs, libconstants.PersonTypeSuperAdmin, tc.body)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d; body %s", rec.Code, tc.want, rec.Body.String())
			}
			if (len(actors.asked) == 1) != tc.asked {
				t.Errorf("the graph was asked %d time(s)", len(actors.asked))
			}

			if !tc.asked {
				return
			}

			// A bulk request never removes administrators, and one proof is
			// good for one request.
			if actors.asked[0].AllowAdministrators {
				t.Error("a bulk request allowed administrators to be purged")
			}
			if proofs.proved {
				t.Error("the proof of identity was not spent")
			}
			if !strings.Contains(rec.Body.String(), `"batch_id":"3f0e3a4e`) {
				t.Errorf("no batch id in the answer: %s", rec.Body.String())
			}
		})
	}
}

// A refused request changed nothing, so it says why with 409 and leaves the
// proof for the corrected request.
func TestARefusedPurgeBatchKeepsTheProof(t *testing.T) {
	actors := &fakePurgingActors{result: &graphpb.ActorPurgeBatchResponse{
		Refused: []*graphpb.ActorPurgeRefusal{{Id: 8, Reason: "not_found"}},
	}}
	proofs := &fakeProofs{proved: true}

	rec := purgeBatch(t, actors, proofs, libconstants.PersonTypeSuperAdmin,
		`{"ids":[7,8],"confirm":"PURGE 2"}`)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"reason":"not_found"`) {
		t.Errorf("the answer does not say why: %s", rec.Body.String())
	}
	if !proofs.proved {
		t.Error("a refused request spent the proof of identity")
	}
}
