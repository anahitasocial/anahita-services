package responses

import (
	"anahita-libs/data/entities"
	"time"
)

type MediumNodeItem struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name,omitempty"`
	Alias     string    `json:"alias,omitempty"`
	Body      string    `json:"body,omitempty"`
	Excerpt   string    `json:"excerpt,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Mimetype  string    `json:"mimetype,omitempty"`
	// The language the author said it is written in. Left out when they did
	// not say.
	Language string `json:"language,omitempty"`
	// On a reply: the post or reply it answers, and the post at the top of
	// its thread. Left out on anything that is not a reply.
	ParentID int64 `json:"parent_id,omitempty"`
	RootID   int64 `json:"root_id,omitempty"`
	// On a reply: removed, and kept as an empty placeholder because other
	// replies hang from it. It carries no text and no author.
	Deleted bool `json:"deleted,omitempty"`
	// On a reply: hidden from its thread. Sent only to somebody who may
	// show it again.
	Hidden             bool   `json:"hidden,omitempty"`
	Access             string `json:"access,omitempty"`
	LikesCount         int64  `json:"likes_count"`
	QuoteCount         int64  `json:"quote_count"`
	RepostCount        int64  `json:"repost_count"`
	CommentCount       int64  `json:"comment_count"`
	IsLikedByViewer    bool   `json:"is_liked_by_viewer"`
	IsRepostedByViewer bool   `json:"is_reposted_by_viewer"`

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
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name,omitempty"`
	Alias     string    `json:"alias,omitempty"`
	Body      string    `json:"body,omitempty"`
	Excerpt   string    `json:"excerpt,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Mimetype  string    `json:"mimetype,omitempty"`
	Language  string    `json:"language,omitempty"`
	// As on MediumNodeItem.
	ParentID      int64  `json:"parent_id,omitempty"`
	RootID        int64  `json:"root_id,omitempty"`
	Deleted       bool   `json:"deleted,omitempty"`
	Hidden        bool   `json:"hidden,omitempty"`
	Access        string `json:"access,omitempty"`
	ViewCount     int64  `json:"view_count"`
	LikesCount    int64  `json:"likes_count"`
	QuoteCount    int64  `json:"quote_count"`
	RepostCount   int64  `json:"repost_count"`
	CommentCount  int64  `json:"comment_count"`
	CommentStatus bool   `json:"comment_status"`
	Pinned        bool   `json:"pinned"`
	Enabled       bool   `json:"enabled"`

	IsLikedByViewer    bool `json:"is_liked_by_viewer"`
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
		Deleted:            node.RootID > 0 && node.DeletedAt != nil,
		Hidden:             node.Hidden,
		Access:             node.Access,
		LikesCount:         node.LikesCount,
		IsLikedByViewer:    node.IsLikedByViewer,
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
		Deleted:            node.RootID > 0 && node.DeletedAt != nil,
		Hidden:             node.Hidden,
		Access:             node.Access,
		LikesCount:         node.LikesCount,
		IsLikedByViewer:    node.IsLikedByViewer,
		RepostCount:        node.RepostCount,
		IsRepostedByViewer: node.IsRepostedByViewer,
		CommentCount:       node.CommentCount,
		CommentStatus:      node.CommentStatus,
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
