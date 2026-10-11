package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	an_services "anahita-libs/data/services"
	"anahita-libs/feeds"
	"anahita-libs/policy"
	"anahita-libs/validators"
	"feed/data/services"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// The endpoints of custom feeds, over a graph held in memory.
//
// The graph's own part, which posts a feed shows to whom, is tested against a
// real database beside its query. What is tested here is who may do what to
// a feed, and that the limits hold.

const (
	testAlice = 1
	testBob   = 2
	testAdmin = 9
)

// fakeGraph holds nodes and edges as the graph would, and answers the calls
// the handler makes. Anything else it is asked panics, on the nil interface
// it is built on.
type fakeGraph struct {
	an_services.GraphNodeGRPCInterface

	nodes  map[int64]*entities.AnahitaNode
	edges  []*entities.AnahitaEdge
	nextID int64
	// Who follows whom, for a feed shared with its owner's followers.
	follows map[int64][]int64
}

func newFakeGraph() *fakeGraph {
	return &fakeGraph{
		nodes:   map[int64]*entities.AnahitaNode{},
		nextID:  100,
		follows: map[int64][]int64{},
	}
}

func (g *fakeGraph) Read(id int64) (*entities.AnahitaNode, error) {
	node, ok := g.nodes[id]
	if !ok {
		return entities.NewAnahitaNode(), nil
	}
	copied := *node
	return &copied, nil
}

func (g *fakeGraph) Add(node *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	g.nextID++
	copied := *node
	copied.ID = g.nextID
	g.nodes[copied.ID] = &copied
	return &copied, nil
}

func (g *fakeGraph) Edit(node *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	copied := *node
	g.nodes[node.ID] = &copied
	return node, nil
}

func (g *fakeGraph) Delete(id int64) (bool, error) {
	delete(g.nodes, id)
	kept := g.edges[:0]
	for _, edge := range g.edges {
		if edge.NodeBID != id {
			kept = append(kept, edge)
		}
	}
	g.edges = kept
	return true, nil
}

func (g *fakeGraph) owned(req *an_services.GraphNodeBrowseRequest) []*entities.AnahitaNode {
	var list []*entities.AnahitaNode
	for _, node := range g.nodes {
		if node.Type == req.Types[0] && node.OwnerID == req.OwnerIDs[0] {
			list = append(list, node)
		}
	}
	return list
}

func (g *fakeGraph) Count(req *an_services.GraphNodeBrowseRequest) (uint64, error) {
	return uint64(len(g.owned(req))), nil
}

func (g *fakeGraph) Browse(req *an_services.GraphNodeBrowseRequest) ([]*entities.AnahitaNode, error) {
	return g.owned(req), nil
}

func (g *fakeGraph) BatchRead(req *an_services.GraphNodeBatchReadRequest) ([]*entities.AnahitaNode, error) {
	var list []*entities.AnahitaNode
	for _, id := range req.IDs {
		if node, ok := g.nodes[id]; ok {
			list = append(list, node)
		}
	}
	return list, nil
}

// FilterVisible judges a feed as the graph does: its owner sees it, and
// otherwise its access decides.
func (g *fakeGraph) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	var visible []int64
	for _, id := range ids {
		node, ok := g.nodes[id]
		if !ok {
			continue
		}
		switch {
		case node.OwnerID == viewer.ID && viewer.ID > 0,
			node.Access == constants.AnahitaAccessPublic,
			node.Access == constants.AnahitaAccessRegistered && viewer.ID > 0:
			visible = append(visible, id)
		case node.Access == constants.AnahitaAccessFollowers:
			for _, follower := range g.follows[node.OwnerID] {
				if follower == viewer.ID {
					visible = append(visible, id)
				}
			}
		}
	}
	return visible, nil
}

// fakeEdges is the same graph, asked for its edges.
type fakeEdges struct {
	an_services.GraphEdgeGRPCInterface
	graph *fakeGraph
}

func (g fakeEdges) BatchReadByFilters(req *an_services.GraphEdgeBatchReadByFiltersRequest) ([]*entities.AnahitaEdge, error) {
	var list []*entities.AnahitaEdge
	for _, edge := range g.graph.edges {
		if edge.NodeAID == req.NodeAID && edge.Type == req.Types[0] {
			list = append(list, edge)
		}
	}
	return list, nil
}

