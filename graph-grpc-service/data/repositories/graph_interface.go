package repositories

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// GraphRepoInterface defines graph operations for navigating nodes and edges
// in the social graph. It supports 1-hop neighborhood queries, filtered
// connection queries, graph-based ranking, and multi-hop traversal.
type GraphRepoInterface interface {

	// BrowseNeighbours returns a paginated list of 1-hop neighbors for a given node.
	//
	// This method:
	//   - supports filtering by edge types and direction (IN, OUT, BOTH)
	//   - supports limit + offset pagination
	//   - performs permission checks based on the viewer
	//
	// Use this for building lists of related nodes, such as friends, followers,
	// group members, etc.
	BrowseNeighbours(req *graphpb.BrowseNeighboursRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// CountNeighbours returns the total number of 1-hop neighbors that match
	CountNeighbours(req *graphpb.BrowseNeighboursRequest, viewer *entities.Viewer) (uint64, error)

	// BrowseConnectionNodes returns the 1-hop neighbors connected by a single edge type.
	//
	// This mirrors BrowseNeighbours but is specialized for a single edge type.
	// It is useful for scenarios like fetching friends, followers, or members
	// of a specific group.
	BrowseConnectionNodes(req *graphpb.BrowseConnectionNodesRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// CountConnectionNodes returns the total number of 1-hop neighbors connected by a single edge type.
	//
	// This mirrors CountNeighbours but is specialized for a single edge type.
	// It is useful for scenarios like counting friends, followers, or members
	// of a specific group.
	CountConnectionNodes(req *graphpb.BrowseConnectionNodesRequest, viewer *entities.Viewer) (uint64, error)

	// BrowseOrderedByDegree returns nodes ordered by their degree (number of connections).
	//
	// This method:
	//   - supports filtering by node type
	//   - supports filtering by edge types and direction
	//   - supports limit + offset pagination
	//   - performs permission checks based on the viewer
	//
	// Use this for discovery features, popular nodes, or highly connected entities.
	BrowseOrderedByDegree(req *graphpb.BrowseOrderedByDegreeRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// ReadNeighbourhoods retrieves the full 1-hop neighborhood for a set of nodes.
	//
	// This method:
	//   - supports filtering by edge types and direction
	//   - returns all connected nodes and edges in a structured format
	//   - performs permission checks based on the viewer
	//
	// Use this for visualizing local graph structures, ego networks,
	// or detailed relationship views.
	ReadNeighbourhoods(req *graphpb.ReadNeighbourhoodRequest, viewer *entities.Viewer) (*graphpb.ReadNeighbourhoodResponse, error)

	// Traverse performs multi-hop traversals starting from a set of nodes.
	//
	// This method:
	//   - supports filtering by edge types and direction at each hop
	//   - allows specifying max depth and traversal strategies (BFS, DFS)
	//   - performs permission checks based on the viewer
	//
	// Use this for recommendations, pathfinding, or exploring extended
	// relationships in the graph.
	Traverse(req *graphpb.TraverseRequest, viewer *entities.Viewer) (*graphpb.TraverseResponse, error)

	// ReadShortestPath finds the shortest path between two nodes.
	//
	// This method:
	//   - supports filtering by edge types and direction
	//   - returns the sequence of nodes and edges forming the shortest path
	//   - performs permission checks based on the viewer
	//
	// Use this for connection discovery, relationship insights,
	// or path analysis within the graph.
	ReadShortestPath(req *graphpb.ReadShortestPathRequest, viewer *entities.Viewer) (*graphpb.ReadShortestPathResponse, error)

	// Trending returns nodes that have had the most activity in a given time window.
	//
	// This method:
	//   - supports filtering by node type, edge types, and direction
	//   - ranks nodes based on recent edge creation activity
	//   - performs permission checks based on the viewer
	//
	// Use this for discovery features, trending topics,
	// or highlighting active entities in the graph.
	Trending(req *graphpb.TrendingRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)
}
