package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func extractNodeIDs(nodes []*entities.AnahitaNode) []int64 {
	ids := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

func edgesToGrpc(edges []*entities.AnahitaEdge) []*graphpb.Edge {
	grpcEdges := make([]*graphpb.Edge, 0, len(edges))
	for _, edge := range edges {
		grpcEdges = append(grpcEdges, edgeToGrpc(edge))
	}
	return grpcEdges
}

func edgeToGrpc(edge *entities.AnahitaEdge) *graphpb.Edge {
	grpcEdge := &graphpb.Edge{
		Id:         edge.ID,
		Type:       edge.Type,
		NodeAId:    edge.NodeAID,
		NodeAType:  edge.NodeAType,
		NodeBId:    edge.NodeBID,
		NodeBType:  edge.NodeBType,
		Meta:       edge.Meta,
		CreatedAt:  timestamppb.New(edge.CreatedAt),
		CreatedBy:  edge.CreatedBy,
		UpdatedAt:  timestamppb.New(edge.UpdatedAt),
		ModifiedBy: edge.ModifiedBy,
		Ordering:   edge.Ordering,
		StartsAt:   timestamppb.New(edge.StartsAt),
		EndsAt:     timestamppb.New(edge.EndsAt),
	}
	return grpcEdge
}