func (g fakeEdges) BatchDeleteByFilters(req *an_services.GraphEdgeBatchDeleteByFiltersRequest) error {
	// The handler must never ask for a person's edges without saying which.
	if req.NodeAID == 0 || len(req.Types) != 1 || req.Types[0] != feeds.EdgePinType || req.NodeBType != feeds.NodeType {
		panic(fmt.Sprintf("edges deleted without being narrowed to pins: %+v", req))
	}

	kept := g.graph.edges[:0]
	for _, edge := range g.graph.edges {
		if !(edge.NodeAID == req.NodeAID && edge.Type == feeds.EdgePinType) {
			kept = append(kept, edge)
		}
	}
	g.graph.edges = kept
	return nil
}

func (g fakeEdges) BatchAdd(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error) {
	g.graph.edges = append(g.graph.edges, edges...)
	return edges, nil
}

// pinsOf is the feeds on somebody's home page, in order.
func (g *fakeGraph) pinsOf(person int64) []int64 {
	places := map[int32]int64{}
	most := int32(0)
	for _, edge := range g.edges {
		if edge.NodeAID == person && edge.Type == feeds.EdgePinType {
			places[edge.Ordering] = edge.NodeBID
			if edge.Ordering > most {
				most = edge.Ordering
			}
		}
	}

	var ids []int64
	for place := int32(1); place <= most; place++ {
		if id, ok := places[place]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// fakeFiles signs nothing: no test here is about pictures.
type fakeFiles struct {
	an_services.FilesGRPCInterface
}

func (fakeFiles) BatchPresign(paths []string) ([]string, error) {
	return make([]string, len(paths)), nil
}

// fakeFeedReader records what the graph was asked to read a feed as.
type fakeFeedReader struct {
	owner  int64
	viewer int64
	asked  int
	err    error
}

func (r *fakeFeedReader) Browse(def *feeds.Definition, ownerID int64, before feeds.Cursor, limit uint64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	r.asked++
	r.owner = ownerID
	r.viewer = viewer.ID
	return []*entities.AnahitaNode{}, r.err
}

type feedTest struct {
	t       *testing.T
	graph   *fakeGraph
	reader  *fakeFeedReader
	handler *customFeedHandler
	echo    *echo.Echo
}

func newFeedTest(t *testing.T) *feedTest {
	t.Helper()

	graph := newFakeGraph()
	reader := &fakeFeedReader{}

	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())

	return &feedTest{
		t:      t,
		graph:  graph,
		reader: reader,
		echo:   e,
		handler: NewCustomFeedHandler(&Env{Srv: &services.Services{
			GraphNodeGRPC:  graph,
			GraphEdgeGRPC:  fakeEdges{graph: graph},
			FeedCustomGRPC: reader,
			FilesGRPC:      fakeFiles{},
		}}),
	}
}

func person(id int64) *entities.Viewer {
	viewer := entities.NewViewer()
	if id > 0 {
		viewer.ID = id
		viewer.Authenticated = true
		viewer.PersonType = "registered"
		viewer.Scopes = []string{constants.OAuthScopeMediaWrite}
	}
	if id == testAdmin {
		viewer.PersonType = constants.PersonTypeAdmin
	}
	return viewer
}

func withoutScopes(viewer *entities.Viewer) *entities.Viewer {
	viewer.Scopes = nil
	return viewer
}

// call runs one handler as viewer, with the feed's id in the path when one
// is given, and returns the status and the body.
func (f *feedTest) call(handler echo.HandlerFunc, method string, viewer *entities.Viewer, id int64, body string) (int, map[string]any) {
	f.t.Helper()

	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := f.echo.NewContext(req, rec)
	c.Set(constants.ViewerContextKey, viewer)
	if id > 0 {
		c.SetParamNames("id")
		c.SetParamValues(fmt.Sprint(id))
	}

	if err := handler(c); err != nil {
		f.t.Fatalf("handler: %v", err)
	}

	answer := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
			f.t.Fatalf("answer is not JSON: %s", rec.Body.String())
		}
	}

	return rec.Code, answer
}

const followingFeed = `{"name":"Mine","definition":{"sources":{"following":true}}}`

// make adds a feed as its owner and returns its id.
func (f *feedTest) make(owner int64, body string) int64 {
	f.t.Helper()

	status, answer := f.call(f.handler.Add, http.MethodPost, person(owner), 0, body)
	if status != http.StatusCreated {
		f.t.Fatalf("add answered %d %v", status, answer)
	}

	return int64(answer["id"].(float64))
}

