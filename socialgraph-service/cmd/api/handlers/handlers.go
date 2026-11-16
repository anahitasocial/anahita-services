package handlers

import (
	"socialgraph/cmd/api/permissions"
	"socialgraph/config"
	"socialgraph/data/services"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
	Perms  *permissions.Permissions
}

type Handlers struct {
	Block         *blockHandler
	Mute          *muteHandler
	ServerBlock   *serverBlockHandler
	Follower      *followerHandler
	Leader        *leaderHandler
	Mutual        *mutualHandler
	FollowRequest *followRequestHandler
	Invite        *inviteHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Block:         NewBlockHandler(env),
		Mute:          NewMuteHandler(env),
		ServerBlock:   NewServerBlockHandler(env),
		Follower:      NewFollowerHandler(env),
		Leader:        NewLeaderHandler(env),
		Mutual:        NewMutualHandler(env),
		FollowRequest: NewFollowRequestHandler(env),
		Invite:        NewInviteHandler(env),
	}
}
