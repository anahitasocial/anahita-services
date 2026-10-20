package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"actor/data/services"

	"anahita-libs/apidoc"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libs_services "anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// People in a picture or a video (roadmap row 103).

const (
	tagAuthorID  = int64(1) // wrote the photo
	tagPersonID  = int64(2) // never chose: approves each tag first
	tagAnybodyID = int64(3) // lets anybody tag them
	tagNobodyID  = int64(4) // lets nobody
	tagGroupID   = int64(5)
	tagPhotoID   = int64(100)
	tagNoteID    = int64(101)
	tagHiddenID  = int64(102) // a photo the viewer cannot see
)

type tagNodes struct {
	libs_services.GraphNodeGRPCInterface
	nodes map[int64]*entities.AnahitaNode
}

func (f *tagNodes) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	var visible []int64
	for _, id := range ids {
		if id != tagHiddenID {
			visible = append(visible, id)
		}
	}
	return visible, nil
}

func (f *tagNodes) Read(id int64) (*entities.AnahitaNode, error) {
	return f.nodes[id], nil
}

func (f *tagNodes) BatchRead(req *libs_services.GraphNodeBatchReadRequest) ([]*entities.AnahitaNode, error) {
	var list []*entities.AnahitaNode
	for _, id := range req.IDs {
		if n, ok := f.nodes[id]; ok {
			list = append(list, n)
		}
	}
	return list, nil
}

type tagEdges struct {
	libs_services.GraphEdgeGRPCInterface
	next  int64
	edges map[int64]*entities.AnahitaEdge
}

func (f *tagEdges) BatchReadByFilters(req *libs_services.GraphEdgeBatchReadByFiltersRequest) ([]*entities.AnahitaEdge, error) {
	var list []*entities.AnahitaEdge
	for _, e := range f.edges {
		if e.Type == req.Types[0] && e.NodeBID == req.NodeBID {
			copied := *e
			list = append(list, &copied)
		}
	}
	return list, nil
}

func (f *tagEdges) Add(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error) {
	f.next++
	edge.ID = f.next
	f.edges[edge.ID] = edge
	return edge, nil
}

func (f *tagEdges) Edit(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error) {
	f.edges[edge.ID] = edge
	return edge, nil
}

func (f *tagEdges) Delete(id int64) error {
	delete(f.edges, id)
	return nil
}

type tagFiles struct {
	libs_services.FilesGRPCInterface
}

func (f *tagFiles) BatchPresign(paths []string) ([]string, error) {
	return paths, nil
}

type tagSocialgraph struct {
	libs_services.SocialgraphGRPCInterface
}

func (f *tagSocialgraph) Read(a, b int64) (*entities.ActorRelation, error) {
	return &entities.ActorRelation{}, nil
}

func tagPerson(id int64, access string) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID, n.Type, n.Name = id, an_constants.NodeActorPersonEntityType, "Person"
	if access != "" {
		n.FeaturesRaw = []byte(`[{"service":"socialgraph-service","add_permissions":[{"entity":"tag","access":"` + access + `"}]}]`)
	}
	return n
}

type tagFixture struct {
	t     *testing.T
	h     *personHandler
	edges *tagEdges
}

func newTagFixture(t *testing.T) *tagFixture {
	photo := entities.NewAnahitaNode()
	photo.ID, photo.Type, photo.CreatedBy = tagPhotoID, an_constants.NodeMediumPhotoEntityType, tagAuthorID
	note := entities.NewAnahitaNode()
	note.ID, note.Type, note.CreatedBy = tagNoteID, "node.medium.text-service.note.v1", tagAuthorID
	group := entities.NewAnahitaNode()
	group.ID, group.Type = tagGroupID, an_constants.NodeActorGroupEntityType

	edges := &tagEdges{edges: map[int64]*entities.AnahitaEdge{}}
	env := &Env{Srv: &services.Services{
		GraphNodeGRPC: &tagNodes{nodes: map[int64]*entities.AnahitaNode{
			tagAuthorID:  tagPerson(tagAuthorID, ""),
			tagPersonID:  tagPerson(tagPersonID, ""),
			tagAnybodyID: tagPerson(tagAnybodyID, "registered"),
			tagNobodyID:  tagPerson(tagNobodyID, "admins"),
			tagGroupID:   group,
			tagPhotoID:   photo,
			tagNoteID:    note,
		}},
		GraphEdgeGRPC:   edges,
		SocialgraphGRPC: &tagSocialgraph{},
		FilesGRPC:       &tagFiles{},
	}}
	return &tagFixture{t: t, h: &personHandler{Env: env}, edges: edges}
}

func tagViewer(id int64) *entities.Viewer {
	v := entities.NewViewer()
	v.ID = id
	v.PersonType = an_constants.PersonTypeRegistered
	v.Authenticated = true
	return v
}

func (f *tagFixture) call(method, path string, viewer *entities.Viewer, params map[string]string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	f.t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(an_constants.ViewerContextKey, viewer)
	var names, values []string
	for name, value := range params {
		names = append(names, name)
		values = append(values, value)
	}
	c.SetParamNames(names...)
	c.SetParamValues(values...)
	if err := handler(c); err != nil {
		f.t.Fatalf("%s %s: %v", method, path, err)
	}
	apidoc.Conforms(f.t, "../../../../docs/api/actor-service.json", method+" "+path, rec)
	return rec
}

