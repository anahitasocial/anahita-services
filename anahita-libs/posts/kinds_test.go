package posts

import (
	"slices"
	"testing"
)

// THE RULE THIS GUARDS (roadmap row 170): every kind of post has one page,
// one word and one answer to whether it goes out, read from this table by
// every service; and no two kinds share a page or a word.
func TestEveryKindOfPostHasOnePageAndOneAnswer(t *testing.T) {
	pages, names := map[string]bool{}, map[string]bool{}
	for _, kind := range Kinds {
		if pages[kind.Path] || names[kind.Name] {
			t.Errorf("%s: its page or its word is another kind's", kind.Type)
		}
		pages[kind.Path], names[kind.Name] = true, true
		if got, ok := KindAt(kind.Path); !ok || got.Type != kind.Type {
			t.Errorf("/%s/ is not %s's page", kind.Path, kind.Name)
		}
		if got := Page(kind.Type, 12); got != "/"+kind.Path+"/12" {
			t.Errorf("%s is read at %s", kind.Name, got)
		}
	}

	for page, want := range map[string]string{
		"/notes/12": "node.medium.text-service.note.v1", "/photos/12": "node.medium.photo-service.photo.v1",
		"/videos/12": "node.medium.video-service.video.v1", "/audio/12": "node.medium.video-service.audio.v1",
		"/articles/12": "node.medium.text-service.article.v1", "/topics/12": TopicType,
	} {
		if Page(want, 12) != page {
			t.Errorf("%s is read at %s, want %s", want, Page(want, 12), page)
		}
	}
	if Page("node.medium.somewhere.else.v1", 12) != "/notes/12" {
		t.Error("a type that is no kind of post is not read as a note")
	}

	// Topics stay here; the rest go out.
	if Federates(TopicType) || slices.Contains(FederatingTypes(), TopicType) {
		t.Error("a topic goes out")
	}
	if len(FederatingTypes()) != len(Kinds)-1 || !Federates("node.medium.video-service.audio.v1") {
		t.Errorf("going out: %v", FederatingTypes())
	}
}
