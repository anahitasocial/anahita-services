package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// stubActorNode is the minimum ActorNodeInterface the permission checks
// touch: they only ever read the node's id.
type stubActorNode struct {
	*entities.AnahitaNode
}

func newStubActor(id int64) *stubActorNode {
	n := &entities.AnahitaNode{}
	n.ID = id
	return &stubActorNode{AnahitaNode: n}
}

func viewerWith(userType string, scopes ...string) *entities.Viewer {
	v := entities.NewViewer()
	v.ID = 100
	v.Authenticated = true
	v.PersonType = userType
	v.Scopes = scopes
	return v
}

// TestCanAdministrateEncodesTheDoctrine pins the rule the whole scope
// redesign rests on: a scope gates the API surface, a role gates the
// authority, and a role NEVER bypasses a missing scope.
//
// The last case is the important one. Under the previous design an
// admin:actors scope carried both meanings at once, which is why the
// admin tier was removed — but that only works if the write scope is
// still independently required of admins.
func TestCanAdministrateEncodesTheDoctrine(t *testing.T) {
	perms := NewActorPermissions()
	actor := newStubActor(555)

	cases := []struct {
		name    string
		viewer  *entities.Viewer
		rel     *entities.ActorRelation
		allowed bool
	}{
		{
			name:    "guest with no scopes is denied",
			viewer:  entities.NewViewer(),
			rel:     &entities.ActorRelation{},
			allowed: false,
		},
		{
			name:    "registered with the scope but no relationship is denied",
			viewer:  viewerWith(constants.PersonTypeRegistered, constants.OAuthScopeActorsWrite),
			rel:     &entities.ActorRelation{},
			allowed: false,
		},
		{
			name:    "registered with the scope and an admin relationship is allowed",
			viewer:  viewerWith(constants.PersonTypeRegistered, constants.OAuthScopeActorsWrite),
			rel:     &entities.ActorRelation{HasAdmin: true},
			allowed: true,
		},
		{
			name:    "site admin with the scope is allowed",
			viewer:  viewerWith(constants.PersonTypeAdmin, constants.OAuthScopeActorsWrite),
			rel:     &entities.ActorRelation{},
			allowed: true,
		},
		{
			// The doctrine row: role must not rescue a missing scope.
			name:    "site admin WITHOUT the scope is denied",
			viewer:  viewerWith(constants.PersonTypeAdmin),
			rel:     &entities.ActorRelation{},
			allowed: false,
		},
		{
			name:    "super admin WITHOUT the scope is denied",
			viewer:  viewerWith(constants.PersonTypeSuperAdmin),
			rel:     &entities.ActorRelation{},
			allowed: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := perms.CanAdministrate(&ActorAccessContext{
				Viewer:         tc.viewer,
				Actor:          actor,
				ActorViewerRel: tc.rel,
				AdminViewerRel: &entities.ActorRelation{},
			})
			if ok != tc.allowed {
				t.Errorf("CanAdministrate = %v (err %v), want %v", ok, err, tc.allowed)
			}
			if !tc.allowed && err == nil {
				t.Error("a denial must carry an error explaining why")
			}
		})
	}
}
