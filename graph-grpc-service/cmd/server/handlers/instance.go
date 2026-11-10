package handlers

import (
	"context"
	"strings"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	features "graph/data/actor_features"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ReadInstance says what the instance offers (roadmap row 139): the kinds
// of profile that may be made and of post that may be written, from
// instance.json and VIDEO_UPLOADS and AUDIO_UPLOADS.
func (h *graphHandler) ReadInstance(ctx context.Context, req *graphpb.ReadInstanceRequest) (*graphpb.ReadInstanceResponse, error) {
	profiles, posts, err := features.NewFeatureLoader(h.env.Config.GetActorFeaturesPath()).Offered()
	if err != nil {
		return nil, err
	}
	return &graphpb.ReadInstanceResponse{Profiles: profiles, Posts: posts}, nil
}

// refuseUnoffered refuses a post made here of a kind the instance does not
// offer (instance.json): whichever service asks, nothing of it is saved. A
// post from another server is kept whatever its kind; this is about what
// is made here.
func refuseUnoffered(dir string, node *entities.AnahitaNode) error {
	if node == nil || node.IsRemote() {
		return nil
	}
	parts := strings.Split(node.Type, ".")
	if len(parts) < 4 {
		return nil
	}
	instance, err := features.LoadInstance(dir)
	if err != nil || instance.Offers(parts[2], parts[3]) {
		return nil
	}
	return status.Errorf(codes.PermissionDenied, "off: this site does not offer %ss", parts[3])
}
