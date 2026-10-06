package handlers

import (
	"encoding/json"
	"reflect"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// Edit re-reads the node and copies a whitelist over it, so a field left out
// of that list is silently discarded: the endpoint returns 200, the database
// keeps its old value, and nothing errors or logs. website_url did exactly
// that after being added to the column, the proto, the entity and the
// response — correct in five places, dropped in one.
//
// These tests walk the copy reflectively, so the NEXT field added to the
// request is covered without anybody remembering to come back here.

// editableFields is the contract: what a caller may change on an existing
// actor. A field added to the request and expected to be settable belongs
// here; one that must NOT be settable deliberately does not.
var editableFields = []string{
	"Enabled", "Access", "Name", "Body", "Alias", "WebsiteURL",
	"Verified", "AvatarFilename", "CoverFilename", "AllowFollowRequest",
	"ShowRepostsOnProfile", "QuotePolicy", "FeaturesRaw", "ModifiedBy", "Language",
}

// createOnlyFields may be set when the actor is created and never afterwards.
var createOnlyFields = []string{"Type", "Service", "Meta", "CreatedBy"}

// fullRequest sets every field the whitelists read, to non-zero values.
func fullRequest() *graphpb.Node {
	return &graphpb.Node{
		Id:                   42,
		Type:                 "node.actor.group-service.group.v1",
		Service:              "group-service",
		Enabled:              true,
		Access:               "public",
		Name:                 "Cyclists",
		Alias:                "cyclists",
		Body:                 "A group.",
		WebsiteUrl:           "https://example.com",
		Verified:             true,
		AvatarFilename:       "avatar.png",
		CoverFilename:        "cover.png",
		AllowFollowRequest:   true,
		ShowRepostsOnProfile: true,
		QuotePolicy:          "followers",
		FeaturesRaw:          `{"a":1}`,
		Meta:                 `{"b":2}`,
		CreatedBy:            7,
		ModifiedBy:           9,
		Language:             "fa",
	}
}

func fieldValue(t *testing.T, node *entities.AnahitaNode, name string) reflect.Value {
	t.Helper()
	v := reflect.ValueOf(*node).FieldByName(name)
	if !v.IsValid() {
		t.Fatalf("entities.AnahitaNode has no field %q — the list in this test is stale", name)
	}
	return v
}

// Every field a caller is allowed to change must actually arrive on the node.
func TestApplyEditableActorFieldsCopiesEverythingEditable(t *testing.T) {
	// A node with nothing set, so anything still zero afterwards was dropped.
	node := entities.NewAnahitaNode()
	applyEditableActorFields(node, fullRequest())

	for _, name := range editableFields {
		if fieldValue(t, node, name).IsZero() {
			t.Errorf("%s was not copied from the request. Editing it will return "+
				"200 and save nothing, with no error and nothing in a log.", name)
		}
	}
}

// And the whitelist must stay a whitelist. A caller must not be able to set
// counts, timestamps or somebody else's authorship by putting them in the
// request — which is what replacing the node wholesale would allow.
func TestApplyEditableActorFieldsIgnoresEverythingElse(t *testing.T) {
	node := entities.NewAnahitaNode()

	// Values a caller might try to smuggle in on an edit.
	req := fullRequest()
	req.FollowerCount = 9999
	req.LeaderCount = 8888
	req.ViewCount = 7777
	req.CreatedBy = 1234

	applyEditableActorFields(node, req)

	for name, got := range map[string]any{
		"FollowerCount": node.FollowerCount,
		"LeaderCount":   node.LeaderCount,
		"ViewCount":     node.ViewCount,
		"CreatedBy":     node.CreatedBy,
	} {
		if !reflect.ValueOf(got).IsZero() {
			t.Errorf("%s = %v, but an edit must not be able to set it", name, got)
		}
	}
}

// Creation carries the editable fields PLUS the ones that can only be set once.
func TestNewActorFromRequestCarriesCreateOnlyFields(t *testing.T) {
	node := newActorFromRequest(fullRequest())

	for _, name := range append(append([]string{}, editableFields...), createOnlyFields...) {
		if fieldValue(t, node, name).IsZero() {
			t.Errorf("%s was not carried into a newly created actor", name)
		}
	}
}

// The two lists must not drift into being the same thing: type, service and
// authorship are set once and never by an edit.
func TestCreateOnlyFieldsAreNotEditable(t *testing.T) {
	existing := entities.NewAnahitaNode()
	existing.Type = "node.actor.group-service.group.v1"
	existing.Service = "group-service"
	existing.CreatedBy = 7
	existing.Meta = json.RawMessage(`{"original":true}`)

	req := fullRequest()
	req.Type = "node.actor.person-service.person.v1"
	req.Service = "person-service"
	req.CreatedBy = 999
	req.Meta = `{"tampered":true}`

	applyEditableActorFields(existing, req)

	if existing.Type != "node.actor.group-service.group.v1" {
		t.Errorf("an edit changed the actor's type to %q — a group became a person", existing.Type)
	}
	if existing.Service != "group-service" {
		t.Errorf("an edit changed the owning service to %q", existing.Service)
	}
	if existing.CreatedBy != 7 {
		t.Errorf("an edit reassigned authorship to %d", existing.CreatedBy)
	}
	if string(existing.Meta) != `{"original":true}` {
		t.Errorf("an edit overwrote meta: %s", existing.Meta)
	}
}

// Nil inputs must not panic: both arrive from a gRPC request, which is to say
// from outside.
func TestActorFieldHelpersSurviveNilInput(t *testing.T) {
	applyEditableActorFields(nil, fullRequest())
	applyEditableActorFields(entities.NewAnahitaNode(), nil)
	applyEditableActorFields(nil, nil)

	if node := newActorFromRequest(nil); node == nil {
		t.Error("newActorFromRequest(nil) returned nil rather than an empty node")
	}
}
