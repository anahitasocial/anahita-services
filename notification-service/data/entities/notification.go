package entities

import (
	"anahita-libs/data/entities"
	"errors"
	"time"
)

// Validate the notification
const (
	ErrInvalidType        = "invalid type"
	ErrInvalidService     = "invalid service"
	ErrInvalidRecipientID = "invalid recipient id"
	ErrInvalidSubjectID   = "invalid subject id"
	ErrInvalidObjectID    = "invalid object id"
	ErrInvalidCommentID   = "invalid comment id"
	ErrInvalidTargetID    = "invalid target id"
	ErrInvalidSubjectType = "invalid subject type"
	ErrInvalidObjectType  = "invalid object type"
	ErrInvalidTargetType  = "invalid target type"
)

type Notification struct {
	ID          int64
	Type        string
	Service     string
	RecipientID int64
	Subject     *entities.AnahitaNode
	Object      *entities.AnahitaNode
	Target      *entities.AnahitaNode
	CommentID   int64
	// What the reader can do from it, worked out when it is read: never
	// stored (handlers/actions.go).
	Actions   []string
	CreatedAt time.Time
	UpdatedAt time.Time
	IsRead    bool
}

func NewNotification() *Notification {
	n := &Notification{}
	n.IsRead = false
	n.Subject = entities.NewAnahitaNode()
	n.Target = entities.NewAnahitaNode()
	n.Object = entities.NewAnahitaNode()

	return n
}

// Validate the notification
// Type required
// Service required
// RecipientID required
// SubjectID required
// SubjectType required
// ObjectID optional
// ObjectType required if ObjectID is set
// CommentID optional, but if set, ObjectID must be set and ObjectType must be set
// TargetID required
// TargetType required
func (n *Notification) Validate() error {
	if n.Type == "" {
		return errors.New(ErrInvalidType)
	}

	if n.Service == "" {
		return errors.New(ErrInvalidService)
	}

	if n.RecipientID == 0 {
		return errors.New(ErrInvalidRecipientID)
	}

	if n.Subject.ID == 0 {
		return errors.New(ErrInvalidSubjectID)
	}

	if n.Subject.Type == "" {
		return errors.New(ErrInvalidSubjectType)
	}

	if n.Object.ID > 0 && n.Object.Type == "" {
		return errors.New(ErrInvalidObjectType)
	}

	if n.CommentID > 0 && n.Object.ID == 0 {
		return errors.New(ErrInvalidObjectID)
	}

	if n.CommentID > 0 && n.Object.Type == "" {
		return errors.New(ErrInvalidObjectType)
	}

	if n.Target.ID == 0 {
		return errors.New(ErrInvalidTargetID)
	}

	if n.Target.Type == "" {
		return errors.New(ErrInvalidTargetType)
	}

	return nil
}
