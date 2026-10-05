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
	person.Alias = "ada"
	person.Name = "Ada Lovelace"
	person.WebsiteURL = "https://example.com"

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

// The person-only fields reach the response from the embedded node, not from
// copies on Person.
//
// Person used to declare PersonType itself while also embedding a node that
// has one. The outer field shadows the embedded one, so assigning the node
// left the copy empty and the response carried "" for everybody — which is
// exactly what happened when person_type moved onto the node.
func TestPersonResponseReadsThroughToTheNode(t *testing.T) {
	person := entities.NewPerson()
	person.ID = 42
	person.PersonType = "super-administrator"
	person.PersonPronouns = "they/them"

	encoded, err := json.Marshal((&PersonResponse{}).MapNode(person))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, want := range []string{
		`"person_type":"super-administrator"`,
		`"person_pronouns":"they/them"`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("%s is missing from the response:\n%s", want, encoded)
		}
	}
}

// email, given_name, family_name, gender and username are gone from the
// person response. email is a login identifier that was being sent for every
// person in a public browse.
func TestPersonResponseDropsTheCredentialFields(t *testing.T) {
	person := entities.NewPerson()
	person.ID = 42
	person.Name = "Ada Lovelace"
	person.Alias = "ada"

	encoded, err := json.Marshal((&PersonResponse{}).MapNode(person))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, gone := range []string{"email", "given_name", "family_name", "gender", `"username"`} {
		if strings.Contains(string(encoded), gone) {
			t.Errorf("%s is still in the person response:\n%s", gone, encoded)
		}
	}
}
