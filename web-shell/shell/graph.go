package shell

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

// GraphReader reads through the graph, as a visitor.
type GraphReader struct {
	Nodes  services.GraphNodeGRPCInterface
	Actors services.ActorsGRPCInterface
	Media  services.MediaGRPCInterface
}

func (g *GraphReader) Person(alias string) (*entities.AnahitaNode, error) {
	req := services.NewGraphNodeRequest().ReadByAlias
	req.Alias = alias
	req.NodeType = constants.NodeActorPersonEntityType

	// The alias gives the id and nothing more: the lookup takes no viewer,
	// so what it returns is not used.
	node, err := g.Nodes.ReadByAlias(req)
	if err != nil || node == nil || node.ID == 0 {
		return nil, err
	}

	return g.Actor(node.ID, constants.NodeActorPersonEntityType)
}

func (g *GraphReader) Actor(id int64, nodeType string) (*entities.AnahitaNode, error) {
	return g.Actors.ReadOfType(id, nodeType, entities.NewViewer())
}

func (g *GraphReader) Medium(id int64) (*entities.AnahitaNode, error) {
	return g.Media.Read(id, entities.NewViewer())
}

// FileSigner signs through file-grpc-service.
type FileSigner struct {
	Files services.FilesGRPCInterface
}

func (f *FileSigner) Sign(key string) (string, error) {
	req := services.NewFileRequest().Presign
	req.Path = key
	return f.Files.Presign(req)
}
