package services

import (
	"encoding/json"
	"testing"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// A field that survives the database, the proto and the response but is
// dropped at ONE mapping hop produces a 200 with nothing saved — no error
// anywhere, and nothing in a log to look at. website_url did exactly that:
// it reached the wire and was discarded by the edit handler's field list.
//
// This asserts the round trip through both mappers, which is the pair a new
// field is most likely to be added to only halfway.

func TestNodeMappingRoundTripPreservesEditableFields(t *testing.T) {
	original := entities.NewAnahitaNode()
	original.ID = 42
	original.Type = "com.people.person"
	original.Service = "person-service"
	original.Name = "Ada Lovelace"
	original.Alias = "ada"
	original.Body = "Mathematician."
	original.WebsiteURL = "https://example.com"
	original.Enabled = true
	original.Verified = true

	got := grpcToGraphNode(graphNodeToGrpc(original))
	if got == nil {
		t.Fatal("round trip produced nil")
	}

	// Everything a person can edit on their own profile. Each is named
	// individually so a failure says which field was lost.
	checks := []struct {
		field string
		want  any
		got   any
	}{
		{"ID", original.ID, got.ID},
		{"Type", original.Type, got.Type},
		{"Name", original.Name, got.Name},
		{"Alias", original.Alias, got.Alias},
		{"Body", original.Body, got.Body},
		{"WebsiteURL", original.WebsiteURL, got.WebsiteURL},
		{"Enabled", original.Enabled, got.Enabled},
		{"Verified", original.Verified, got.Verified},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s did not survive the round trip: got %v, want %v",
				c.field, c.got, c.want)
		}
	}
}

// An empty website must round-trip as empty rather than becoming something
// else — clearing the field is how somebody removes their link.
func TestEmptyWebsiteRoundTripsAsEmpty(t *testing.T) {
	original := entities.NewAnahitaNode()
	original.ID = 42
	original.WebsiteURL = ""

	got := grpcToGraphNode(graphNodeToGrpc(original))
	if got.WebsiteURL != "" {
		t.Errorf("an empty website became %q", got.WebsiteURL)
	}
}

// A node whose meta or features column is NULL arrives over gRPC as an empty
// string. Converting that straight to json.RawMessage produces a NON-nil,
// zero-length slice, and json.RawMessage marshals nil as "null" but a
// zero-length slice as nothing at all — which fails the WHOLE response with
//
//	json: error calling MarshalJSON for type json.RawMessage:
//	unexpected end of JSON input
//
// Follow and unfollow serialise the actor node directly, so every actor with
// no features row — which is nearly all of them — made those endpoints answer
// 500. Absent has to map to nil, not to empty.
func TestEmptyRawJSONColumnsMarshal(t *testing.T) {
	node := grpcToGraphNode(&graphpb.Node{Id: 42, Meta: "", FeaturesRaw: ""})

	if node.Meta != nil {
		t.Errorf("an absent meta column became a non-nil %#v", node.Meta)
	}
	if node.FeaturesRaw != nil {
		t.Errorf("an absent features column became a non-nil %#v", node.FeaturesRaw)
	}

	if _, err := json.Marshal(node); err != nil {
		t.Fatalf("a node with no meta and no features could not be marshalled: %v", err)
	}
}

// The other half of the same trap: clearing an actor's features must leave the
// field marshallable rather than storing an empty slice.
func TestSetFeaturesNilLeavesNodeMarshallable(t *testing.T) {
	node := entities.NewAnahitaNode()
	if err := node.SetFeatures(nil); err != nil {
		t.Fatalf("SetFeatures(nil): %v", err)
	}

	if _, err := json.Marshal(node); err != nil {
		t.Fatalf("a node whose features were cleared could not be marshalled: %v", err)
	}
}

// featured_at crosses both mappers, and "not featured" has to stay nil on the
// way back rather than becoming the epoch — which a client would read as
// featured long ago, and which would put every actor in the featured list.
func TestFeaturedAtRoundTrips(t *testing.T) {
	featured := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)

	original := entities.NewAnahitaNode()
	original.ID = 42
	original.FeaturedAt = &featured

	got := grpcToGraphNode(graphNodeToGrpc(original))
	if got.FeaturedAt == nil || !got.FeaturedAt.Equal(featured) {
		t.Errorf("featured_at did not survive the round trip: got %v, want %v",
			got.FeaturedAt, featured)
	}

	original.FeaturedAt = nil
	if got := grpcToGraphNode(graphNodeToGrpc(original)); got.FeaturedAt != nil {
		t.Errorf("an actor that is not featured came back featured at %v", *got.FeaturedAt)
	}
}
