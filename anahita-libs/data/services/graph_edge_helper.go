package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func grpcToGraphEdges(edges []*graphpb.Edge) []*entities.AnahitaEdge {
	var result []*entities.AnahitaEdge
	for _, e := range edges {
		result = append(result, grpcToGraphEdge(e))
	}
	return result
}

func grpcToGraphEdge(e *graphpb.Edge) *entities.AnahitaEdge {
	edge := &entities.AnahitaEdge{
		ID:         e.Id,
		Type:       e.Type,
		NodeAID:    e.NodeAId,
		NodeAType:  e.NodeAType,
		NodeBID:    e.NodeBId,
		NodeBType:  e.NodeBType,
		Meta:       e.Meta,
		CreatedBy:  e.CreatedBy,
		CreatedOn:  e.CreatedOn.AsTime(),
		ModifiedBy: e.ModifiedBy,
		ModifiedOn: e.ModifiedOn.AsTime(),
		Ordering:   e.Ordering,
		StartDate:  e.StartDate.AsTime(),
		EndDate:    e.EndDate.AsTime(),
	}

	return edge
}

func graphEdgeToGrpc(edge *entities.AnahitaEdge) *graphpb.Edge {
	grpcEdge := &graphpb.Edge{
		Id:         edge.ID,
		Type:       edge.Type,
		NodeAId:    edge.NodeAID,
		NodeAType:  edge.NodeAType,
		NodeBId:    edge.NodeBID,
		NodeBType:  edge.NodeBType,
		Meta:       edge.Meta,
		CreatedBy:  edge.CreatedBy,
		ModifiedBy: edge.ModifiedBy,
		Ordering:   edge.Ordering,
		StartDate:  timestamppb.New(edge.StartDate),
		EndDate:    timestamppb.New(edge.EndDate),
	}
	return grpcEdge
}
