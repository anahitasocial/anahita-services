package services

import (
	"testing"

	"anahita-libs/data/entities"
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
