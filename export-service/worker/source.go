package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"export/archive"
)

// GraphSource reads a person's data through the graph, as that person:
// what they follow and who follows them are the lists they see themselves.
type GraphSource struct {
	Actors  services.ActorsGRPCInterface
	Written services.AuthoredInterface
	Media   services.MediaGRPCInterface
	Graph   services.GraphGRPCInterface
}

// as is the person, for the calls that ask who is looking.
func as(personID int64) *entities.Viewer {
	viewer := entities.NewViewer()
	viewer.ID = personID
	viewer.Authenticated = true
	viewer.Enabled = true
	viewer.PersonType = constants.PersonTypeRegistered
	return viewer
}

func (g *GraphSource) Profile(personID int64) (*entities.AnahitaNode, error) {
	return g.Actors.ReadOfType(personID, constants.NodeActorPersonEntityType, as(personID))
}

func (g *GraphSource) Authored(personID, afterID int64, limit uint64) ([]*entities.AnahitaNode, error) {
	return g.Written.ListAuthored(personID, afterID, limit)
}

func (g *GraphSource) PhotoFiles(postIDs []int64) ([]*entities.PhotoFile, error) {
	return g.Media.BrowsePhotoFiles(postIDs)
}

// Which edge each kind of connection is, and which way it runs from the
// person.
var connections = map[string]struct {
	edge      string
	direction string
}{
	archive.Following: {constants.EdgeFollowEntityType, constants.GraphQueryDirectionOutbound},
	archive.Followers: {constants.EdgeFollowEntityType, constants.GraphQueryDirectionInbound},
	archive.Blocked:   {constants.EdgeBlockEntityType, constants.GraphQueryDirectionOutbound},
	archive.Liked:     {constants.EdgeBaseLikeEntityType, constants.GraphQueryDirectionOutbound},
}

func (g *GraphSource) Connected(personID int64, kind string, offset uint64) ([]*entities.AnahitaNode, error) {
	connection, ok := connections[kind]
	if !ok {
		return nil, fmt.Errorf("no such connection: %s", kind)
	}

	req := services.NewGraphRequest().BrowseNeighbours
	req.NodeId = personID
	req.EdgeTypes = []string{connection.edge}
	req.Direction = connection.direction
	req.Offset = offset
	req.Limit = archive.PageSize
	req.Ordering = constants.OrderingConnected
	req.Viewer = as(personID)
	req.AccessCheck = true

	nodes, _, err := g.Graph.BrowseNeighbours(req)
	return nodes, err
}

func (g *GraphSource) Saved(personID int64, offset uint64) ([]*entities.AnahitaNode, error) {
	nodes, _, err := g.Media.Saved(archive.PageSize, offset, as(personID))
	return nodes, err
}

// StoredFiles opens files in storage, at addresses the file service signs
// for a service to read.
type StoredFiles struct {
	Transfers services.FileTransfersInterface
	Client    *http.Client
}

func (s *StoredFiles) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	address, err := s.Transfers.FetchURL(key)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			return nil, archive.ErrNoFile
		}
		return nil, fmt.Errorf("reading %s answered %d", key, res.StatusCode)
	}
	return res.Body, nil
}
