package responses

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/videos"
	"time"
)

type MediumNodeItem struct {
	ID int64 `json:"id"`
	// What it is called on the server it came from, and that server, when it
	// came from another. Left out of everything made here.
	URI          string    `json:"uri,omitempty"`
	OriginDomain string    `json:"origin_domain,omitempty"`
	Type         string    `json:"type"`
	Name         string    `json:"name,omitempty"`
	Alias        string    `json:"alias,omitempty"`
	Body         string    `json:"body,omitempty"`
	Excerpt      string    `json:"excerpt,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	Mimetype     string    `json:"mimetype,omitempty"`
	// The language the author said it is written in. Left out when they did
	// not say.
	Language string `json:"language,omitempty"`
	// On a reply: the post or reply it answers, and the post at the top of
	// its thread. Left out on anything that is not a reply.
	ParentID int64 `json:"parent_id,omitempty"`
	RootID   int64 `json:"root_id,omitempty"`
	// On a note that quotes a post: that post, or why it is not shown.
	// On a note that carries one: the preview of a link in it.
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
	// What a video post plays.
	Video *Video      `json:"video,omitempty"`
	Quote *QuoteEmbed `json:"quote,omitempty"`
	// The id of the post it quotes, whether or not it is shown.
	QuoteID int64 `json:"quote_id,omitempty"`
	// On a reply: removed, and kept as an empty placeholder because other
	// replies hang from it. It carries no text and no author.
	Deleted bool `json:"deleted,omitempty"`
	// On a reply: hidden from its thread. Sent only to somebody who may
	// show it again.
	Hidden          bool   `json:"hidden,omitempty"`
	Access          string `json:"access,omitempty"`
	LikesCount      int64  `json:"likes_count"`
	QuoteCount      int64  `json:"quote_count"`
	RepostCount     int64  `json:"repost_count"`
	CommentCount    int64  `json:"comment_count"`
	IsLikedByViewer bool   `json:"is_liked_by_viewer"`
	// Whether the viewer has saved it. Sent to them alone, like the rest
	// of what is said here about the viewer.
	IsSavedByViewer    bool `json:"is_saved_by_viewer,omitempty"`
	IsRepostedByViewer bool `json:"is_reposted_by_viewer"`

	// Related Entities
	Author *Author `json:"author,omitempty"`
	Owner  *Owner  `json:"owner,omitempty"`

	// Media URLs
	Cover    map[string]Image `json:"cover_urls,omitempty"`
	Portrait map[string]Image `json:"portrait_urls,omitempty"`
	// The images of a photo post, in order. The first is the one
	// portrait_urls shows.
	Files []*PhotoFile `json:"files,omitempty"`
}

type MediumNode struct {
	ID int64 `json:"id"`
	// What it is called on the server it came from, and that server, when it
	// came from another. Left out of everything made here.
	URI          string    `json:"uri,omitempty"`
	OriginDomain string    `json:"origin_domain,omitempty"`
	Type         string    `json:"type"`
	Name         string    `json:"name,omitempty"`
	Alias        string    `json:"alias,omitempty"`
	Body         string    `json:"body,omitempty"`
	Excerpt      string    `json:"excerpt,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
	Mimetype     string    `json:"mimetype,omitempty"`
	Language     string    `json:"language,omitempty"`
	// As on MediumNodeItem.
	ParentID int64 `json:"parent_id,omitempty"`
	RootID   int64 `json:"root_id,omitempty"`
	// On a note that quotes a post: that post, or why it is not shown.
	// On a note that carries one: the preview of a link in it.
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
	// What a video post plays.
	Video *Video      `json:"video,omitempty"`
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
	AllowDownload *bool  `json:"allow_download,omitempty"`
	Deleted       bool   `json:"deleted,omitempty"`
	Hidden        bool   `json:"hidden,omitempty"`
	Access        string `json:"access,omitempty"`
	ViewCount     int64  `json:"view_count"`
	LikesCount    int64  `json:"likes_count"`
	QuoteCount    int64  `json:"quote_count"`
	RepostCount   int64  `json:"repost_count"`
	CommentCount  int64  `json:"comment_count"`
	CommentStatus bool   `json:"comment_status"`
	// Who may reply, as the author set it: "anyone", "nobody", or any of
	// "followers", "following" and "mentioned" joined by commas. Not on a
	// reply, which follows the post at the top of its thread.
	ReplyAccess string `json:"reply_access,omitempty"`
	Pinned      bool   `json:"pinned"`
	Enabled     bool   `json:"enabled"`

	IsLikedByViewer bool `json:"is_liked_by_viewer"`
	// Whether the viewer has saved it. Sent to them alone, like the rest
	// of what is said here about the viewer.
	IsSavedByViewer    bool `json:"is_saved_by_viewer,omitempty"`
	IsRepostedByViewer bool `json:"is_reposted_by_viewer"`

	// Authorization Information
	Authorized *NodeAuthorized `json:"authorized,omitempty"`

	// Related Entities
	Author *Author `json:"author,omitempty"`
	Editor *Editor `json:"editor,omitempty"`
	Owner  *Owner  `json:"owner,omitempty"`

	// Media URLs
	Cover    map[string]Image `json:"cover_urls,omitempty"`
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
		Video:              NewVideo(node),
		Quote:              NewQuoteEmbed(node),
		QuoteID:            node.QuoteID,
		Root:               NewRoot(node.Root),
		QuotePolicy:        node.QuotePolicy,
		AllowDownload:      allowDownloadOf(node),
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
