package ap

import (
	"encoding/json"
	"strings"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/federation"
	"anahita-libs/videos"
)

// Note is a post, as it is sent to and read from another server.
type Note struct {
	Context      any    `json:"@context,omitempty"`
	ID           string `json:"id"`
	Type         string `json:"type"`
	AttributedTo string `json:"attributedTo"`
	// An Article's title, and the opening it is summed up by. A note has
	// neither: Mastodon shows a note's summary as a warning to click past.
	Name       string            `json:"name,omitempty"`
	Summary    string            `json:"summary,omitempty"`
	Content    string            `json:"content"`
	ContentMap map[string]string `json:"contentMap,omitempty"`
	Published  string            `json:"published,omitempty"`
	Updated    string            `json:"updated,omitempty"`
	URL        string            `json:"url,omitempty"`
	To         []string          `json:"to"`
	Cc         []string          `json:"cc"`
	Tag        []Tag             `json:"tag,omitempty"`
	// Where it is: the first location its author chose for it, a `Place`
	// (roadmap row 82, part 3b). Every one is also a `Place` in Tag.
	Location map[string]any `json:"location,omitempty"`
	// What it answers, by the address that has between servers.
	InReplyTo string `json:"inReplyTo,omitempty"`
	// The pictures of a photo post, or the video of a video post.
	Attachment []SentPicture `json:"attachment,omitempty"`
	// An Article's cover.
	Image *Image `json:"image,omitempty"`
}

