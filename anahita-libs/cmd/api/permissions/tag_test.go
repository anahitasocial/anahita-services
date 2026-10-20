package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

func personChoosing(id int64, access string) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = id
	n.Type = constants.NodeActorPersonEntityType
	if access != "" {
		n.FeaturesRaw = []byte(`[{"service":"socialgraph-service","add_permissions":[{"entity":"tag","access":"` + access + `"}]}]`)
	}
	return n
}

func TestWhoMayBeTaggedAndWhetherItWaits(t *testing.T) {
	author := entities.NewViewer()
	author.ID = 1

	cases := []struct {
		name             string
		target           *entities.AnahitaNode
		rel              *entities.ActorRelation
		allowed, pending bool
	}{
		{"never chose: approval first", personChoosing(2, ""), nil, true, true},
		{"approve", personChoosing(2, "approve"), nil, true, true},
		{"anybody", personChoosing(2, "registered"), nil, true, false},
		{"nobody", personChoosing(2, "admins"), nil, false, false},
		{"themselves, whatever they chose", personChoosing(1, "admins"), nil, true, false},
		{"blocks the author", personChoosing(2, "registered"), &entities.ActorRelation{IsBlocking: true}, false, false},
	}
	for _, tc := range cases {
		allowed, pending := TagDecision(tc.target, author, tc.rel)
		if allowed != tc.allowed || pending != tc.pending {
			t.Errorf("%s: got %v, %v; want %v, %v", tc.name, allowed, pending, tc.allowed, tc.pending)
		}
	}

	group := entities.NewAnahitaNode()
	group.ID = 3
	group.Type = constants.NodeActorGroupEntityType
	if allowed, _ := TagDecision(group, author, nil); allowed {
		t.Error("a group was tagged")
	}
	remote := personChoosing(4, "registered")
	remote.Type = constants.NodeActorActivityPubPersonEntityType
	if allowed, _ := TagDecision(remote, author, nil); allowed {
		t.Error("somebody on another server was tagged")
	}
}
