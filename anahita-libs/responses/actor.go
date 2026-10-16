package responses

import (
	"anahita-libs/data/entities"
	"time"
)

// ActorNodeItem is a profile: a person, a group or an event.
type ActorNodeItem struct {
	// Its id.
	ID int64 `json:"id"`
	// What it is called on the server it came from, and that server, when it
	// came from another. Left out of everything made here.
	URI string `json:"uri,omitempty"`
	// The server it came from, when it came from another.
	OriginDomain string `json:"origin_domain,omitempty"`
	// What kind of profile it is: `node.actor.actor-service.person.v1`,
	// `...group.v1` or `...event.v1`, or one of another server's.
	Type string `json:"type"`
	// Its name.
	Name string `json:"name,omitempty"`
	// Its handle (`ana`, or `ana@mastodon.social` for somebody on another
	// server).
	Alias string `json:"alias,omitempty"`
	// Its description.
	Body string `json:"body,omitempty"`
	// One link on the profile. Rendered as an anchor, so the value is
	// validated to http(s) on the way in — see requests/actor.go.
	WebsiteURL string `json:"website_url,omitempty"`
	// A mark kept from earlier versions of Anahita. Nothing sets it yet.
	Verified bool `json:"verified,omitempty"`
	// Whether it is enabled: false when a site administrator has disabled it,
	// or while it is being deleted.
	//
	// No omitempty. false is the zero value, so omitempty would drop the field
	// exactly when it carries information — a client could not tell a disabled
	// actor from one whose response simply did not mention it.
	Enabled bool `json:"enabled"`
	// Present only while the actor is scheduled for deletion, so the settings
	// page can offer Restore instead of the delete form. Deleting also clears
	// Enabled, and a client reading only that cannot tell an account on its way
	// out from one an administrator disabled.
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	// Who may see it: `public`, `registered`, `followers`, `leaders`,
	// `mutuals`, `admins` or `myself` (permissions.md).
	//
	// Absent from the response entirely until now, which broke the settings
	// card that edits them: it seeds its dropdown from actor.access, got
	// undefined, and saving wrote whatever the control defaulted to over the
	// real setting. A field the client is expected to EDIT has to be readable.
	//
	// No omitempty on Access — it is never legitimately empty, and dropping it
	// would put the card back in the state this fixes.
	//
	// AND NONE ON AllowFollowRequest, for the same reason and with more
	// force, because omitempty on a BOOL omits false — the ordinary
	// value, not an edge case. The access card read undefined, stored
	// it, and sent the string "undefined" in the form body; Echo cannot
	// bind that to a bool, so the whole request was rejected with a 400
	// before validation ran and no access change could be saved at all
	// by anybody whose follow-request setting was off.
	Access string `json:"access"`
	// Whether people its profile is not open to may ask to follow it.
	AllowFollowRequest bool `json:"allow_follow_request"`
	// Whether what this actor reposts is also among its profile's posts.
	ShowRepostsOnProfile bool `json:"show_reposts_on_profile"`
	// On a person: who may quote their posts, when a post does not say.
	// "anyone", "followers" or "nobody"; left out when they have not said,
	// which is anyone.
	QuotePolicy string `json:"quote_policy,omitempty"`
	// What this actor's new photos and videos start with: whether they may
	// be downloaded.
	AllowDownload bool `json:"allow_download"`
	// Permanent, and set only once archived, so the settings page can show the
	// state rather than offering an action that no longer applies.
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	// Set only while the installation features this actor, so a super
	// administrator's profile menu can show the toggle's state.
	FeaturedAt *time.Time `json:"featured_at,omitempty"`
	// When it was made.
	CreatedAt time.Time `json:"created_at"`
	// How many follow it.
	FollowerCount int64 `json:"follower_count,omitempty"`
	// How many it follows.
	LeaderCount int64 `json:"leader_count,omitempty"`
	// Whether the viewer follows it.
	IsLeadingViewer bool `json:"is_leading_viewer,omitempty"`
	// Whether the viewer is one of its administrators.
	IsAdminedByViewer bool `json:"is_admined_by_viewer,omitempty"`
	// Whether it follows the viewer.
	IsFollowingViewer bool `json:"is_following_viewer,omitempty"`
	// Whether the viewer has blocked it.
	IsBlockedByViewer bool `json:"is_blocked_by_viewer,omitempty"`
	// Whether the viewer has been invited to follow this actor and is yet
	// to answer.
	IsInvited bool `json:"is_invited,omitempty"`
	// Its picture, in each size there is: `small`, `medium`, `large`,
	// `square`.
	Avatar map[string]Image `json:"avatar_urls,omitempty"`
	// Its cover picture, in each size there is: `medium`, `large`.
	Cover map[string]Image `json:"cover_urls,omitempty"`

	// Author information
	Author *Author `json:"author,omitempty"`

	// On an event: when it is, how many may go and are going, and what the
	// viewer answered. Left out on anything else.
	Event *Event `json:"event,omitempty"`
}

