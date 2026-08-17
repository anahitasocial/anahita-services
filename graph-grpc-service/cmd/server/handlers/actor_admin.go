package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/proto/graphpb"
	"context"
	"fmt"
	"log"
	"strings"
)

type actorAdminHandler struct {
	graphpb.ActorAdminServiceServer
	env *Env
}

func NewActorAdminHandler(env *Env) *actorAdminHandler {
	return &actorAdminHandler{env: env}
}

// Browse returns a list of admins
func (h *actorAdminHandler) Browse(ctx context.Context, req *graphpb.ActorAdminBrowseRequest) (*graphpb.ActorAdminBrowseResponse, error) {
	log.Printf("Received Browse request: %v", req)
	var err error

	admins, err := h.env.Repos.ActorAdmins.Browse(req.ActorId)
	if err != nil {
		return nil, err
	}

	return &graphpb.ActorAdminBrowseResponse{
		Admins: admins,
	}, nil
}

// Add adds an admin
func (h *actorAdminHandler) Add(ctx context.Context, req *graphpb.ActorAdminAddRequest) (*graphpb.ActorAdminAddResponse, error) {
	log.Printf("Received Add request: %v", req)
	var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	if err = h.env.Repos.ActorAdmins.Add(req.Admin, req.Actor, viewer); err != nil {
		return nil, err
	}

	// publish event
	go func() {
		srvName := strings.Split(req.Actor.Service, ".")[0]
		routing_key := fmt.Sprintf(event.EventFormatEdgeAdminAdded, srvName)
		payload := &event.EventPayload{
			Name:           routing_key,
			SourceService:  req.Actor.Service,
			SubjectID:      req.Viewer.Id,
			ObjectID:       req.Admin.Id,
			TargetID:       req.Actor.Id,
			ViewerID:       viewer.ID,
			ViewerUserType: viewer.UserType,
		}
		h.env.Publisher.Publish(payload, routing_key)
	}()

	return &graphpb.ActorAdminAddResponse{
		Admin: req.Admin,
	}, nil
}

// Delete removes an admin
func (h *actorAdminHandler) Delete(ctx context.Context, req *graphpb.ActorAdminDeleteRequest) (*graphpb.ActorAdminDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req)
	var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType

	if err = h.env.Repos.ActorAdmins.Delete(req.Admin, req.Actor); err != nil {
		return nil, err
	}

	// publish event
	go func() {
		srvName := strings.Split(req.Actor.Service, ".")[0]
		routing_key := fmt.Sprintf(event.EventFormatEdgeAdminDeleted, srvName)
		payload := &event.EventPayload{
			Name:           routing_key,
			SourceService:  req.Actor.Service,
			SubjectID:      req.Viewer.Id,
			ObjectID:       req.Admin.Id,
			TargetID:       req.Actor.Id,
			ViewerID:       viewer.ID,
			ViewerUserType: viewer.UserType,
		}
		h.env.Publisher.Publish(payload, routing_key)
	}()

	return &graphpb.ActorAdminDeleteResponse{
		Admin: req.Admin,
	}, nil
}