func TestACustomFeedIsMadePrivateAndPutOnHome(t *testing.T) {
	f := newFeedTest(t)

	status, answer := f.call(f.handler.Add, http.MethodPost, person(testAlice), 0, followingFeed)
	if status != http.StatusCreated {
		t.Fatalf("answered %d %v", status, answer)
	}

	id := int64(answer["id"].(float64))
	stored := f.graph.nodes[id]
	if stored.Type != feeds.NodeType || stored.OwnerID != testAlice || stored.Access != constants.AnahitaAccessMyself {
		t.Errorf("stored as %q, owner %d, access %q", stored.Type, stored.OwnerID, stored.Access)
	}
	if def, err := feeds.Parse(stored.Meta); err != nil || !def.Sources.Following {
		t.Errorf("definition stored as %s (%v)", stored.Meta, err)
	}
	if answer["is_pinned"] != true || fmt.Sprint(f.graph.pinsOf(testAlice)) != fmt.Sprint([]int64{id}) {
		t.Errorf("not put on its maker's home page: %v, pins %v", answer["is_pinned"], f.graph.pinsOf(testAlice))
	}
	authorized := answer["authorized"].(map[string]any)
	if authorized["edit"] != true || authorized["delete"] != true {
		t.Errorf("its maker may not change it: %v", authorized)
	}
}

func TestACustomFeedIsRefused(t *testing.T) {
	cases := map[string]struct {
		viewer *entities.Viewer
		body   string
		status int
		reason string
	}{
		"from a visitor":            {person(0), followingFeed, http.StatusForbidden, ""},
		"without the scope":         {withoutScopes(person(testAlice)), followingFeed, http.StatusForbidden, ""},
		"with no name":              {person(testAlice), `{"name":"  ","definition":{"sources":{"following":true}}}`, http.StatusUnprocessableEntity, "name_required"},
		"with no definition":        {person(testAlice), `{"name":"Mine"}`, http.StatusUnprocessableEntity, "malformed"},
		"of the whole site":         {person(testAlice), `{"name":"All","definition":{}}`, http.StatusUnprocessableEntity, "too_broad"},
		"with a key not known":      {person(testAlice), `{"name":"X","definition":{"sources":{"following":true},"sql":"1"}}`, http.StatusUnprocessableEntity, "unknown_field"},
		"with an audience not ours": {person(testAlice), `{"name":"X","access":"mutuals","definition":{"sources":{"following":true}}}`, http.StatusUnprocessableEntity, "invalid"},
	}

	for name, c := range cases {
		f := newFeedTest(t)
		status, answer := f.call(f.handler.Add, http.MethodPost, c.viewer, 0, c.body)
		if status != c.status || (c.reason != "" && answer["error"] != c.reason) {
			t.Errorf("%s: answered %d %v, want %d %q", name, status, answer, c.status, c.reason)
		}
		if len(f.graph.nodes) != 0 {
			t.Errorf("%s: a feed was stored all the same", name)
		}
	}
}

func TestNoNewPublicFeedWhereNothingNewIsPublic(t *testing.T) {
	restore := policy.SetForTest(constants.AnahitaAccessRegistered)
	defer restore()

	f := newFeedTest(t)

	status, answer := f.call(f.handler.Add, http.MethodPost, person(testAlice), 0,
		`{"name":"X","access":"public","definition":{"sources":{"following":true}}}`)
	if status != http.StatusUnprocessableEntity || answer["error"] != "access_not_allowed" {
		t.Errorf("a public feed answered %d %v", status, answer)
	}

	status, _ = f.call(f.handler.Add, http.MethodPost, person(testAlice), 0,
		`{"name":"X","access":"registered","definition":{"sources":{"following":true}}}`)
	if status != http.StatusCreated {
		t.Errorf("a feed for anyone signed in answered %d", status)
	}
}

func TestAPersonOwnsOnlySoManyFeeds(t *testing.T) {
	f := newFeedTest(t)

	for i := 0; i < feeds.MaxOwned; i++ {
		f.make(testAlice, followingFeed)
	}

	status, answer := f.call(f.handler.Add, http.MethodPost, person(testAlice), 0, followingFeed)
	if status != http.StatusConflict || answer["error"] != "too_many_feeds" {
		t.Errorf("one more answered %d %v", status, answer)
	}

	// The home page was full long before, and the feeds were made anyway.
	if got := len(f.graph.pinsOf(testAlice)); got != feeds.MaxPinned {
		t.Errorf("%d feeds on the home page, want %d", got, feeds.MaxPinned)
	}

	// Somebody else has their own count.
	if status, _ := f.call(f.handler.Add, http.MethodPost, person(testBob), 0, followingFeed); status != http.StatusCreated {
		t.Errorf("Bob's first answered %d", status)
	}
}

