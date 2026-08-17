package handlers

import (
	"file/config"
	"file/data/services"
)

type Env struct {
	Config   *config.Config
	Services *services.Services
}

type Handlers struct {
	File *fileHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		File: NewFileHandler(env),
	}
}
