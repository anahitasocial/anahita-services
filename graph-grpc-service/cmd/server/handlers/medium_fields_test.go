package handlers

import (
	"reflect"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// What an edit may change on a post. A field the services send and expect
// to be saved belongs here.
var editableMediumFields = []string{
	"Enabled", "Pinned", "Access", "Name", "Alias", "Body", "Excerpt",
	"Filename", "CoverFilename", "Meta", "ModifiedBy", "CommentStatus",
	"CommentCount", "Language", "ReplyAccess",
}

// Set when a post is made and never by an edit.
var createOnlyMediumFields = []string{
	"OwnerID", "OwnerType", "Type", "Service", "Mimetype", "CreatedBy",
}

func fullMediumRequest() *graphpb.Node {
	return &graphpb.Node{
		Id:            42,
		Type:          "node.medium.text-service.note.v1",
		Service:       "text-service",
		OwnerId:       5,
		OwnerType:     "node.actor.person-service.person.v1",
		Enabled:       true,
		Pinned:        true,
		Access:        "followers",
		Name:          "A title",
		Alias:         "a-title",
		Body:          "متن",
		Excerpt:       "An excerpt",
		Filename:      "a.jpg",
		Mimetype:      "image/jpeg",
		CoverFilename: "cover.jpg",
		Meta:          `{"a":1}`,
		CreatedBy:     5,
		ModifiedBy:    5,
		CommentStatus: true,
		CommentCount:  3,
		Language:      "fa",
		ReplyAccess:   "followers,mentioned",
	}
}

func TestApplyEditableMediumFieldsCopiesEverythingEditable(t *testing.T) {
	// A node with nothing set, so anything still zero afterwards was dropped.
	node := entities.NewAnahitaNode()
	applyEditableMediumFields(node, fullMediumRequest())

	for _, name := range editableMediumFields {
		if fieldValue(t, node, name).IsZero() {
			t.Errorf("%s was not copied from the request. Editing it will "+
				"succeed and save nothing, with no error and nothing in a log.", name)
		}
	}
}

func TestNewMediumFromRequestCarriesEverything(t *testing.T) {
	node := newMediumFromRequest(fullMediumRequest())

	for _, name := range append(append([]string{}, editableMediumFields...), createOnlyMediumFields...) {
		if fieldValue(t, node, name).IsZero() {
			t.Errorf("%s was not carried into a newly created post", name)
		}
	}
}

// The language in particular, by name: it is the field this was written for.
func TestAPostKeepsTheLanguageItWasSentWith(t *testing.T) {
	if got := newMediumFromRequest(fullMediumRequest()).Language; got != "fa" {
		t.Errorf("a new post's language = %q, want fa", got)
	}

	existing := entities.NewAnahitaNode()
	existing.Language = "en"
	applyEditableMediumFields(existing, fullMediumRequest())
	if existing.Language != "fa" {
		t.Errorf("an edited post's language = %q, want fa", existing.Language)
	}
}

// An edit must not be able to move a post, retype it, or claim it for
// somebody else, nor set what is counted.
func TestAnEditCannotSetWhatIsSetOnce(t *testing.T) {
	existing := entities.NewAnahitaNode()
	existing.Type = "node.medium.photo-service.photo.v1"
	existing.Service = "photo-service"
	existing.OwnerID = 8
	existing.CreatedBy = 7

	req := fullMediumRequest()
	req.RepostCount = 999
	req.ViewCount = 888

	applyEditableMediumFields(existing, req)

	if existing.Type != "node.medium.photo-service.photo.v1" || existing.Service != "photo-service" ||
		existing.OwnerID != 8 || existing.CreatedBy != 7 {
		t.Errorf("an edit changed what is set once: %s %s owner %d by %d",
			existing.Type, existing.Service, existing.OwnerID, existing.CreatedBy)
	}

	for name, got := range map[string]any{
		"RepostCount": existing.RepostCount,
		"ViewCount":   existing.ViewCount,
	} {
		if !reflect.ValueOf(got).IsZero() {
			t.Errorf("%s = %v, but an edit must not be able to set it", name, got)
		}
	}
}
