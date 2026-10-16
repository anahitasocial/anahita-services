package responses

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/videos"
	"time"
)

// MediumNodeItem is a post or a reply in a list.
type MediumNodeItem struct {
	// Its id.
	ID int64 `json:"id"`
	// What it is called on the server it came from, and that server, when it
	// came from another. Left out of everything made here.
	URI string `json:"uri,omitempty"`
	// The server it came from, when it came from another.
	OriginDomain string `json:"origin_domain,omitempty"`
	// What kind of post it is: `node.medium.text-service.note.v1` for a note,
	// and so on.
	Type string `json:"type"`
	// Its title, where it has one.
	Name string `json:"name,omitempty"`
	// Its short name, where it has one.
	Alias string `json:"alias,omitempty"`
	// Its words, as Markdown.
	Body string `json:"body,omitempty"`
	// When it was written.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// The kind of file it holds, where it holds one.
	Mimetype string `json:"mimetype,omitempty"`
	// The language the author said it is written in. Left out when they did
	// not say.
	Language string `json:"language,omitempty"`
	// On a reply: the post or reply it answers, and the post at the top of
	// its thread. Left out on anything that is not a reply.
	ParentID int64 `json:"parent_id,omitempty"`
	// On a reply: the post at the top of its thread.
	RootID int64 `json:"root_id,omitempty"`
	// On a note that carries one: the preview of a link in it.
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
	// The cards of the pages it shows within its text, an article's or a
	// topic's: an address on a line of its own each.
	LinkPreviews []*LinkPreview `json:"link_previews,omitempty"`
	// On a photo post from another server whose pictures are no longer
	// stored here.
	MediaRemoved *MediaRemoved `json:"media_removed,omitempty"`
	// What a video post plays.
	Video *Video `json:"video,omitempty"`
	// On a note that quotes a post: that post, or why it is not shown.
	Quote *QuoteEmbed `json:"quote,omitempty"`
	// The id of the post it quotes, whether or not it is shown.
	QuoteID int64 `json:"quote_id,omitempty"`
	// On a reply: removed, and kept as an empty placeholder because other
	// replies hang from it. It carries no text and no author.
	Deleted bool `json:"deleted,omitempty"`
	// On a reply: hidden from its thread. Sent only to somebody who may
	// show it again.
	Hidden bool `json:"hidden,omitempty"`
	// Who may see it: `public`, `registered`, `followers`, `leaders`,
	// `mutuals`, `admins` or `myself` (permissions.md).
	Access string `json:"access,omitempty"`
	// How many like it.
	LikesCount int64 `json:"likes_count"`
	// How many notes quote it.
	QuoteCount int64 `json:"quote_count"`
	// How many have reposted it.
	RepostCount int64 `json:"repost_count"`
	// How many replies are under it.
	CommentCount int64 `json:"comment_count"`
	// Whether the viewer likes it.
	IsLikedByViewer bool `json:"is_liked_by_viewer"`
	// Whether the viewer has saved it. Sent to them alone, like the rest
	// of what is said here about the viewer.
	IsSavedByViewer bool `json:"is_saved_by_viewer,omitempty"`
	// Whether the viewer has reposted it.
	IsRepostedByViewer bool `json:"is_reposted_by_viewer"`

	// Related Entities
	Author *Author `json:"author,omitempty"`
	// The profile it is on.
	Owner *Owner `json:"owner,omitempty"`

	// Media URLs
	Cover map[string]Image `json:"cover_urls,omitempty"`
	// A photo post's first picture, in each size there is: `square`,
	// `small`, `medium`, `large`, `original`.
	Portrait map[string]Image `json:"portrait_urls,omitempty"`
	// The images of a photo post, in order. The first is the one
	// portrait_urls shows.
	Files []*PhotoFile `json:"files,omitempty"`
}

