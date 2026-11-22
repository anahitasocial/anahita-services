package posts

import (
	"fmt"

	"anahita-libs/constants"
	"anahita-libs/videos"
)

// TopicType is a topic's node type: a forum's thread, written here and read
// here only.
const TopicType = "node.medium.text-service.topic.v1"

// Kind is a kind of post: the type the graph keeps it as, the word for it,
// the page it is read at in the web app, and whether it goes to the
// fediverse and Bluesky.
type Kind struct {
	Type string
	// Name is the word for it, in a custom feed's definition and an
	// embedded post: "note".
	Name string
	// Path is the first part of its page's address in the web app:
	// "notes", for /notes/12.
	Path string
	// Federates says whether it goes to the fediverse and to Bluesky, for
	// somebody who chose that. A topic stays here (the user, 2026-10-17).
	Federates bool
}

// Kinds is every kind of post, kept here once (roadmap row 170: it was
// written in six places, which disagreed: Bluesky's had no topics, the
// embeds no audio).
var Kinds = []Kind{
	{Type: constants.NodeMediumNoteEntityType, Name: "note", Path: "notes", Federates: true},
	{Type: constants.NodeMediumPhotoEntityType, Name: "photo", Path: "photos", Federates: true},
	{Type: videos.NodeType, Name: "video", Path: "videos", Federates: true},
	{Type: videos.AudioNodeType, Name: "audio", Path: "audio", Federates: true},
	{Type: constants.NodeMediumArticleEntityType, Name: "article", Path: "articles", Federates: true},
	{Type: TopicType, Name: "topic", Path: "topics", Federates: false},
}

// KindOf is the kind of a post's type.
func KindOf(nodeType string) (Kind, bool) {
	for _, kind := range Kinds {
		if kind.Type == nodeType {
			return kind, true
		}
	}
	return Kind{}, false
}

// KindAt is the kind whose pages are at a path's first part: "notes".
func KindAt(path string) (Kind, bool) {
	for _, kind := range Kinds {
		if kind.Path == path {
			return kind, true
		}
	}
	return Kind{}, false
}

// Page is where a post is read in the web app: /notes/12, /articles/12. A
// type that is no kind of post is read as a note.
func Page(nodeType string, id int64) string {
	path := "notes"
	if kind, ok := KindOf(nodeType); ok {
		path = kind.Path
	}
	return fmt.Sprintf("/%s/%d", path, id)
}

// Federates says whether a kind of post goes to the fediverse and Bluesky.
func Federates(nodeType string) bool {
	kind, ok := KindOf(nodeType)
	return ok && kind.Federates
}

// FederatingTypes is the types of the kinds that go to the fediverse and
// Bluesky.
func FederatingTypes() []string {
	var types []string
	for _, kind := range Kinds {
		if kind.Federates {
			types = append(types, kind.Type)
		}
	}
	return types
}