func (f *tagFixture) tag(viewer int64, person, post string) int {
	return f.call(http.MethodPost, "/people/"+person+"/tags/"+post, tagViewer(viewer),
		map[string]string{"id": person, "post_id": post}, f.h.AddTag).Code
}

func (f *tagFixture) taggedIn(viewer int64) []TaggedPerson {
	f.t.Helper()
	rec := f.call(http.MethodGet, "/people/tagged-in/100", tagViewer(viewer),
		map[string]string{"post_id": "100"}, f.h.TaggedIn)
	var list struct {
		Data []TaggedPerson `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		f.t.Fatalf("tagged-in: %s", rec.Body.String())
	}
	return list.Data
}

func TestAPersonIsTaggedAsTheyChose(t *testing.T) {
	f := newTagFixture(t)

	if got := f.tag(tagAuthorID, "2", "100"); got != http.StatusAccepted {
		t.Errorf("approval first: %d, want 202", got)
	}
	if got := f.tag(tagAuthorID, "3", "100"); got != http.StatusCreated {
		t.Errorf("anybody: %d, want 201", got)
	}
	if got := f.tag(tagAuthorID, "4", "100"); got != http.StatusConflict {
		t.Errorf("nobody: %d, want 409", got)
	}
	if got := f.tag(tagAuthorID, "5", "100"); got != http.StatusConflict {
		t.Errorf("a group: %d, want 409", got)
	}
	if got := f.tag(tagAuthorID, "1", "100"); got != http.StatusCreated {
		t.Errorf("themselves: %d, want 201", got)
	}
	// Only the author tags, only a photo or a video, only one they can see.
	if got := f.tag(tagPersonID, "3", "100"); got != http.StatusForbidden {
		t.Errorf("not the author: %d, want 403", got)
	}
	if got := f.tag(tagAuthorID, "3", "101"); got != http.StatusNotFound {
		t.Errorf("a note: %d, want 404", got)
	}
	if got := f.tag(tagAuthorID, "3", "102"); got != http.StatusNotFound {
		t.Errorf("hidden: %d, want 404", got)
	}
	// Tagging again changes nothing.
	if got := f.tag(tagAuthorID, "2", "100"); got != http.StatusAccepted || len(f.edges.edges) != 3 {
		t.Errorf("again: %d with %d edges", got, len(f.edges.edges))
	}
}

func TestATagWaitingIsSeenOnlyByTheTwoOfThem(t *testing.T) {
	f := newTagFixture(t)
	f.tag(tagAuthorID, "2", "100")
	f.tag(tagAuthorID, "3", "100")

	if got := f.taggedIn(tagNobodyID); len(got) != 1 {
		t.Errorf("a stranger sees %d, want the approved one", len(got))
	}
	for _, viewer := range []int64{tagAuthorID, tagPersonID} {
		got := f.taggedIn(viewer)
		if len(got) != 2 || !got[0].Pending || got[1].Pending {
			t.Errorf("viewer %d: %+v", viewer, got)
		}
	}
}

func TestTheTaggedApproveOrDeclineAndTheAuthorUntags(t *testing.T) {
	f := newTagFixture(t)
	f.tag(tagAuthorID, "2", "100")
	params := map[string]string{"id": "2", "post_id": "100"}

	// Nobody approves for them.
	if rec := f.call(http.MethodPut, "/people/2/tags/100", tagViewer(tagAuthorID), params, f.h.ApproveTag); rec.Code != http.StatusForbidden {
		t.Errorf("author approving: %d", rec.Code)
	}
	if rec := f.call(http.MethodPut, "/people/2/tags/100", tagViewer(tagPersonID), params, f.h.ApproveTag); rec.Code != http.StatusOK {
		t.Errorf("approving: %d", rec.Code)
	}
	if got := f.taggedIn(tagNobodyID); len(got) != 1 {
		t.Errorf("approved, a stranger sees %d", len(got))
	}

	// A stranger cannot take them out; they and the author can.
	if rec := f.call(http.MethodDelete, "/people/2/tags/100", tagViewer(tagNobodyID), params, f.h.RemoveTag); rec.Code != http.StatusForbidden {
		t.Errorf("stranger removing: %d", rec.Code)
	}
	if rec := f.call(http.MethodDelete, "/people/2/tags/100", tagViewer(tagPersonID), params, f.h.RemoveTag); rec.Code != http.StatusNoContent {
		t.Errorf("removing themselves: %d", rec.Code)
	}
	if len(f.edges.edges) != 0 {
		t.Errorf("still tagged: %d", len(f.edges.edges))
	}
	f.tag(tagAuthorID, "3", "100")
	if rec := f.call(http.MethodDelete, "/people/3/tags/100", tagViewer(tagAuthorID), map[string]string{"id": "3", "post_id": "100"}, f.h.RemoveTag); rec.Code != http.StatusNoContent || len(f.edges.edges) != 0 {
		t.Errorf("author untagging: %d", rec.Code)
	}
}
