package ap

import (
	"encoding/json"
	"strings"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/federation"
)

// Note is a post, as it is sent to and read from another server.
type Note struct {
	Context      any               `json:"@context,omitempty"`
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	AttributedTo string            `json:"attributedTo"`
	Content      string            `json:"content"`
	ContentMap   map[string]string `json:"contentMap,omitempty"`
	Published    string            `json:"published,omitempty"`
	Updated      string            `json:"updated,omitempty"`
	URL          string            `json:"url,omitempty"`
	To           []string          `json:"to"`
	Cc           []string          `json:"cc"`
	Tag          []Tag             `json:"tag,omitempty"`
}

// Tag names somebody or a hashtag that a post mentions, beside its text.
type Tag struct {
	Type string `json:"type"`
	Href string `json:"href"`
	Name string `json:"name"`
}

// NewNote is a post made here, as other servers are sent it: its Markdown
// as HTML that is safe to show, open to everybody, with its author's
// followers named as who it is for.
func NewNote(a Addresses, note, author *entities.AnahitaNode, links federation.Links) *Note {
	rendered := federation.ToHTML(note.Body, links)

	sent := &Note{
		ID:           a.Note(note.UUID),
		Type:         "Note",
		AttributedTo: a.Actor(author.UUID),
		Content:      rendered.HTML,
		URL:          a.NotePage(note.ID),
		To:           []string{Public},
		Cc:           []string{a.Followers(author.UUID)},
	}

	if !note.CreatedAt.IsZero() && note.CreatedAt.Unix() > 0 {
		sent.Published = note.CreatedAt.UTC().Format(time.RFC3339)
	}
	// Said only when it was changed after it was written.
	if note.UpdatedAt.Sub(note.CreatedAt) > time.Second {
		sent.Updated = note.UpdatedAt.UTC().Format(time.RFC3339)
	}

	// The language its author said it is in, which is what lets a reader
	// elsewhere filter by language, or be offered a translation.
	if language := strings.TrimSpace(note.Language); language != "" && language != "und" {
		sent.ContentMap = map[string]string{language: rendered.HTML}
	}

	for _, mention := range rendered.Mentions {
		sent.Tag = append(sent.Tag, Tag{Type: "Mention", Href: mention.Href, Name: mention.Name})
	}
	for _, hashtag := range rendered.Hashtags {
		sent.Tag = append(sent.Tag, Tag{Type: "Hashtag", Href: hashtag.Href, Name: hashtag.Name})
	}

	return sent
}

// WithContext is the note as a document of its own, not inside an activity.
func (n *Note) WithContext() *Note {
	alone := *n
	alone.Context = contextActivityStreams
	return &alone
}

// Posted is a post having been made, changed or taken away.
type Posted struct {
	Context   any      `json:"@context"`
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Actor     string   `json:"actor"`
	Published string   `json:"published,omitempty"`
	To        []string `json:"to"`
	Cc        []string `json:"cc,omitempty"`
	Object    any      `json:"object"`
}

// NewCreate says a post was made.
func NewCreate(a Addresses, note *Note, noteUUID string) *Posted {
	return &Posted{
		Context: contextActivityStreams, ID: a.NoteCreate(noteUUID), Type: "Create",
		Actor: note.AttributedTo, Published: note.Published, To: note.To, Cc: note.Cc, Object: note,
	}
}

// NewUpdate says a post was changed. A new name each time: it is a new
// thing that happened.
func NewUpdate(id string, note *Note) *Posted {
	return &Posted{
		Context: contextActivityStreams, ID: id, Type: "Update",
		Actor: note.AttributedTo, Published: note.Updated, To: note.To, Cc: note.Cc, Object: note,
	}
}

// Tombstone stands where something was.
type Tombstone struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// NewDelete says a post was taken away, to everybody who may have it.
func NewDelete(id, actor, noteID string) *Posted {
	return &Posted{
		Context: contextActivityStreams, ID: id, Type: "Delete",
		Actor: actor, To: []string{Public}, Object: Tombstone{ID: noteID, Type: "Tombstone"},
	}
}

// NewDeletePerson says somebody is gone, to everybody: whatever a server
// has of them is to be removed.
func NewDeletePerson(id, actor string) *Posted {
	return &Posted{Context: contextActivityStreams, ID: id, Type: "Delete", Actor: actor, To: []string{Public}, Object: actor}
}

// NewUpdatePerson says somebody's description changed, and gives it whole.
func NewUpdatePerson(id string, actor *Actor) *Posted {
	described := *actor
	described.Context = nil
	return &Posted{Context: actorContext, ID: id, Type: "Update", Actor: actor.ID, To: []string{Public}, Object: &described}
}

// ReceivedNote is a post as another server sends it. Read leniently: what
// servers send as one thing or as several is kept raw.
type ReceivedNote struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	AttributedTo json.RawMessage   `json:"attributedTo"`
	Content      string            `json:"content"`
	ContentMap   map[string]string `json:"contentMap"`
	Published    string            `json:"published"`
	URL          json.RawMessage   `json:"url"`
	To           json.RawMessage   `json:"to"`
	Cc           json.RawMessage   `json:"cc"`
	InReplyTo    json.RawMessage   `json:"inReplyTo"`
}

// Author is who a received post says wrote it.
func (n *ReceivedNote) Author() string { return idOf(n.AttributedTo) }

// IsReply says whether it answers another post.
func (n *ReceivedNote) IsReply() bool {
	return len(n.InReplyTo) > 0 && string(n.InReplyTo) != "null" && idOf(n.InReplyTo) != ""
}

// IsPublic says whether it is addressed to everybody. Only such a post is
// kept: one for followers only is not this installation's to show to
// whoever may read a public post here.
func (n *ReceivedNote) IsPublic() bool {
	for _, raw := range []json.RawMessage{n.To, n.Cc} {
		for _, address := range strings2(raw) {
			if address == Public || address == "as:Public" || address == "Public" {
				return true
			}
		}
	}
	return false
}

// Language is the language it says it is in, when it says one.
func (n *ReceivedNote) Language() string {
	if len(n.ContentMap) != 1 {
		return ""
	}
	for language := range n.ContentMap {
		if len(language) >= 2 && len(language) <= 12 {
			return strings.ToLower(language)
		}
	}
	return ""
}

// Note reads the post an activity carries, or nil when it carries only an
// address.
func (a *Activity) Note() *ReceivedNote {
	var note ReceivedNote
	if len(a.Object) == 0 || a.Object[0] != '{' || json.Unmarshal(a.Object, &note) != nil || note.ID == "" {
		return nil
	}
	return &note
}

// strings2 reads addresses sent as one string or as a list of them.
func strings2(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return many
	}
	return nil
}
