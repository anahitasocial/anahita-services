package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
)

// fakeSession holds the proofs of step-up given, as Redis does.
type fakeSession struct{ proofs map[string]bool }

func (f *fakeSession) Exists(key string) (bool, error) { return f.proofs[key], nil }
func (f *fakeSession) Del(keys ...string) error {
	for _, key := range keys {
		delete(f.proofs, key)
	}
	return nil
}

type movingWorld struct {
	*world
	graph   *fakeGraphEdges
	queue   *fakeQueue
	session *fakeSession
	self    string
}

// Ana, on the fediverse here, followed by Amos here and by somebody on
// tycho.example; Naomi's new account there names Ana's, or not.
func newMovingWorld(naming []string) *movingWorld {
	w := newWorld()
	graph := &fakeGraphEdges{}
	amos := &entities.AnahitaNode{ID: 1, Type: constants.NodeActorPersonEntityType, Alias: "amos", Enabled: true, Access: "public"}
	w.nodes.people = append(w.nodes.people, amos)
	graph.add(constants.EdgeFollowEntityType, 1, anaID)

	m := &movingWorld{
		world:   w,
		graph:   graph,
		queue:   &fakeQueue{},
		session: &fakeSession{proofs: map[string]bool{}},
		self:    w.h.addresses.Actor(anaUUID),
	}
	if naming == nil {
		naming = []string{m.self}
	}
	w.h.Fetcher = &fakeFetcher{pages: map[string]string{naomiNew: naomiAt(naming, false)}}
	w.h.Actors = &fakeActors{kept: map[int64]*repositories.RemoteActor{}}
	w.h.Edges = graph
	w.h.Relations = &pairRelations{graph: graph}
	w.h.FollowEdges = &graphFollows{graph: graph}
	w.h.BlockEdges = &graphBlocks{graph: graph}
	w.h.MuteEdges = &graphMutes{graph: graph}
	w.h.Queue = m.queue
	w.h.Session = m.session
	w.h.Follows = &fakeFollows{kept: map[string]*repositories.Follow{}, inboxes: []string{"https://tycho.example/inbox"}}
	w.h.Announce = func(*event.EventPayload, string) error { return nil }
	return m
}

func (m *movingWorld) proved() { m.session.proofs["stepup:"+itoa(anaID)] = true }

func itoa(n int64) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

func (m *movingWorld) move(to string) *httptest.ResponseRecorder {
	return m.call(m.h.Move, http.MethodPost, "/", `{"to":"`+to+`"}`, anaID)
}

func (m *movingWorld) has(kind string, a, b int64) bool {
	return (&moveWorld{graph: m.graph}).has(kind, a, b)
}

// THE RULE THIS GUARDS (roadmap row 124, part 3): moving away needs a
// fresh proof of who one is and a new account that names this one; then
// the account says where it went, its followers' servers are sent a Move
// and its description, its followers here follow the new account, and
// the profile points there.
func TestMovingAwayTellsEverybodyAndPointsTheWay(t *testing.T) {
	m := newMovingWorld(nil)

	if rec := m.move(naomiNew); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "step_up_required") {
		t.Fatalf("without step-up: %d %s", rec.Code, rec.Body.String())
	}

	m.proved()
	rec := m.move(naomiNew)
	if rec.Code != http.StatusOK {
		t.Fatalf("move: %d %s", rec.Code, rec.Body.String())
	}
	if m.accounts.moved[anaID] != naomiNew {
		t.Errorf("the account does not say where it went: %q", m.accounts.moved[anaID])
	}
	newer, _ := m.h.Nodes.ReadByURI(naomiNew)
	if newer == nil || !m.has(constants.EdgeMovedEntityType, anaID, newer.ID) {
		t.Fatalf("the profile does not point to the new account")
	}
	if !m.has(constants.EdgeFollowEntityType, 1, newer.ID) || m.has(constants.EdgeFollowEntityType, 1, anaID) {
		t.Errorf("Amos was not moved")
	}

	var sent []string
	for _, q := range m.queue.sent {
		sent = append(sent, q.payload["type"].(string))
		if q.payload["type"] == "Move" && (q.payload["object"] != m.self || q.payload["target"] != naomiNew) {
			t.Errorf("the Move: %v", q.payload)
		}
		if q.payload["type"] == "Update" {
			object, _ := q.payload["object"].(map[string]any)
			if object["movedTo"] != naomiNew {
				t.Errorf("the description does not say where it went: %v", object["movedTo"])
			}
		}
	}
	if strings.Join(sent, ",") != "Update,Move" {
		t.Errorf("sent: %v", sent)
	}

	// Once moved, not again, and not off the fediverse.
	m.proved()
	if rec := m.move(naomiNew); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "moved_already") {
		t.Errorf("moved twice: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMovingAwayToAnAccountThatDoesNotNameThisOneIsRefused(t *testing.T) {
	m := newMovingWorld([]string{"https://elsewhere.example/users/someone"})
	m.proved()
	rec := m.move(naomiNew)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "not_named_there") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if m.accounts.moved[anaID] != "" || len(m.queue.sent) > 0 {
		t.Errorf("something was done")
	}
}

