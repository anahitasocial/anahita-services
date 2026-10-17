package handlers

import (
	"net/http/httptest"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// fakeActors reads the one owner a list's posts are on.
type fakeActors struct {
	services.ActorsGRPCInterface
	owners map[int64]*entities.AnahitaNode
	reads  int
}

func (f *fakeActors) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	f.reads++
	return f.owners[id], nil
}

// A profile's list of notes said nothing of what the viewer may do with
// each, and the web app read that as "no": Quote was off on the author's
// own public notes. The list answers as the note's own page does.
func TestSetListPostFlagsAnswersForEachPost(t *testing.T) {
	const person = "node.actor.person-service.person.v1"

	owner := relNode(5, person)
	owner.Access = constants.AnahitaAccessPublic

	note := func(id int64) *entities.AnahitaNode {
		n := entities.NewAnahitaNode()
		n.ID = id
		n.Type = "node.medium.text-service.note.v1"
		n.Access = constants.AnahitaAccessPublic
		n.OwnerID = 5
		n.CreatedBy = 5
		n.Authorized = nil // as a list reads them
		return n
	}
	posts := []*entities.AnahitaNode{note(10), note(11), nil}

	viewer := entities.NewViewer()
	viewer.ID = 5
	viewer.Authenticated = true
	viewer.Scopes = []string{constants.OAuthScopeMediaWrite}

	actors := &fakeActors{owners: map[int64]*entities.AnahitaNode{5: owner}}
	relations := &fakeRelations{rels: map[int64]*entities.ActorRelation{}}
	c := echo.New().NewContext(httptest.NewRequest("GET", "/notes/5/", nil), httptest.NewRecorder())

	SetListPostFlags(c, actors, relations, posts, viewer)

	for _, post := range posts[:2] {
		if post.Authorized == nil || post.Authorized.Quote == nil {
			t.Fatalf("post %d: no answer on quoting", post.ID)
		}
		if !*post.Authorized.Quote {
			t.Errorf("post %d: its author may not quote their own public note", post.ID)
		}
	}
	if len(relations.asked) != 1 || relations.asked[0] != 5 {
		t.Errorf("relationships asked for %v, want the one owner [5]", relations.asked)
	}
	if actors.reads != 1 {
		t.Errorf("owner read %d times, want once for the list", actors.reads)
	}
}
