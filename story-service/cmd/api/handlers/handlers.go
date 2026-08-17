package handlers

import (
	"story/config"
)

type Env struct {
	Config *config.Config
}

type Handlers struct {
	Story *storyHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Story: NewStoryHandler(env),
	}
}
