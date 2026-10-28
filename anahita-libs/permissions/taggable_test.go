package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// A profile or post to tie a location to: its own id, who wrote it and
// whose it is.
type taggable struct{ id, author, owner int64 }

func (t taggable) GetID() int64       { return t.id }
func (t taggable) GetAuthorID() int64 { return t.author }
func (t taggable) GetOwnerID() int64  { return t.owner }
func (t taggable) GetParentID() int64 { return 0 }

// Who may tie a location to a profile or a post, and take it away: its
// author, its owner, the profile itself, and administrators. A person's
// profile records neither an author nor an owner, so without the profile
// itself nobody but an administrator could give it a location.
func TestWhoMayTagWithALocation(t *testing.T) {
	member := func(id int64) *entities.Viewer {
		v := &entities.Viewer{}
		v.ID = id
		v.PersonType = constants.PersonTypeRegistered
		v.Scopes = []string{constants.OAuthScopeMediaWrite}
		return v
	}
	perms := &TaggablePermissions{}

	cases := []struct {
		name   string
		viewer *entities.Viewer
		node   taggable
		want   bool
	}{
		{"their own profile", member(7), taggable{id: 7}, true},
		{"somebody else's profile", member(7), taggable{id: 8}, false},
		{"a post they wrote, in a group", member(7), taggable{id: 50, author: 7, owner: 90}, true},
		{"a post on their profile", member(7), taggable{id: 51, author: 8, owner: 7}, true},
		{"somebody else's post", member(7), taggable{id: 52, author: 8, owner: 8}, false},
	}
	for _, each := range cases {
		added, _ := perms.CanAdd(each.viewer, each.node)
		taken, _ := perms.CanDelete(each.viewer, each.node)
		if added != each.want || taken != each.want {
			t.Errorf("%s: may add %v and take away %v, want %v", each.name, added, taken, each.want)
		}
	}

	// A visitor's id is 0, and so is a profile's author when it has none:
	// 0 is never somebody.
	visitor := member(0)
	if added, _ := perms.CanAdd(visitor, taggable{id: 0}); added {
		t.Error("a visitor may tag a node with no id")
	}
}
