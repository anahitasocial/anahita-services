package permissions

import (
	"anahita-libs/cmd/api/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		Comments: permissions.NewCommentPermissions(),
	}
}

type Permissions struct {
	Comments permissions.CommentPermissionsInterface
}
