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
	Follower      *followerHandler
	Leader        *leaderHandler
	Mutual        *mutualHandler
	FollowRequest *followRequestHandler
	Invite        *inviteHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Block:         NewBlockHandler(env),
		Follower:      NewFollowerHandler(env),
		Leader:        NewLeaderHandler(env),
		Mutual:        NewMutualHandler(env),
		FollowRequest: NewFollowRequestHandler(env),
		Invite:        NewInviteHandler(env),
	}
}
