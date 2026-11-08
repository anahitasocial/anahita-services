package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"time"
)

// SocialgraphMuteGRPCInterface mutes and unmutes (roadmap row 133).
type SocialgraphMuteGRPCInterface interface {
	// Add mutes target for source, or changes the mute: hideNotifications
	// is whether source stops being told of what target does, endsAt when
	// the mute ends, zero for until unmuted.
	Add(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer, hideNotifications bool, endsAt time.Time) error

	// Delete unmutes.
	Delete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error
}

type SocialgraphMuteGRPC struct {
	config GraphGRPCConfigInterface
}

func NewSocialgraphMuteGRPC(config GraphGRPCConfigInterface) SocialgraphMuteGRPCInterface {
	return &SocialgraphMuteGRPC{config: config}
}

func (srv *SocialgraphMuteGRPC) getClient() (graphpb.SocialgraphMuteServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewSocialgraphMuteServiceClient(conn), nil
}

func (srv *SocialgraphMuteGRPC) Add(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer, hideNotifications bool, endsAt time.Time) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Add(context.Background(), &graphpb.SocialgraphMuteAddRequest{
		Source:            actorNodeToSocialgraphSource(source),
		Target:            actorNodeToSocialgraphTarget(target),
		Viewer:            muteViewer(viewer),
		HideNotifications: hideNotifications,
		EndsAt:            timeToUnix(endsAt),
	})
	return err
}

func (srv *SocialgraphMuteGRPC) Delete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &graphpb.SocialgraphMuteDeleteRequest{
		Source: actorNodeToSocialgraphSource(source),
		Target: actorNodeToSocialgraphTarget(target),
		Viewer: muteViewer(viewer),
	})
	return err
}

func muteViewer(viewer *entities.Viewer) *graphpb.Viewer {
	return &graphpb.Viewer{
		Id:         viewer.ID,
		PersonType: viewer.PersonType,
		Email:      viewer.Email,
		Username:   viewer.Username,
	}
}
