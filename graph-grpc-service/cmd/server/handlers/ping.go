package handlers

import (
	"anahita-libs/proto/typespb"
	"context"
	"log"
)

type pingHandler struct {
	typespb.TypesServiceServer
	env *Env
}

func NewPingHandler(env *Env) *pingHandler {
	return &pingHandler{env: env}
}

// Ping
func (h *pingHandler) Ping(ctx context.Context, req *typespb.PingRequest) (*typespb.PingResponse, error) {
	log.Printf("Received Ping request: %v", req)
	var err error

	return &typespb.PingResponse{Message: "Pong " + req.Message}, err
}
