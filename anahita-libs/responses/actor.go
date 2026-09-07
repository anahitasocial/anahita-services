package responses

import (
	"anahita-libs/data/entities"
	"time"
)

type ActorNodeItem struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name,omitempty"`
	Alias string `json:"alias,omitempty"`
	Body  string `json:"body,omitempty"`
	// One link on the profile. Rendered as an anchor, so the value is
	// validated to http(s) on the way in — see requests/actor.go.
	WebsiteURL string `json:"website_url,omitempty"`
	Verified   bool   `json:"verified,omitempty"`
	// No omitempty. false is the zero value, so omitempty would drop the field
	// exactly when it carries information — a client could not tell a disabled
	// actor from one whose response simply did not mention it.
	Enabled bool `json:"enabled"`
	// Present only while the actor is scheduled for deletion, so the settings
	// page can offer Restore instead of the delete form. Deleting also clears
	// Enabled, and a client reading only that cannot tell an account on its way
	// out from one an administrator disabled.
	DeletedAt         *time.Time       `json:"deleted_at,omitempty"`
	CreationTime      time.Time        `json:"creation_time"`
	FollowerCount     int64            `json:"follower_count,omitempty"`
	LeaderCount       int64            `json:"leader_count,omitempty"`
	IsLeadingViewer   bool             `json:"is_leading_viewer,omitempty"`
	IsAdminedByViewer bool             `json:"is_admined_by_viewer,omitempty"`
	IsFollowingViewer bool             `json:"is_following_viewer,omitempty"`
	IsBlockedByViewer bool             `json:"is_blocked_by_viewer,omitempty"`
	Avatar            map[string]Image `json:"avatar_urls,omitempty"`
	Cover             map[string]Image `json:"cover_urls,omitempty"`

	// Author information
	Author *Author `json:"author,omitempty"`
}

type ActorNode struct {
	ActorNodeItem

	UpdateTime     time.Time         `json:"update_time,omitempty"`
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
		UpdateTime:    node.UpdateTime,
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
		ID:    node.ID,
		Type:  node.Type,
		Name:  node.Name,
		Alias: node.Alias,
		Body:  node.Body,
		// Declared and never assigned until this was caught: the column, the
		// proto and the response struct all carried it while the mapper did
		// not, so it stored correctly and never appeared on the wire.
		WebsiteURL: node.WebsiteURL,
		Verified:   node.Verified,
		// Declared since this struct was written and never assigned, so
		// `enabled` has never appeared in the API at all and no client could
		// tell an enabled actor from a disabled one.
		Enabled:      node.Enabled,
		CreationTime: node.CreationTime,
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