// MediumNode is a post or a reply, read on its own or answered with after a
// change.
type MediumNode struct {
	// Its id.
	ID int64 `json:"id"`
	// What it is called on the server it came from, and that server, when it
	// came from another. Left out of everything made here.
	URI string `json:"uri,omitempty"`
	// The server it came from, when it came from another.
	OriginDomain string `json:"origin_domain,omitempty"`
	// What kind of post it is: `node.medium.text-service.note.v1` for a note,
	// and so on.
	Type string `json:"type"`
	// Its title, where it has one.
	Name string `json:"name,omitempty"`
	// Its short name, where it has one.
	Alias string `json:"alias,omitempty"`
	// Its words, as Markdown.
	Body string `json:"body,omitempty"`
	// When it was written.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// When it was last changed.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	// The kind of file it holds, where it holds one.
	Mimetype string `json:"mimetype,omitempty"`
	// The language it is written in, as its author said (`en`, `fa`).
	Language string `json:"language,omitempty"`
	// As on MediumNodeItem.
	ParentID int64 `json:"parent_id,omitempty"`
	// On a reply: the post at the top of its thread.
	RootID int64 `json:"root_id,omitempty"`
	// On a note that carries one: the preview of a link in it.
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
	// The cards of the pages it shows within its text, an article's or a
	// topic's: an address on a line of its own each.
	LinkPreviews []*LinkPreview `json:"link_previews,omitempty"`
	// On a photo post from another server whose pictures are no longer
	// stored here.
	MediaRemoved *MediaRemoved `json:"media_removed,omitempty"`
	// What a video post plays.
	Video *Video `json:"video,omitempty"`
	// On a note that quotes a post: that post, or why it is not shown.
	Quote *QuoteEmbed `json:"quote,omitempty"`
	// The id of the post it quotes, whether or not it is shown.
	QuoteID int64 `json:"quote_id,omitempty"`
	// On a reply read on its own: the post at the top of its thread.
	Root *Parent `json:"root,omitempty"`
	// Who may quote this post, when its author has said: "anyone",
	// "followers" or "nobody". Left out when they have not, and their own
	// setting applies.
	QuotePolicy string `json:"quote_policy,omitempty"`
	// Whether whoever can see it may download what it holds. Sent on a post
	// that holds a file, for whoever may change it.
	AllowDownload *bool `json:"allow_download,omitempty"`
	// That it is in its author's media library only: put up to be shown in
	// an article, and listed nowhere else.
	LibraryOnly bool `json:"library_only,omitempty"`
	// That it is a draft, an article or a topic still being written, seen
	// by its author alone.
	Draft bool `json:"draft,omitempty"`
	// How many people cite it, each once, never its author.
	CitationCount int64 `json:"citation_count"`
	// On a reply: removed, and kept as an empty placeholder because other
	// replies hang from it. It carries no text and no author.
	Deleted bool `json:"deleted,omitempty"`
	// On a reply: hidden from its thread. Sent only to somebody who may
	// show it again.
	Hidden bool `json:"hidden,omitempty"`
	// Who may see it: `public`, `registered`, `followers`, `leaders`,
	// `mutuals`, `admins` or `myself` (permissions.md).
	Access string `json:"access,omitempty"`
	// How many times it has been read.
	ViewCount int64 `json:"view_count"`
	// How many like it.
	LikesCount int64 `json:"likes_count"`
	// How many notes quote it.
	QuoteCount int64 `json:"quote_count"`
	// How many have reposted it.
	RepostCount int64 `json:"repost_count"`
	// How many replies are under it.
	CommentCount int64 `json:"comment_count"`
	// Whether it takes replies at all.
	CommentStatus bool `json:"comment_status"`
	// Who may reply, as the author set it: "anyone", "nobody", or any of
	// "followers", "following" and "mentioned" joined by commas. Not on a
	// reply, which follows the post at the top of its thread.
	ReplyAccess string `json:"reply_access,omitempty"`
	// Whether it is pinned to the profile it is on, which it then leads.
	Pinned bool `json:"pinned"`
	// Whether it is shown: false when it has been disabled.
	Enabled bool `json:"enabled"`

	// Whether the viewer likes it.
	IsLikedByViewer bool `json:"is_liked_by_viewer"`
	// Whether the viewer has saved it. Sent to them alone, like the rest
	// of what is said here about the viewer.
	IsSavedByViewer bool `json:"is_saved_by_viewer,omitempty"`
	// Whether the viewer has reposted it.
	IsRepostedByViewer bool `json:"is_reposted_by_viewer"`

	// Authorization Information
	Authorized *NodeAuthorized `json:"authorized,omitempty"`

	// Related Entities
	Author *Author `json:"author,omitempty"`
	// Who changed it last.
	Editor *Editor `json:"editor,omitempty"`
	// The profile it is on.
	Owner *Owner `json:"owner,omitempty"`

	// Media URLs
	Cover map[string]Image `json:"cover_urls,omitempty"`
	// A photo post's first picture, in each size there is: `square`,
	// `small`, `medium`, `large`, `original`.
	Portrait map[string]Image `json:"portrait_urls,omitempty"`
	// The images of a photo post, in order. The first is the one
	// portrait_urls shows.
	Files []*PhotoFile `json:"files,omitempty"`
}

type MediumNodeResponse struct{}