func TestWhoMaySeeChangeAndDeleteAFeed(t *testing.T) {
	f := newFeedTest(t)
	f.graph.follows[testAlice] = []int64{testBob}

	onlyMe := f.make(testAlice, followingFeed)
	forFollowers := f.make(testAlice, `{"name":"F","access":"followers","definition":{"sources":{"following":true}}}`)
	forAll := f.make(testAlice, `{"name":"P","access":"public","definition":{"sources":{"following":true}}}`)

	const carol = 3

	reads := []struct {
		what   string
		feed   int64
		viewer int64
		want   int
	}{
		{"its owner, her private one", onlyMe, testAlice, http.StatusOK},
		{"somebody else, a private one", onlyMe, testBob, http.StatusNotFound},
		{"a visitor, a private one", onlyMe, 0, http.StatusNotFound},
		{"an administrator, a private one", onlyMe, testAdmin, http.StatusOK},
		{"a follower, one for followers", forFollowers, testBob, http.StatusOK},
		{"somebody who does not follow, one for followers", forFollowers, carol, http.StatusNotFound},
		{"a visitor, a public one", forAll, 0, http.StatusOK},
		{"anybody, one that is not there", 9999, testBob, http.StatusNotFound},
	}
	for _, c := range reads {
		for name, handler := range map[string]echo.HandlerFunc{"read": f.handler.Read, "items": f.handler.Items} {
			if status, _ := f.call(handler, http.MethodGet, person(c.viewer), c.feed, ""); status != c.want {
				t.Errorf("%s by %s: answered %d, want %d", name, c.what, status, c.want)
			}
		}
	}

	// Seeing a feed is not being able to change it.
	edit := `{"name":"Taken","definition":{"sources":{"following":true}}}`
	if status, _ := f.call(f.handler.Edit, http.MethodPatch, person(testBob), forAll, edit); status != http.StatusForbidden {
		t.Errorf("somebody else changing a public feed answered %d", status)
	}
	if status, _ := f.call(f.handler.Delete, http.MethodDelete, person(testBob), forAll, ""); status != http.StatusForbidden {
		t.Errorf("somebody else deleting a public feed answered %d", status)
	}
	// One they may not see is not there to change.
	if status, _ := f.call(f.handler.Edit, http.MethodPatch, person(testBob), onlyMe, edit); status != http.StatusNotFound {
		t.Errorf("somebody else changing a private feed answered %d", status)
	}
	if f.graph.nodes[forAll].Name != "P" || f.graph.nodes[onlyMe] == nil {
		t.Errorf("a feed was changed by somebody it is not theirs")
	}
	_, read := f.call(f.handler.Read, http.MethodGet, person(testBob), forAll, "")
	if authorized := read["authorized"].(map[string]any); authorized["edit"] != false || authorized["pin"] != true || authorized["copy"] != true {
		t.Errorf("what a reader is told they may do: %v", authorized)
	}

	// Its owner changes it: the name, the audience, what it shows.
	status, answer := f.call(f.handler.Edit, http.MethodPatch, person(testAlice), onlyMe,
		`{"name":"Gardens","access":"public","definition":{"filters":{"hashtags_any":["#Garden"]}}}`)
	if status != http.StatusOK {
		t.Fatalf("its owner changing it answered %d %v", status, answer)
	}
	stored := f.graph.nodes[onlyMe]
	def, _ := feeds.Parse(stored.Meta)
	if stored.Name != "Gardens" || stored.Access != constants.AnahitaAccessPublic || fmt.Sprint(def.Filters.Hashtags) != "[garden]" {
		t.Errorf("stored as %q %q %s", stored.Name, stored.Access, stored.Meta)
	}

	// A change that is refused changes nothing. The audience is kept when
	// none is sent.
	if status, answer := f.call(f.handler.Edit, http.MethodPatch, person(testAlice), onlyMe, `{"name":"All","definition":{}}`); status != http.StatusUnprocessableEntity || answer["error"] != "too_broad" {
		t.Errorf("a feed of everything answered %d %v", status, answer)
	}
	f.call(f.handler.Edit, http.MethodPatch, person(testAlice), forFollowers, edit)
	if f.graph.nodes[onlyMe].Name != "Gardens" || f.graph.nodes[forFollowers].Access != constants.AnahitaAccessFollowers {
		t.Errorf("after a refused change and one with no audience: %q, %q", f.graph.nodes[onlyMe].Name, f.graph.nodes[forFollowers].Access)
	}

	// Deleted by its owner, it is gone from every home page.
	f.call(f.handler.Pin, http.MethodPut, person(testBob), forAll, "")
	if status, _ := f.call(f.handler.Delete, http.MethodDelete, person(testAlice), forAll, ""); status != http.StatusNoContent {
		t.Errorf("its owner deleting it answered %d", status)
	}
	if f.graph.nodes[forAll] != nil || len(f.graph.pinsOf(testBob)) != 0 {
		t.Errorf("deleted feed still there, or still on a home page: %v", f.graph.pinsOf(testBob))
	}
}

