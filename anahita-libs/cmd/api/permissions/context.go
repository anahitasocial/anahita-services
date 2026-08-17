package permissions

import "anahita-libs/data/entities"

type ActorAccessContext struct {
	Viewer         *entities.Viewer
	Actor          ActorNodeInterface
	ActorViewerRel *entities.ActorRelation
	AdminViewerRel *entities.ActorRelation
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
