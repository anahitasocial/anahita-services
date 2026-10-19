package handlers

import (
	"context"
	"testing"

	"anahita-libs/proto/graphpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Refused before anything is read: nobody, anybody who is not an
// administrator, anybody changing their own, which is what keeps the last
// super administrator, and a type there is not (the user, 2026-10-18).
// Which change an administrator may make is constants.MayChangePersonType,
// tested in anahita-libs.
func TestSetPersonTypeRefusesBeforeReading(t *testing.T) {
	h := &actorHandler{}
	for _, each := range []struct {
		name   string
		viewer *graphpb.Viewer
		id     int64
		want   codes.Code
	}{
		{"nobody", nil, 2, codes.Unauthenticated},
		{"somebody registered", &graphpb.Viewer{Id: 1, PersonType: "registered"}, 2, codes.PermissionDenied},
		{"a super administrator, their own", &graphpb.Viewer{Id: 1, PersonType: "super-administrator"}, 1, codes.FailedPrecondition},
		{"an administrator, their own", &graphpb.Viewer{Id: 1, PersonType: "administrator"}, 1, codes.FailedPrecondition},
	} {
		_, err := h.SetPersonType(context.Background(), &graphpb.ActorSetPersonTypeRequest{
			Id: each.id, PersonType: "registered", Viewer: each.viewer,
		})
		if status.Code(err) != each.want {
			t.Errorf("%s: %v; want %s", each.name, err, each.want)
		}
	}

	_, err := h.SetPersonType(context.Background(), &graphpb.ActorSetPersonTypeRequest{
		Id: 2, PersonType: "guest", Viewer: &graphpb.Viewer{Id: 1, PersonType: "super-administrator"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("a guest given: %v; want InvalidArgument", err)
	}
}

func TestPersonTypeActionsFitTheAuditColumn(t *testing.T) {
	for _, personType := range []string{"registered", "administrator", "super-administrator"} {
		if action := personTypeAction(personType); len(action) > 20 {
			t.Errorf("%s: %q is longer than node_deletion_logs.action", personType, action)
		}
	}
}
