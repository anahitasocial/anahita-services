package responses

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// A person's account type goes to themselves and to administrators, and to
// nobody else: not a visitor, not another member, not a handler that forgot
// to say who is asking.
func TestAccountTypeIsToldOnlyToThemAndAdministrators(t *testing.T) {
	person := &entities.Person{}
	person.ID = 5
	person.PersonType = constants.PersonTypeSuperAdmin

	viewer := func(id int64, personType string) *entities.Viewer {
		v := &entities.Viewer{}
		v.ID = id
		v.PersonType = personType
		return v
	}

	cases := []struct {
		name   string
		viewer *entities.Viewer
		want   string
	}{
		{"nobody said", nil, ""},
		{"a visitor", &entities.Viewer{}, ""},
		{"another member", viewer(9, constants.PersonTypeRegistered), ""},
		{"themselves", viewer(5, constants.PersonTypeSuperAdmin), constants.PersonTypeSuperAdmin},
		{"an administrator", viewer(9, constants.PersonTypeAdmin), constants.PersonTypeSuperAdmin},
		{"a super administrator", viewer(9, constants.PersonTypeSuperAdmin), constants.PersonTypeSuperAdmin},
	}
	for _, each := range cases {
		got := NewPersonResponse(each.viewer).MapNodeItem(person).PersonType
		if got != each.want {
			t.Errorf("%s: told %q, want %q", each.name, got, each.want)
		}
	}
	if got := (&PersonResponse{}).MapNodeItem(person).PersonType; got != "" {
		t.Errorf("a response made without a viewer told %q", got)
	}
}