type ActorNode struct {
	ActorNodeItem

	UpdatedAt      time.Time         `json:"updated_at,omitempty"`
	Features       []*NodeFeature    `json:"features,omitempty"`
	Authorized     *NodeAuthorized   `json:"authorized,omitempty"`
	Permissions    map[string]string `json:"permissions,omitempty"`
	Administrators []*Admin          `json:"administrators,omitempty"`

	Editor *Editor `json:"editor,omitempty"`
}

type ActorResponse struct{}

// MapNode maps an AnahitaNode entity to an ActorNode response.
func (r *ActorResponse) MapNode(node *entities.AnahitaNode) *ActorNode {
	if node == nil {
		return nil
	}

	return &ActorNode{
		ActorNodeItem: *r.MapNodeItem(node),
		UpdatedAt:     node.UpdatedAt,
		Editor: func() *Editor {
			if node.Editor != nil && node.Editor.ID != 0 {
				return NewEditor(node.Editor)
			}
			return nil
		}(),
		Features: func() []*NodeFeature {
			var features []*NodeFeature
			for _, feature := range node.Features {
				features = append(features, NewNodeFeature(feature))
			}
			return features
		}(),
		Authorized: func() *NodeAuthorized {
			if node.Authorized != nil {
				return NewNodeAuthorized(node)
			}
			return nil
		}(),
		Permissions: node.Permissions,
		Administrators: func() []*Admin {
			var admins []*Admin
			for _, adminNode := range node.Administrators {
				admins = append(admins, NewAdmin(adminNode))
			}
			return admins
		}(),
	}
}

// MapNodeItem maps an AnahitaNode entity to an ActorNodeItem response.
func (r *ActorResponse) MapNodeItem(node *entities.AnahitaNode) *ActorNodeItem {
	if node == nil {
		return nil
	}

	return &ActorNodeItem{
		ID:           node.ID,
		URI:          node.URI,
		OriginDomain: node.OriginDomain,
		Type:         node.Type,
		Name:         node.Name,
		Alias:        node.Alias,
		Body:         node.Body,
		// Declared and never assigned until this was caught: the column, the
		// proto and the response struct all carried it while the mapper did
		// not, so it stored correctly and never appeared on the wire.
		WebsiteURL: node.WebsiteURL,
		Verified:   node.Verified,
		// Declared since this struct was written and never assigned, so
		// `enabled` has never appeared in the API at all and no client could
		// tell an enabled actor from a disabled one.
		Enabled:              node.Enabled,
		Access:               node.Access,
		AllowFollowRequest:   node.AllowFollowRequest,
		ShowRepostsOnProfile: node.ShowRepostsOnProfile,
		QuotePolicy:          node.QuotePolicy,
		AllowDownload:        node.AllowDownload,
		ArchivedAt:           node.ArchivedAt,
		FeaturedAt:           node.FeaturedAt,
		CreatedAt:            node.CreatedAt,
		// nil for everything not scheduled for deletion, and omitempty keeps it
		// off the wire in that case — which is every actor but a handful.
		DeletedAt: node.DeletedAt,
		Author: func() *Author {
			if node.Author != nil && node.Author.ID != 0 {
				return NewAuthor(node.Author)
			}
			return nil
		}(),
		FollowerCount:     node.FollowerCount,
		LeaderCount:       node.LeaderCount,
		IsLeadingViewer:   node.IsLeadingViewer,
		IsAdminedByViewer: node.IsAdminedByViewer,
		IsFollowingViewer: node.IsFollowingViewer,
		IsBlockedByViewer: node.IsBlockedByViewer,
		IsInvited:         node.IsInvitedViewer,
		Event:             NewEvent(node),
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
	}
}

func (r *ActorResponse) MapNodeItems(actors []*entities.AnahitaNode) []*ActorNodeItem {
	var items []*ActorNodeItem
	for _, actor := range actors {
		items = append(items, r.MapNodeItem(actor))
	}
	return items
}