func TestMovingAwayToAnAccountHereIsRefused(t *testing.T) {
	m := newMovingWorld(nil)
	m.proved()
	rec := m.move("amos@example.org")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), errLookupOurs) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

// Moving in, and moving back: a Move from elsewhere to Ana is heeded when
// Ana names the old account; to an account that had moved away from
// there, it takes the signpost down.
func TestAMoveToSomebodyHereIsHeededWhenTheyNameTheOldAccount(t *testing.T) {
	m := newMovingWorld(nil)
	m.proved()
	if rec := m.move(naomiNew); rec.Code != http.StatusOK {
		t.Fatalf("move: %d %s", rec.Code, rec.Body.String())
	}
	newer, _ := m.h.Nodes.ReadByURI(naomiNew)
	m.h.Actors.Save(&repositories.RemoteActor{NodeID: newer.ID, URI: naomiNew})

	back := func() {
		activity := &ap.Activity{Type: "Move"}
		activity.Object, _ = json.Marshal(naomiNew)
		activity.Target, _ = json.Marshal(m.self)
		if err := m.h.moved(context.Background(), &remote.Actor{ID: naomiNew}, activity); err != nil {
			t.Fatal(err)
		}
	}

	// Not named: nothing happens.
	back()
	if m.accounts.moved[anaID] != naomiNew {
		t.Fatalf("moved back without naming the account")
	}

	m.accounts.aliases = map[int64][]string{anaID: {naomiNew}}
	back()
	if m.accounts.moved[anaID] != "" {
		t.Errorf("the signpost is still up")
	}
	if m.has(constants.EdgeMovedEntityType, anaID, newer.ID) {
		t.Errorf("the profile still points away")
	}
	if !m.has(constants.EdgeFollowEntityType, 1, anaID) {
		t.Errorf("Amos did not come back")
	}
}

func TestAliasesAreFoundAndKept(t *testing.T) {
	m := newMovingWorld(nil)
	rec := m.call(m.h.EditAliases, http.MethodPut, "/", `{"also_known_as":["`+naomiNew+`"]}`, anaID)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if got := m.accounts.aliases[anaID]; len(got) != 1 || got[0] != naomiNew {
		t.Errorf("kept: %v", got)
	}
	if !strings.Contains(rec.Body.String(), "naomi@tycho.example") {
		t.Errorf("answered: %s", rec.Body.String())
	}

	rec = m.call(m.h.EditAliases, http.MethodPut, "/", `{"also_known_as":["amos@example.org"]}`, anaID)
	if rec.Code != http.StatusNotFound {
		t.Errorf("an account here was named: %d", rec.Code)
	}

	m.accounts.enabled[anaID] = false
	rec = m.call(m.h.EditAliases, http.MethodPut, "/", `{"also_known_as":[]}`, anaID)
	if rec.Code != http.StatusConflict {
		t.Errorf("off the fediverse: %d", rec.Code)
	}
}

// Coming back: the signpost comes down and the account is whole again; the
// followers stay where they went until the new account moves back.
func TestComingBackTakesTheSignpostDown(t *testing.T) {
	m := newMovingWorld(nil)
	if rec := m.call(m.h.ComeBack, http.MethodDelete, "/", "", anaID); rec.Code != http.StatusConflict {
		t.Errorf("came back without moving: %d", rec.Code)
	}
	m.proved()
	if rec := m.move(naomiNew); rec.Code != http.StatusOK {
		t.Fatalf("move: %d %s", rec.Code, rec.Body.String())
	}
	newer, _ := m.h.Nodes.ReadByURI(naomiNew)

	rec := m.call(m.h.ComeBack, http.MethodDelete, "/", "", anaID)
	if rec.Code != http.StatusOK {
		t.Fatalf("come back: %d %s", rec.Code, rec.Body.String())
	}
	if m.accounts.moved[anaID] != "" || m.has(constants.EdgeMovedEntityType, anaID, newer.ID) {
		t.Errorf("the signpost is still up")
	}
	if !m.has(constants.EdgeFollowEntityType, 1, newer.ID) {
		t.Errorf("Amos was moved back without the new account moving")
	}
}
