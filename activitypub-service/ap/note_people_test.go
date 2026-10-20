package ap

import (
	"testing"

	"anahita-libs/federation"
)

// Who is in a picture goes out as mentioned, once each, beside whoever its
// words name (roadmap row 103).
func TestPeopleInAPictureAreMentionedOnce(t *testing.T) {
	n := &Note{Tag: []Tag{{Type: "Mention", Href: "https://here.example/ap/users/a", Name: "@ana"}}}
	n.WithPeople([]federation.Named{
		{Name: "@ana@here.example", Href: "https://here.example/ap/users/a"},
		{Name: "@worf@here.example", Href: "https://here.example/ap/users/w"},
	})
	if len(n.Tag) != 2 {
		t.Fatalf("tags: %+v", n.Tag)
	}
	if n.Tag[1].Type != "Mention" || n.Tag[1].Name != "@worf@here.example" {
		t.Errorf("worf: %+v", n.Tag[1])
	}
}