// A feed is read as its owner's, by whoever is reading: "whoever I follow"
// is whoever the owner follows, and the graph is told who the reader is.
func TestAFeedIsReadAsItsOwnersForTheReader(t *testing.T) {
	f := newFeedTest(t)
	feed := f.make(testAlice, `{"name":"P","access":"public","definition":{"sources":{"following":true}}}`)

	if status, _ := f.call(f.handler.Items, http.MethodGet, person(testBob), feed, ""); status != http.StatusOK {
		t.Fatalf("answered %d", status)
	}
	if f.reader.owner != testAlice || f.reader.viewer != testBob {
		t.Errorf("read as owner %d for viewer %d", f.reader.owner, f.reader.viewer)
	}

	// One not saved yet is its builder's own.
	status, _ := f.call(f.handler.Preview, http.MethodPost, person(testBob), 0, `{"definition":{"sources":{"following":true}}}`)
	if status != http.StatusOK || f.reader.owner != testBob || f.reader.viewer != testBob {
		t.Errorf("preview answered %d, read as owner %d for viewer %d", status, f.reader.owner, f.reader.viewer)
	}

	// Refused before anything is read.
	asked := f.reader.asked
	if status, answer := f.call(f.handler.Preview, http.MethodPost, person(testBob), 0, `{"definition":{}}`); status != http.StatusUnprocessableEntity || answer["error"] != "too_broad" {
		t.Errorf("a preview of everything answered %d %v", status, answer)
	}
	if status, _ := f.call(f.handler.Preview, http.MethodPost, person(0), 0, `{"definition":{"sources":{"following":true}}}`); status != http.StatusForbidden {
		t.Errorf("a visitor's preview answered %d", status)
	}
	if f.reader.asked != asked {
		t.Errorf("the graph was asked for a feed that was refused")
	}

	// Too much to read in time is said as such.
	f.reader.err = an_services.ErrFeedTooBroad
	if status, answer := f.call(f.handler.Items, http.MethodGet, person(testBob), feed, ""); status != http.StatusUnprocessableEntity || answer["error"] != "too_broad" {
		t.Errorf("a feed too broad to read answered %d %v", status, answer)
	}
}

func TestACopyIsTheCopiersOwnAndPrivate(t *testing.T) {
	f := newFeedTest(t)
	source := f.make(testAlice, `{"name":"Gardens","body":"What grows","access":"public","definition":{"filters":{"hashtags_any":["garden"]}}}`)
	hidden := f.make(testAlice, followingFeed)

	status, answer := f.call(f.handler.Copy, http.MethodPost, person(testBob), source, "")
	if status != http.StatusCreated {
		t.Fatalf("answered %d %v", status, answer)
	}

	copied := f.graph.nodes[int64(answer["id"].(float64))]
	if copied.ID == source || copied.OwnerID != testBob || copied.Access != constants.AnahitaAccessMyself || copied.Name != "Gardens" {
		t.Errorf("copy is %d, owner %d, access %q, name %q", copied.ID, copied.OwnerID, copied.Access, copied.Name)
	}
	if string(copied.Meta) != string(f.graph.nodes[source].Meta) {
		t.Errorf("copy shows %s, not %s", copied.Meta, f.graph.nodes[source].Meta)
	}

	if status, _ := f.call(f.handler.Copy, http.MethodPost, person(testBob), hidden, ""); status != http.StatusNotFound {
		t.Errorf("copying a feed that may not be seen answered %d", status)
	}
}

