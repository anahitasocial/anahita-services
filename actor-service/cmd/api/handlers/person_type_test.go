package handlers

import (
	"testing"

	"anahita-libs/constants"
)

func TestWhatKindOfPersonAHandleIs(t *testing.T) {
	for alias, want := range map[string]string{
		"ana":                    constants.NodeActorPersonEntityType,
		"ana_b-2":                constants.NodeActorPersonEntityType,
		"alice@mastodon.example": constants.NodeActorActivityPubPersonEntityType,
		"alice.bsky.social":      constants.NodeActorATProtoPersonEntityType,
		"did-plc-aaaaaaaaaaaaaaaaaaaaaaaa.invalid": constants.NodeActorATProtoPersonEntityType,
	} {
		if got := personTypeOf(alias); got != want {
			t.Errorf("%s: %s, want %s", alias, got, want)
		}
	}
}
