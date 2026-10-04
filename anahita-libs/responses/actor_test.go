package responses

import (
	"reflect"
	"testing"
	"time"

	"anahita-libs/data/entities"
)

// A response field that is declared but never assigned is invisible: it
// compiles, it serialises (as absent, thanks to omitempty), and the value is
// correct everywhere else in the system. It has happened twice on this struct
// — `enabled` was declared when the struct was written and never populated, and
// `website_url` was added to the column, the proto and the struct while the
// mapper was left behind, so it saved correctly and never reached a client.
//
// This walks the mapped result and fails on any field still holding its zero
// value, which catches the next one without anybody having to remember.

// notMappedFromANode lists fields that legitimately stay zero when mapping a
// bare node, with the reason. Anything NOT here is expected to be populated.
var notMappedFromANode = map[string]string{
	"DeletedAt": "nil unless the actor is scheduled for deletion — that is the signal",
	"Avatar":    "built from the node's avatar map, which a bare node has none of",
	"Cover":     "built from the node's cover map, which a bare node has none of",
	"Author":    "nil unless the node carries a separate author record",
}

// fullyPopulatedNode sets every source field the response is expected to read.
func fullyPopulatedNode() *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = 42
	n.Type = "node.actor.person-service.person.v1"
	n.Name = "Ada Lovelace"
	n.Alias = "ada"
	n.Body = "Mathematician."
	n.WebsiteURL = "https://example.com"
	n.Verified = true
	n.Enabled = true
	// The three that carry a viewer-facing setting. Access especially: the
	// settings card seeds its dropdown from it, so a response that omits it
	// makes the card save its default over the real value.
	n.Access = "followers"
	n.AllowFollowRequest = true
	archived := time.Now()
	n.ArchivedAt = &archived
	featured := time.Now()
	n.FeaturedAt = &featured
	n.CreatedAt = time.Now()
	n.FollowerCount = 3
	n.LeaderCount = 4
	n.IsLeadingViewer = true
	n.IsAdminedByViewer = true
	n.IsFollowingViewer = true
	n.IsBlockedByViewer = true
	return n
}

func TestMapNodeItemAssignsEveryDeclaredField(t *testing.T) {
	mapped := (&ActorResponse{}).MapNodeItem(fullyPopulatedNode())
	if mapped == nil {
		t.Fatal("MapNodeItem returned nil")
	}

	v := reflect.ValueOf(*mapped)
	typ := v.Type()

	for i := range typ.NumField() {
		field := typ.Field(i)
		if reason, skipped := notMappedFromANode[field.Name]; skipped {
			t.Logf("skipping %s: %s", field.Name, reason)
			continue
		}

		if v.Field(i).IsZero() {
			t.Errorf("ActorNodeItem.%s is declared but never assigned in "+
				"MapNodeItem — it will be missing from every response while "+
				"being correct everywhere else", field.Name)
		}
	}
}

// The two that have actually been wrong, asserted by value rather than by
// zero-check, so the failure names the symptom rather than the mechanism.
func TestMapNodeItemCarriesWebsiteAndEnabled(t *testing.T) {
	node := fullyPopulatedNode()
	mapped := (&ActorResponse{}).MapNodeItem(node)

	if mapped.WebsiteURL != node.WebsiteURL {
		t.Errorf("WebsiteURL = %q, want %q — it saves but never reaches a client",
			mapped.WebsiteURL, node.WebsiteURL)
	}
	if !mapped.Enabled {
		t.Error("Enabled was not carried; no client can tell a disabled actor from an enabled one")
	}
}

// enabled must serialise even when false — that is the case where it carries
// information, and omitempty would drop exactly that.
func TestEnabledIsSerialisedWhenFalse(t *testing.T) {
	node := fullyPopulatedNode()
	node.Enabled = false

	mapped := (&ActorResponse{}).MapNodeItem(node)

	field, ok := reflect.TypeOf(*mapped).FieldByName("Enabled")
	if !ok {
		t.Fatal("no Enabled field")
	}
	if tag := field.Tag.Get("json"); tag != "enabled" {
		t.Errorf(`json tag is %q, want "enabled" with no omitempty — false is `+
			`the value that matters and omitempty would drop it`, tag)
	}
}
