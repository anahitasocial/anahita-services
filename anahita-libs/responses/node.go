package responses

import (
	"anahita-libs/data/entities"
	"time"
)

type NodeItem struct {
	ID            int64     `json:"id"`
	Type          string    `json:"type"`
	Name          string    `json:"name,omitempty"`
	Alias         string    `json:"alias,omitempty"`
	Body          string    `json:"body,omitempty"`
	QuoteCount    int64     `json:"quote_count,omitempty"`
	Verified      bool      `json:"verified,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	FollowerCount int64     `json:"follower_count,omitempty"`
	Mimetype      string    `json:"mimetype,omitempty"`
	Language      string    `json:"language,omitempty"`
	Access        string    `json:"access,omitempty"`
	// Set on a reply: the post at the top of its thread. Its parent, what
	// it directly answers, is sent whole as `parent`.
	RootID int64 `json:"root_id,omitempty"`
	// Pinned to the profile it is on, which it then leads.
	Pinned          bool  `json:"pinned,omitempty"`
	LikesCount      int64 `json:"likes_count,omitempty"`
	IsLikedByViewer bool  `json:"is_liked_by_viewer,omitempty"`
	// Whether the viewer has saved it. Sent to them alone, like the rest
	// of what is said here about the viewer.
	IsSavedByViewer    bool  `json:"is_saved_by_viewer,omitempty"`
	RepostCount        int64 `json:"repost_count,omitempty"`
	IsRepostedByViewer bool  `json:"is_reposted_by_viewer,omitempty"`
	// Set by the subscribe and unsubscribe handlers, which answer with the
	// node they acted on. The mapper had no field for it, so mapping their
	// response would have silently dropped the one thing it reports.
	IsSubscribedByViewer bool `json:"is_subscribed_by_viewer,omitempty"`
	IsLeadingViewer      bool `json:"is_leading_viewer,omitempty"`
	IsAdminedByViewer    bool `json:"is_admined_by_viewer,omitempty"`
	IsFollowingViewer    bool `json:"is_following_viewer,omitempty"`
	IsBlockedByViewer    bool `json:"is_blocked_by_viewer,omitempty"`

	// On an event: when it is and how many are going. Left out on anything
	// else, and where whoever sent the node did not attach it.
	Event *Event `json:"event,omitempty"`

	// On a custom feed: its picture and what it shows. Left out on
	// anything else.
	Feed *FeedBrief `json:"feed,omitempty"`

	// What the viewer may do with the item. feed-service fills in comment and
	// like, which the feed's buttons read; nothing else here computes it.
	Authorized *NodeAuthorized `json:"authorized,omitempty"`

	// Related Entities
	Author *Author `json:"author,omitempty"`
	Editor *Editor `json:"editor,omitempty"`
	Parent *Parent `json:"parent,omitempty"`
	// On a reply: the post at the top of its thread, with the profile the
	// thread is on.
	Root *Parent `json:"root,omitempty"`
	// On a note that quotes a post: that post, or why it is not shown.
	Quote   *QuoteEmbed `json:"quote,omitempty"`
	QuoteID int64       `json:"quote_id,omitempty"`
	Owner   *Owner      `json:"owner,omitempty"`

	// Media URLs
	Avatar   map[string]Image `json:"avatar_urls,omitempty"`
	Cover    map[string]Image `json:"cover_urls,omitempty"`
	Portrait map[string]Image `json:"portrait_urls,omitempty"`
	// The images of a photo post, in order. The first is the one
	// portrait_urls shows.
	Files []*PhotoFile `json:"files,omitempty"`
}

type NodeResponse struct{}

func (r *NodeResponse) MapNodeItem(node *entities.AnahitaNode) *NodeItem {
	if node == nil {
		return nil
	}

	return &NodeItem{
		ID:                   node.ID,
		Type:                 node.Type,
		Name:                 node.Name,
		Alias:                node.Alias,
		Body:                 node.Body,
		QuoteCount:           node.QuoteCount,
		RootID:               node.RootID,
		Pinned:               node.Pinned,
		Verified:             node.Verified,
		CreatedAt:            node.CreatedAt,
		FollowerCount:        node.FollowerCount,
		Mimetype:             node.Mimetype,
		Language:             node.Language,
		Access:               node.Access,
		LikesCount:           node.LikesCount,
		IsLikedByViewer:      node.IsLikedByViewer,
		IsSavedByViewer:      node.IsSavedByViewer,
		RepostCount:          node.RepostCount,
		IsRepostedByViewer:   node.IsRepostedByViewer,
		IsSubscribedByViewer: node.IsSubscribedByViewer,
		IsLeadingViewer:      node.IsLeadingViewer,
		IsAdminedByViewer:    node.IsAdminedByViewer,
		IsFollowingViewer:    node.IsFollowingViewer,
		IsBlockedByViewer:    node.IsBlockedByViewer,
		Event:                NewEvent(node),
		Feed:                 NewFeedBrief(node),
		Authorized:           NewNodeAuthorized(node),
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
		Parent: func() *Parent {
			if node.Parent != nil {
				p := NewParent(node.Parent)
				if node.Parent.Owner != nil {
					p.Owner = NewOwner(node.Parent.Owner)
				}
				return p
			}
			return nil
		}(),
		Root:    NewRoot(node.Root),
		Quote:   NewQuoteEmbed(node),
		QuoteID: node.QuoteID,
		Owner: func() *Owner {
			if node.Owner != nil {
				return NewOwner(node.Owner)
			}
			return nil
		}(),
		Avatar: func() map[string]Image {
			if node.Avatar != nil {
				return NewAvatar(node.Avatar)
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

func (r *NodeResponse) MapNodeItems(nodes []*entities.AnahitaNode) []*NodeItem {
	if nodes == nil {
		return nil
	}

	results := make([]*NodeItem, 0, len(nodes))
	for _, node := range nodes {
		results = append(results, r.MapNodeItem(node))
	}
	return results
}
