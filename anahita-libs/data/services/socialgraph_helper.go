package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"time"
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
		HasInvite:    relation.HasInvite,
		ViaHost:      relation.ViaHost,

		IsMuting:               relation.IsMuting,
		MuteHidesNotifications: relation.MuteHidesNotifications,
		MuteEndsAt:             unixToTime(relation.MuteEndsAt),
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
		HasInvite:    relation.HasInvite,
		ViaHost:      relation.ViaHost,

		IsMuting:               relation.IsMuting,
		MuteHidesNotifications: relation.MuteHidesNotifications,
		MuteEndsAt:             timeToUnix(relation.MuteEndsAt),
	}
}

// unixToTime is a time sent as Unix seconds, 0 for none.
func unixToTime(seconds int64) time.Time {
	if seconds == 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}

// timeToUnix is a time as Unix seconds, 0 for none.
func timeToUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
