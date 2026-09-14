package validators

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// A display name is optional, and an actor without one must be creatable.
//
// It was required, and that made the server-rendered signup impossible: the
// form deliberately does not ask for a name — neither Mastodon nor Bluesky
// does — so every account it tried to create was refused with "name is
// required", after the handle had already been checked and reserved.
//
// The handle is the thing that must exist. It is what a URL is keyed on, what
// WebFinger answers with, and what both federation protocols fall back to when
// there is no display name.
func TestActorNameIsOptionalButAliasIsNot(t *testing.T) {
	withName := func(name, alias string) *entities.AnahitaNode {
		node := entities.NewAnahitaNode()
		node.Type = constants.NodeActorPersonEntityType
		node.Service = constants.PersonService
		node.Name = name
		node.Alias = alias
		node.Access = constants.AnahitaAccessPublic
		return node
	}

	v := NewActorValidator()

	if err := v.ValidateAdd(withName("", "ada")); err != nil {
		t.Errorf("an actor with a handle and no display name was refused: %v — "+
			"signup does not ask for a name, so this is every new account", err)
	}

	if err := v.ValidateAdd(withName("Ada Lovelace", "ada")); err != nil {
		t.Errorf("an actor with both was refused: %v", err)
	}

	if err := v.ValidateAdd(withName("Ada Lovelace", "")); err == nil {
		t.Error("an actor with no handle was accepted. It would have no URL, " +
			"no WebFinger answer, and nothing for a display name to fall back to")
	}
}
