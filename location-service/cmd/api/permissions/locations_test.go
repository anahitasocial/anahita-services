package permissions

import (
	"testing"

	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
)

func viewerOf(personType string) *entities.Viewer {
	viewer := entities.NewViewer()
	viewer.ID = 1
	viewer.PersonType = personType
	viewer.Scopes = []string{libs_constants.OAuthScopeTaxonomyWrite}
	return viewer
}

// Who may add a location follows LOCATIONS_FROM, by rank (the user,
// 2026-10-18); editing and deleting stay with administrators.
func TestWhoMayAddALocation(t *testing.T) {
	for _, each := range []struct {
		level      string
		personType string
		may        bool
	}{
		{"administrators", "registered", false},
		{"administrators", "administrator", true},
		{"administrators", "super-administrator", true},
		{"registered", "guest", false},
		{"registered", "registered", true},
		{"registered", "administrator", true},
		{"super-administrators", "administrator", false},
		{"super-administrators", "super-administrator", true},
	} {
		may, _ := newLocationsPermissions(each.level).CanAdd(viewerOf(each.personType), nil)
		if may != each.may {
			t.Errorf("LOCATIONS_FROM=%s, %s: may add %v; want %v", each.level, each.personType, may, each.may)
		}
	}

	member := viewerOf("registered")
	perms := newLocationsPermissions("registered")
	if may, _ := perms.CanEdit(member, nil); may {
		t.Error("somebody registered may edit a location")
	}
	if may, _ := perms.CanDelete(member, nil); may {
		t.Error("somebody registered may delete a location")
	}

	member.Scopes = nil
	if may, _ := perms.CanAdd(member, nil); may {
		t.Error("a token without taxonomy:write may add a location")
	}
}
