package responses

import (
	"time"

	"anahita-libs/data/entities"
)

func NewParent(node *entities.AnahitaNode) *Parent {
	if node == nil {
		return nil
	}

	dto := &Parent{
		ID:                 node.ID,
		Type:               node.Type,
		Alias:              node.Alias,
		Name:               node.Name,
		Body:               node.Body,
		Access:             node.Access,
		CreatedAt:          createdAtOrNil(node),
		IsLikedByViewer:    node.IsLikedByViewer,
		IsSavedByViewer:    node.IsSavedByViewer,
		IsRepostedByViewer: node.IsRepostedByViewer,
	}

	if node.Cover != nil {
		dto.Cover = NewCover(node.Cover)
	}

	if node.Portrait != nil {
		dto.Portrait = NewPortrait(node.Portrait)
	}

	dto.Files = NewPhotoFiles(node.Files)
	// A reposted note is shown as itself, with the card it carries.
	dto.LinkPreview = NewLinkPreview(node)
	dto.Video = NewVideo(node)

	if node.Owner != nil {
		dto.Owner = NewOwner(node.Owner)
	}

	if node.Author != nil {
		dto.Author = NewAuthor(node.Author)
	}

	// A repost's comment and like buttons act on the post it reposts, so
	// the answers ride on the parent.
	dto.Authorized = NewNodeAuthorized(node)

	return dto
}

// Parent is a post another one hangs from: what a reply answers, what a
// repost reposts, or the post at the top of a thread.
type Parent struct {
	// Its id.
	ID int64 `json:"id"`
	// What kind of thing it is: `node.actor.<service>.person.v1`, `...group.v1`,
	// `...event.v1`, or a post's kind.
	Type string `json:"type"`
	// A profile's handle (`ana`, or `ana@mastodon.social` for somebody on
	// another server), or a hashtag's name.
	Alias string `json:"alias,omitempty"`
	// A profile's name, or a post's title where it has one.
	Name string `json:"name,omitempty"`
	// Its words, as Markdown.
	Body string `json:"body,omitempty"`
	// Who may see it. Said so that a repost can be shared as what it reposts.
	Access string `json:"access,omitempty"`
	// When it was written, so that a reposted post can be shown as itself.
	CreatedAt *time.Time `json:"created_at,omitempty"`
	// A profile's cover picture, in each size there is.
	Cover map[string]Image `json:"cover_urls,omitempty"`
	// A photo post's first picture, in each size there is.
	Portrait map[string]Image `json:"portrait_urls,omitempty"`
	// The pictures of a photo post, in order.
	Files []*PhotoFile `json:"files,omitempty"`
	// The preview of a link in it.
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
	// What a video post plays.
	Video *Video `json:"video,omitempty"`
	// The profile it is on.
	Owner *Owner `json:"owner,omitempty"`
	// Who wrote it.
	Author *Author `json:"author,omitempty"`
	// Whether the viewer likes it.
	IsLikedByViewer bool `json:"is_liked_by_viewer,omitempty"`
	// Whether the viewer has saved it. Sent to them alone, like the rest
	// of what is said here about the viewer.
	IsSavedByViewer bool `json:"is_saved_by_viewer,omitempty"`
	// Whether the viewer has reposted it.
	IsRepostedByViewer bool `json:"is_reposted_by_viewer,omitempty"`
	// What the viewer may do with it.
	Authorized *NodeAuthorized `json:"authorized,omitempty"`
}

// NewRoot is the post at the top of a reply's thread, as it is sent with a
// reply shown away from that thread: enough to say what the reply is under,
// and whose profile the conversation is on. A reply is owned by whoever
// wrote it, so its own owner says nothing about where it was said.
func NewRoot(node *entities.AnahitaNode) *Parent {
	if node == nil {
		return nil
	}

	dto := NewParent(node)
	if node.Owner != nil {
		dto.Owner = NewOwner(node.Owner)
	}

	return dto
}

// createdAtOrNil leaves the date out when there is none, where a zero time
// would be sent as the year 1.
func createdAtOrNil(node *entities.AnahitaNode) *time.Time {
	if node.CreatedAt.IsZero() {
		return nil
	}
	created := node.CreatedAt
	return &created
}
