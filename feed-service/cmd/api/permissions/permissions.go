package permissions

import (
	"feed/config"
	"feed/data/services"
)

const (
	ErrInsufficientScope = "insufficient scope"

	// Feed errors
	ErrFeedEditNotAllowed   = "you are not allowed to edit this feed"
	ErrFeedDeleteNotAllowed = "you are not allowed to delete this feed"
	ErrFeedAdminNotAllowed  = "you are not allowed to administrate this feed"

	// Repost errors
	ErrRepostBrowseNotAllowed = "you are not allowed to browse this content"
	ErrRepostAddNotAllowed    = "you are not allowed to repost this content"
	ErrRepostDeleteNotAllowed = "you are not allowed to delete this repost"
	ErrRepostAdminNotAllowed  = "you are not allowed to administrate this repost"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
}

func NewPermissions(env *Env) *Permissions {
	return &Permissions{
		Feeds:   NewFeedPermissions(env),
		Reposts: NewRepostPermissions(env),
	}
}

type Permissions struct {
	Feeds   *feedPermissions
	Reposts *repostPermissions
}
