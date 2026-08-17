package permissions

import (
	"anahita-libs/data/entities"
	"socialgraph/config"
	"socialgraph/data/services"
)

const (
	ErrInsufficientScope             = "insufficient scope"
	ErrFollowersBrowseNotAllowed     = "you are not allowed to browse followers"
	ErrFollowersAddNotAllowed        = "you are not allowed to follow"
	ErrFollowersDeleteNotAllowed     = "you are not allowed to unfollow"
	ErrFollowRequestBrowseNotAllowed = "you are not allowed to browse follow requests"
	ErrFollowRequestEditNotAllowed   = "you are not allowed to edit this follow request"
	ErrFollowRequestAddNotAllowed    = "you are not allowed to send a follow request"
	ErrFollowRequestDeleteNotAllowed = "you are not allowed to delete this follow request"
	ErrBlocksBrowseNotAllowed        = "you are not allowed to browse blocked people"
	ErrBlocksAddNotAllowed           = "you are not allowed to block"
	ErrBlocksDeleteNotAllowed        = "you are not allowed to unblock"
	ErrLeadersBrowseNotAllowed       = "you are not allowed to browse leaders"
	ErrMutualsBrowseNotAllowed       = "you are not allowed to browse mutuals"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
}

type PermissionsInterface interface {
	CanBrowse(actor *entities.AnahitaNode, viewer *entities.Viewer) (bool, error)
	CanEdit(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error)
	CanAdd(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error)
	CanDelete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error)
}

func NewPermissions(env *Env) *Permissions {
	return &Permissions{
		Followers:      newFollowersPermissions(env),
		Leaders:        newLeadersPermissions(env),
		Blocks:         newBlocksPermissions(env),
		Mutuals:        newMutualsPermissions(env),
		FollowRequests: newFollowRequestsPermissions(env),
	}
}

type Permissions struct {
	Followers      PermissionsInterface
	Leaders        PermissionsInterface
	Blocks         PermissionsInterface
	Mutuals        PermissionsInterface
	FollowRequests PermissionsInterface
}