func (r *MediumNodeResponse) MapMediumNodeItem(node *entities.AnahitaNode) *MediumNodeItem {
	if node == nil {
		return nil
	}

	return &MediumNodeItem{
		ID:                 node.ID,
		URI:                node.URI,
		OriginDomain:       node.OriginDomain,
		Type:               node.Type,
		Name:               node.Name,
		Alias:              node.Alias,
		Body:               node.Body,
		QuoteCount:         node.QuoteCount,
		CreatedAt:          node.CreatedAt,
		Mimetype:           node.Mimetype,
		Language:           node.Language,
		ParentID:           replyParent(node),
		RootID:             node.RootID,
		LinkPreview:        NewLinkPreview(node),
		LinkPreviews:       NewLinkPreviews(node),
		MediaRemoved:       NewMediaRemoved(node),
		Video:              NewVideo(node),
		Quote:              NewQuoteEmbed(node),
		QuoteID:            node.QuoteID,
		Deleted:            node.RootID > 0 && node.DeletedAt != nil,
		Hidden:             node.Hidden,
		Access:             node.Access,
		LikesCount:         node.LikesCount,
		IsLikedByViewer:    node.IsLikedByViewer,
		IsSavedByViewer:    node.IsSavedByViewer,
		RepostCount:        node.RepostCount,
		IsRepostedByViewer: node.IsRepostedByViewer,
		CommentCount:       node.CommentCount,
		Author: func() *Author {
			if node.Author != nil {
				return NewAuthor(node.Author)
			}
			return nil
		}(),
		Owner: func() *Owner {
			if node.Owner != nil {
				return NewOwner(node.Owner)
			}
			return nil
		}(),
		Cover: func() map[string]Image {
			if node.Cover != nil {
				return NewCover(node.Cover)
			}
			return nil
		}(),
		Portrait: func() map[string]Image {
			if node.Portrait != nil {
				return NewPortrait(node.Portrait)
			}
			return nil
		}(),
		Files: NewPhotoFiles(node.Files),
	}
}

func (r *MediumNodeResponse) MapMediumNode(node *entities.AnahitaNode) *MediumNode {
	if node == nil {
		return nil
	}

	return &MediumNode{
		ID:                 node.ID,
		URI:                node.URI,
		OriginDomain:       node.OriginDomain,
		Type:               node.Type,
		Name:               node.Name,
		Alias:              node.Alias,
		Body:               node.Body,
		QuoteCount:         node.QuoteCount,
		CreatedAt:          node.CreatedAt,
		UpdatedAt:          node.UpdatedAt,
		Mimetype:           node.Mimetype,
		Language:           node.Language,
		ParentID:           replyParent(node),
		RootID:             node.RootID,
		LinkPreview:        NewLinkPreview(node),
		LinkPreviews:       NewLinkPreviews(node),
		MediaRemoved:       NewMediaRemoved(node),
		Video:              NewVideo(node),
		Quote:              NewQuoteEmbed(node),
		QuoteID:            node.QuoteID,
		Root:               NewRoot(node.Root),
		QuotePolicy:        node.QuotePolicy,
		AllowDownload:      allowDownloadOf(node),
		LibraryOnly:        node.LibraryOnly,
		Draft:              node.Draft,
		CitationCount:      node.CitationCount,
		Deleted:            node.RootID > 0 && node.DeletedAt != nil,
		Hidden:             node.Hidden,
		Access:             node.Access,
		LikesCount:         node.LikesCount,
		IsLikedByViewer:    node.IsLikedByViewer,
		IsSavedByViewer:    node.IsSavedByViewer,
		RepostCount:        node.RepostCount,
		IsRepostedByViewer: node.IsRepostedByViewer,
		CommentCount:       node.CommentCount,
		CommentStatus:      node.CommentStatus,
		ReplyAccess:        replyAccess(node),
		Pinned:             node.Pinned,
		ViewCount:          node.ViewCount,
		Authorized: func() *NodeAuthorized {
			if node.Authorized != nil {
				return NewNodeAuthorized(node)
			}
			return nil
		}(),
		Author: func() *Author {
			if node.Author != nil {
				return NewAuthor(node.Author)
			}
			return nil
		}(),
		Editor: func() *Editor {
			if node.Editor != nil {
				return NewEditor(node.Editor)
			}
			return nil
		}(),
		Owner: func() *Owner {
			if node.Owner != nil {
				return NewOwner(node.Owner)
			}
			return nil
		}(),
		Cover: func() map[string]Image {
			if node.Cover != nil {
				return NewCover(node.Cover)
			}
			return nil
		}(),
		Portrait: func() map[string]Image {
			if node.Portrait != nil {
				return NewPortrait(node.Portrait)
			}
			return nil
		}(),
		Files: NewPhotoFiles(node.Files),
	}
}

func (r *MediumNodeResponse) MapMediumNodeItems(nodes []*entities.AnahitaNode) []*MediumNodeItem {
	if nodes == nil {
		return nil
	}

	results := make([]*MediumNodeItem, 0, len(nodes))
	for _, node := range nodes {
		results = append(results, r.MapMediumNodeItem(node))
	}
	return results
}

// replyParent is what a reply answers. A post that is not a reply reports
// none, even where an older kind of row used parent_id for something else.
func replyParent(node *entities.AnahitaNode) int64 {
	if node.RootID <= 0 {
		return 0
	}
	return node.ParentID
}

// replyAccess is who may reply to a post, as it is read. Nothing for a
// reply.
func replyAccess(node *entities.AnahitaNode) string {
	if node.RootID > 0 {
		return ""
	}

	return constants.EffectiveReplyAccess(node.ReplyAccess, node.CommentStatus)
}

// allowDownloadOf is a post's download switch, for a post that has one: a
// note has nothing to download and says nothing about it.
func allowDownloadOf(node *entities.AnahitaNode) *bool {
	if node == nil || (node.Type != constants.NodeMediumPhotoEntityType && node.Type != videos.NodeType) {
		return nil
	}
	allow := node.AllowDownload
	return &allow
}
