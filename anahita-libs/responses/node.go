package responses

import (
	"anahita-libs/data/entities"
	"time"
)

// NodeItem is a post, a reply, a profile or anything else there is, as a
// list or a read sends it: what it is not is left out.
type NodeItem struct {
	// Its id.
	ID int64 `json:"id"`
	// What it is called on the server it came from, and that server, when it
	// came from another. Left out of everything made here.
	URI string `json:"uri,omitempty"`
	// The server it came from, when it came from another.
	OriginDomain string `json:"origin_domain,omitempty"`
	// What kind of thing it is: `node.medium.text-service.note.v1` for a
	// note, `node.actor.actor-service.person.v1` for a person, and so on.
	Type string `json:"type"`
	// A profile's name, or a post's title where it has one.
	Name string `json:"name,omitempty"`
	// A profile's handle (`ana`, or `ana@mastodon.social` for somebody on
	// another server), or a hashtag's name.
	Alias string `json:"alias,omitempty"`
	// Its words: a post's text, as Markdown, or a profile's description.
	Body string `json:"body,omitempty"`
	// How many notes quote it.
	QuoteCount int64 `json:"quote_count,omitempty"`
	// A mark kept from earlier versions of Anahita. Nothing sets it yet.
	Verified bool `json:"verified,omitempty"`
	// When it was made.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// How many follow it, on a profile.
	FollowerCount int64 `json:"follower_count,omitempty"`
	// The kind of file it holds, where it holds one.
	Mimetype string `json:"mimetype,omitempty"`
	// The language it is written in, as its author said (`en`, `fa`).
	Language string `json:"language,omitempty"`
	// Who may see it: `public`, `registered`, `followers`, `leaders`,
	// `mutuals`, `admins` or `myself` (permissions.md).
	Access string `json:"access,omitempty"`
	// Set on a reply: the post at the top of its thread. Its parent, what
	// it directly answers, is sent whole as `parent`.
	RootID int64 `json:"root_id,omitempty"`
	// Pinned to the profile it is on, which it then leads.
	Pinned bool `json:"pinned,omitempty"`
	// How many like it.
	LikesCount int64 `json:"likes_count,omitempty"`
	// Whether the viewer likes it.
	IsLikedByViewer bool `json:"is_liked_by_viewer,omitempty"`
	// Whether the viewer has saved it. Sent to them alone, like the rest
	// of what is said here about the viewer.
	IsSavedByViewer bool `json:"is_saved_by_viewer,omitempty"`
	// How many have reposted it.
	RepostCount int64 `json:"repost_count,omitempty"`
	// How many replies are under it. A post in a feed said nothing of
	// this, so its interactions counted its replies as none.
	CommentCount int64 `json:"comment_count,omitempty"`
	// Whether the viewer has reposted it.
	IsRepostedByViewer bool `json:"is_reposted_by_viewer,omitempty"`
	// Set by the subscribe and unsubscribe handlers, which answer with the
	// node they acted on. The mapper had no field for it, so mapping their
	// response would have silently dropped the one thing it reports.
	IsSubscribedByViewer bool `json:"is_subscribed_by_viewer,omitempty"`
	// On a profile: whether the viewer follows it.
	IsLeadingViewer bool `json:"is_leading_viewer,omitempty"`
	// On a profile: whether the viewer is one of its administrators.
	IsAdminedByViewer bool `json:"is_admined_by_viewer,omitempty"`
	// On a profile: whether it follows the viewer.
	IsFollowingViewer bool `json:"is_following_viewer,omitempty"`
	// On a profile: whether the viewer has blocked it.
	IsBlockedByViewer bool `json:"is_blocked_by_viewer,omitempty"`

	// On an event: when it is and how many are going. Left out on anything
	// else, and where whoever sent the node did not attach it.
	Event *Event `json:"event,omitempty"`

	// On a note that carries one: the preview of a link in it.
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
	// On a photo post from another server whose pictures are no longer
	// stored here.
	MediaRemoved *MediaRemoved `json:"media_removed,omitempty"`
	// What a video post plays.
	Video *Video `json:"video,omitempty"`

	// On a custom feed: its picture and what it shows. Left out on
	// anything else.
	Feed *FeedBrief `json:"feed,omitempty"`

	// What the viewer may do with the item. feed-service fills in comment and
	// like, which the feed's buttons read; nothing else here computes it.
	Authorized *NodeAuthorized `json:"authorized,omitempty"`

	// Related Entities
	Author *Author `json:"author,omitempty"`
	// Who changed it last.
	Editor *Editor `json:"editor,omitempty"`
	// What it answers, on a reply; what it reposts, on a repost.
	Parent *Parent `json:"parent,omitempty"`
	// On a reply: the post at the top of its thread, with the profile the
	// thread is on.
	Root *Parent `json:"root,omitempty"`
	// On a note that quotes a post: that post, or why it is not shown.
	Quote *QuoteEmbed `json:"quote,omitempty"`
	// The id of the post it quotes, on a note that quotes one.
	QuoteID int64 `json:"quote_id,omitempty"`
	// The profile it is on.
	Owner *Owner `json:"owner,omitempty"`

	// Media URLs
	Avatar map[string]Image `json:"avatar_urls,omitempty"`
	// A profile's cover picture, in each size there is: `medium`, `large`.
	Cover map[string]Image `json:"cover_urls,omitempty"`
	// A photo post's first picture, in each size there is: `square`,
	// `small`, `medium`, `large`, `original`.
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
		URI:                  node.URI,
		OriginDomain:         node.OriginDomain,
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
		CommentCount:         node.CommentCount,
		IsRepostedByViewer:   node.IsRepostedByViewer,
		IsSubscribedByViewer: node.IsSubscribedByViewer,
		IsLeadingViewer:      node.IsLeadingViewer,
		IsAdminedByViewer:    node.IsAdminedByViewer,
		IsFollowingViewer:    node.IsFollowingViewer,
		IsBlockedByViewer:    node.IsBlockedByViewer,
		Event:                NewEvent(node),
		Feed:                 NewFeedBrief(node),
		LinkPreview:          NewLinkPreview(node),
		MediaRemoved:         NewMediaRemoved(node),
		Video:                NewVideo(node),
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
