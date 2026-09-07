package responses

import (
	"encoding/json"
	"strings"
	"testing"

	"anahita-libs/data/entities"
)

// website_url reaches the response through the embedded ActorNodeItem rather
// than through any person-specific field, which makes it easy to add to the
// entity and the database and still never see it on the wire.
func TestPersonResponseCarriesTheWebsite(t *testing.T) {
	person := entities.NewPerson()
	person.ID = 42
	person.Username = "ada"
	person.GivenName = "Ada"
	person.FamilyName = "Lovelace"
	person.AnahitaNode.Name = "Ada Lovelace"
	person.AnahitaNode.WebsiteURL = "https://example.com"

	encoded, err := json.Marshal((&PersonResponse{}).MapNode(person))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(encoded), `"website_url":"https://example.com"`) {
		t.Errorf("website_url is not in the response:\n%s", encoded)
	}
}

// Empty is omitted rather than sent as "", so a profile without one stays
// quiet — that is what omitempty is for here.
func TestPersonResponseOmitsAnEmptyWebsite(t *testing.T) {
	person := entities.NewPerson()
	person.ID = 42

	encoded, err := json.Marshal((&PersonResponse{}).MapNode(person))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(encoded), "website_url") {
		t.Errorf("an empty website was serialised:\n%s", encoded)
	}
}