func TestFeedsOnTheHomePage(t *testing.T) {
	f := newFeedTest(t)
	bob := person(testBob)

	var shared []int64
	for i := 0; i < feeds.MaxPinned+1; i++ {
		shared = append(shared, f.make(testAlice, `{"name":"P","access":"public","definition":{"sources":{"following":true}}}`))
	}
	hidden := f.make(testAlice, followingFeed)

	// Added one at a time, each after the last, and twice is once.
	for _, id := range []int64{shared[0], shared[1], shared[0]} {
		if status, _ := f.call(f.handler.Pin, http.MethodPut, bob, id, ""); status != http.StatusNoContent {
			t.Fatalf("adding %d answered %d", id, status)
		}
	}
	if got := fmt.Sprint(f.graph.pinsOf(testBob)); got != fmt.Sprint(shared[:2]) {
		t.Errorf("home page holds %s", got)
	}

	// One that may not be seen is not added, by either way in.
	if status, _ := f.call(f.handler.Pin, http.MethodPut, bob, hidden, ""); status != http.StatusNotFound {
		t.Errorf("adding a hidden feed answered %d", status)
	}
	if status, answer := f.call(f.handler.SetPins, http.MethodPut, bob, 0, fmt.Sprintf(`{"ids":[%d,%d]}`, shared[0], hidden)); status != http.StatusUnprocessableEntity || answer["error"] != "feed_not_found" {
		t.Errorf("setting a hidden feed answered %d %v", status, answer)
	}
	if status, _ := f.call(f.handler.Pin, http.MethodPut, person(0), shared[0], ""); status != http.StatusForbidden {
		t.Errorf("a visitor adding a feed answered %d", status)
	}

	// Put in another order, repeats dropped.
	status, answer := f.call(f.handler.SetPins, http.MethodPut, bob, 0, fmt.Sprintf(`{"ids":[%d,%d,%d]}`, shared[1], shared[0], shared[1]))
	if status != http.StatusOK || fmt.Sprint(f.graph.pinsOf(testBob)) != fmt.Sprint([]int64{shared[1], shared[0]}) {
		t.Errorf("reordering answered %d, home page holds %v", status, f.graph.pinsOf(testBob))
	}
	if list := answer["data"].([]any); len(list) != 2 || int64(list[0].(map[string]any)["id"].(float64)) != shared[1] {
		t.Errorf("sent back as %v", answer["data"])
	}

	// Full at ten.
	var ten []string
	for _, id := range shared[:feeds.MaxPinned] {
		ten = append(ten, fmt.Sprint(id))
	}
	f.call(f.handler.SetPins, http.MethodPut, bob, 0, `{"ids":[`+strings.Join(ten, ",")+`]}`)
	if status, answer := f.call(f.handler.Pin, http.MethodPut, bob, shared[feeds.MaxPinned], ""); status != http.StatusConflict || answer["error"] != "too_many_pins" {
		t.Errorf("an eleventh answered %d %v", status, answer)
	}
	if status, _ := f.call(f.handler.SetPins, http.MethodPut, bob, 0, `{"ids":[`+strings.Join(append(ten, fmt.Sprint(shared[feeds.MaxPinned])), ",")+`]}`); status != http.StatusUnprocessableEntity {
		t.Errorf("setting eleven answered %d", status)
	}

	// A feed its owner stops sharing drops off the list, and can still be
	// taken off.
	gone := shared[0]
	f.graph.nodes[gone].Access = constants.AnahitaAccessMyself
	_, listed := f.call(f.handler.Pinned, http.MethodGet, bob, 0, "")
	if got := len(listed["data"].([]any)); got != feeds.MaxPinned-1 {
		t.Errorf("%d feeds listed after one stopped being shared", got)
	}
	if status, _ := f.call(f.handler.Unpin, http.MethodDelete, bob, gone, ""); status != http.StatusNoContent {
		t.Errorf("taking it off answered %d", status)
	}
	if got := len(f.graph.pinsOf(testBob)); got != feeds.MaxPinned-1 {
		t.Errorf("%d feeds on the home page after taking one off", got)
	}

	// Alice's own home page was never touched by any of this.
	if got := len(f.graph.pinsOf(testAlice)); got != feeds.MaxPinned {
		t.Errorf("Alice's home page holds %d", got)
	}
}