// SentPicture is one picture of a post, as it is sent: where to fetch it,
// what it is, how big, and what its author said it shows.
type SentPicture struct {
	Type      string `json:"type"`
	MediaType string `json:"mediaType"`
	URL       string `json:"url"`
	Name      string `json:"name,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	// A video's poster, which Mastodon shows before it plays.
	Icon *SentIcon `json:"icon,omitempty"`
}

// SentIcon is a still that stands for a video.
type SentIcon struct {
	Type      string `json:"type"`
	MediaType string `json:"mediaType"`
	URL       string `json:"url"`
}

// VideoMax is the heaviest video file sent to other servers: what Mastodon
// and GoToSocial take of one from elsewhere unless told otherwise is 40 MB
// and more. A post with no size that light goes as its words and a link.
const VideoMax = 40 * 1024 * 1024

// WithVideo gives the note the video of the video post it is, at the size
// sent, and the page it is read at. A video with no sound, as a GIF posted
// here is, is shown by Mastodon as a GIF: silently, on a loop.
func (n *Note) WithVideo(a Addresses, post *entities.AnahitaNode, video *videos.Video, file *videos.Rendition) *Note {
	n.URL = a.VideoPage(post.ID)
	if file == nil {
		return n
	}
	sent := SentPicture{
		Type:      "Document",
		MediaType: videos.MimeVideo,
		URL:       a.Video(post.UUID),
		Width:     file.Width,
		Height:    file.Height,
	}
	if video.Poster != "" {
		sent.Icon = &SentIcon{Type: "Image", MediaType: videos.MimePoster, URL: a.VideoPoster(post.UUID)}
	}
	n.Attachment = append(n.Attachment, sent)
	return n
}

// WithPictures gives the note the pictures of the photo post it is, and
// the page a photo post is read at.
func (n *Note) WithPictures(a Addresses, post *entities.AnahitaNode, files []*entities.PhotoFile) *Note {
	n.URL = a.PhotoPage(post.ID)
	for position, file := range files {
		n.Attachment = append(n.Attachment, SentPicture{
			Type:      "Document",
			MediaType: file.Mimetype,
			URL:       a.Picture(post.UUID, position),
			Name:      file.AltText,
			Width:     file.Width,
			Height:    file.Height,
		})
	}
	return n
}

// Tag names somebody or a hashtag that a post mentions, beside its text,
// or a location it was tagged with, a `Place` with its point.
type Tag struct {
	Type string `json:"type"`
	Href string `json:"href"`
	Name string `json:"name"`
	// A Place's point.
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// WithPlaces gives the post the locations its author chose for it (the
// user, 2026-10-19: choosing one is their consent, for that post; roadmap
// row 82, part 3b): the first as its `location`, as Pixelfed sends and reads
// one, since servers that read `location` expect one, and every one as a
// `Place` in its `tag`. Mastodon shows neither.
func (n *Note) WithPlaces(places []federation.Place) *Note {
	for i, place := range places {
		if i == 0 {
			n.Location = place.AsLocation()
		}
		latitude, longitude := place.Latitude, place.Longitude
		n.Tag = append(n.Tag, Tag{
			Type:      "Place",
			Href:      place.URL,
			Name:      place.Name,
			Latitude:  &latitude,
			Longitude: &longitude,
		})
	}
	return n
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

// AsArticle makes the post an Article: its title, its opening as its
// summary, all of it as its content, the page it is read at, and its
// cover. Mastodon shows one as its title and a link to that page;
// WriteFreely, Ghost, Friendica and the like show the article.
func (n *Note) AsArticle(a Addresses, article *entities.AnahitaNode, summary string) *Note {
	n.Type = "Article"
	n.Name = strings.TrimSpace(article.Name)
	n.Summary = summary
	n.URL = a.ArticlePage(article.ID)
	if article.CoverFilename != "" {
		n.Image = &Image{Type: "Image", MediaType: mediaType(article.CoverFilename), URL: a.Cover(article.UUID)}
	}
	return n
}

// Answering makes the note a reply: to what, and, when whoever wrote that
// is on another server, to whom. They are named beside the text as
// mentioned and as somebody it is for, which is what has their server tell
// them. Their name is not written into the reply.
func (n *Note) Answering(what, actor, handle string) *Note {
	n.InReplyTo = what
	if actor == "" {
		return n
	}

	for _, already := range n.Cc {
		if already == actor {
			return n
		}
	}
	n.Cc = append(n.Cc, actor)

	for _, tag := range n.Tag {
		if tag.Type == "Mention" && tag.Href == actor {
			return n
		}
	}
	n.Tag = append(n.Tag, Tag{Type: "Mention", Href: actor, Name: handle})
	return n
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
	// What is listed beside the text. Read for its custom emoji.
	Tag json.RawMessage `json:"tag"`
	// What came with it: pictures, and other things that are not kept.
	Attachment json.RawMessage `json:"attachment"`
	// Marked by its author as something not everybody wants to see.
	Sensitive bool `json:"sensitive"`
}

// Attachment is something that came with a post.
type Attachment struct {
	Type      string `json:"type"`
	MediaType string `json:"mediaType"`
	URL       string `json:"url"`
	// What it shows, for people who cannot see it.
	Name string `json:"name"`
}

// Pictures is the pictures that came with a post, in the order they came:
// what says it is an image, at an address on the web. One thing or a list
// of them is read alike.
func (n *ReceivedNote) Pictures() []Attachment {
	if len(n.Attachment) == 0 {
		return nil
	}
	var many []Attachment
	if json.Unmarshal(n.Attachment, &many) != nil {
		var one Attachment
		if json.Unmarshal(n.Attachment, &one) != nil {
			return nil
		}
		many = []Attachment{one}
	}

	var pictures []Attachment
	for _, each := range many {
		if !strings.HasPrefix(strings.ToLower(each.MediaType), "image/") || !strings.HasPrefix(each.URL, "https://") || len(each.URL) > 2000 {
			continue
		}
		pictures = append(pictures, each)
	}
	return pictures
}

// Author is who a received post says wrote it.
func (n *ReceivedNote) Author() string { return idOf(n.AttributedTo) }

// IsReply says whether it answers another post.
func (n *ReceivedNote) IsReply() bool {
	return len(n.InReplyTo) > 0 && string(n.InReplyTo) != "null" && idOf(n.InReplyTo) != ""
}

// Answers is the address of what it answers, or "" when it answers
// nothing.
func (n *ReceivedNote) Answers() string {
	if !n.IsReply() {
		return ""
	}
	return idOf(n.InReplyTo)
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
