package permissions

import "anahita-libs/data/entities"

type ActorAccessContext struct {
	Viewer         *entities.Viewer
	Actor          ActorNodeInterface
	ActorViewerRel *entities.ActorRelation
	AdminViewerRel *entities.ActorRelation

	// CreateLevel is the minimum role that may create an actor, as the
	// operator configured it — GROUPS_FROM, resolved through the service's
	// own config so the default is applied before it gets here.
	//
	// Only CanAdd reads it, and only group-service sets it. Left empty it
	// ranks above every person type and CanAdd refuses everybody, which is
	// the safe direction for a field a caller forgot.
	CreateLevel string
}

type MediumAccessContext struct {
	Viewer          *entities.Viewer
	Medium          MediumNodeInterface
	OwnerViewerRel  *entities.ActorRelation
	AuthorViewerRel *entities.ActorRelation
}

type CommentAccessContext struct {
	Viewer               *entities.Viewer
	Comment              CommentNodeInterface
	ParentOwnerViewerRel *entities.ActorRelation
	AuthorViewerRel      *entities.ActorRelation
}
