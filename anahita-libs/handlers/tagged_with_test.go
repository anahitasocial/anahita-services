package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/permissions"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type taggedPerms struct {
	permissions.TaggablePermissionsInterface
	may bool
}

func (p taggedPerms) CanBrowse(*entities.Viewer) (bool, error) { return p.may, nil }

type taggedNodes struct {
	services.GraphNodeGRPCInterface
}

func (taggedNodes) Read(id int64) (*entities.AnahitaNode, error) {
	if id != 7 {
		return nil, errors.New("no such node")
	}
	n := entities.NewAnahitaNode()
	n.ID = 7
	return n, nil
}

type taggedGraph struct {
	services.GraphGRPCInterface
	asked *services.GraphBrowseConnectionNodesRequest
}

func (g *taggedGraph) BrowseConnectionNodes(req *services.GraphBrowseConnectionNodesRequest) ([]*entities.AnahitaNode, uint64, error) {
	g.asked = req
	post := entities.NewAnahitaNode()
	post.ID = 12
	return []*entities.AnahitaNode{post}, 3, nil
}

type taggedFiles struct{ services.FilesGRPCInterface }

func (taggedFiles) BatchPresign(paths []string) ([]string, error) { return paths, nil }

type taggedMedia struct{ services.MediaGRPCInterface }

func (taggedMedia) BrowsePhotoFiles([]int64) ([]*entities.PhotoFile, error) { return nil, nil }

// THE RULE THIS GUARDS (roadmap row 170): the posts a hashtag or a location
// is on are listed alike, those the viewer may see, by the edge of its
// kind; refused to somebody who may not browse, and a tag that is not there
// is not found.
func TestThePostsATagIsOnAreListedAlike(t *testing.T) {
	graph := &taggedGraph{}
	readers := func(may bool) TagReaders {
		return TagReaders{Perms: taggedPerms{may: may}, Nodes: taggedNodes{}, Graph: graph, Files: taggedFiles{}, Media: taggedMedia{}}
	}
	ask := func(r TagReaders, id string) *httptest.ResponseRecorder {
		e := echo.New()
		e.Validator = validators.NewRequestValidator(validator.New())
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
		c.SetParamNames("id")
		c.SetParamValues(id)
		_ = TaggedWith(c, r, constants.EdgeTagLocationType)
		return rec
	}

	if rec := ask(readers(false), "7"); rec.Code != http.StatusForbidden {
		t.Errorf("somebody who may not browse: %d", rec.Code)
	}
	if rec := ask(readers(true), "8"); rec.Code != http.StatusNotFound {
		t.Errorf("a tag that is not there: %d", rec.Code)
	}

	rec := ask(readers(true), "7")
	var page struct {
		Data       []struct{ ID int64 } `json:"data"`
		Pagination struct{ Total uint64 }
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil || len(page.Data) != 1 || page.Pagination.Total != 3 {
		t.Fatalf("a page: %d %s", rec.Code, rec.Body.String())
	}
	if a := graph.asked; a.NodeId != 7 || a.EdgeType != constants.EdgeTagLocationType || !a.AccessCheck || a.Direction != constants.GraphQueryDirectionOutbound {
		t.Errorf("asked the graph %+v", a)
	}
}
