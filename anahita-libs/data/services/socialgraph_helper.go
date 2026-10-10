package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

func actorNodeToSocialgraphTarget(a *entities.AnahitaNode) *graphpb.Target {
	target := &graphpb.Target{
		Id:   a.ID,
		Type: a.Type,
	}

	return target
}

func actorNodeToSocialgraphSource(a *entities.AnahitaNode) *graphpb.Source {
	source := &graphpb.Source{
		Id:   a.ID,
		Type: a.Type,
	}

	return source
}

func GrpcToRelationEntity(relation *graphpb.Socialgraph) *entities.ActorRelation {
	return &entities.ActorRelation{
		IsLeading:    relation.IsLeading,
		IsFollowing:  relation.IsFollowing,
		IsMutual:     relation.IsMutual,
		IsBlocking:   relation.IsBlocking,
		HasAdmin:     relation.HasAdmin,
		HasRequested: relation.HasRequested,
	}
}

func RelationEntityToGrpc(relation *entities.ActorRelation) *graphpb.Socialgraph {
	return &graphpb.Socialgraph{
		IsLeading:    relation.IsLeading,
		IsFollowing:  relation.IsFollowing,
		IsMutual:     relation.IsMutual,
		IsBlocking:   relation.IsBlocking,
		HasAdmin:     relation.HasAdmin,
		HasRequested: relation.HasRequested,
	}
}
