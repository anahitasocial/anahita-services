package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"time"
)

// PersonNodeItem is an actor node plus the two person-only fields.
//
// What left: email, which is a login identifier and has no business on a public
// profile; given_name and family_name, replaced by the actor's `name`; gender,
// replaced by free-text pronouns; and username, which is the actor's `alias`
// and was being sent twice under two names.
type PersonNodeItem struct {
	responses.ActorNodeItem

	PersonType     string `json:"person_type,omitempty"`
	PersonPronouns string `json:"person_pronouns,omitempty"`
	// The language this person usually posts in. Left out when not set.
	Language string `json:"language,omitempty"`
}

type PersonNode struct {
	PersonNodeItem

	UpdatedAt   time.Time                 `json:"updated_at,omitempty"`
	Editor      *responses.Editor         `json:"editor,omitempty"`
	Features    []*responses.NodeFeature  `json:"features,omitempty"`
	Authorized  *responses.NodeAuthorized `json:"authorized,omitempty"`
	Permissions map[string]string         `json:"permissions,omitempty"`
}

type PersonResponse struct {
	responses.ActorResponse
}

// MapNode maps a Person entity to a PersonNode response.
func (r *PersonResponse) MapNode(node *entities.Person) *PersonNode {
	if node == nil {
		return nil
	}

	return &PersonNode{
		PersonNodeItem: *r.MapNodeItem(node),
		UpdatedAt:      node.UpdatedAt,
		Editor: func() *responses.Editor {
			if node.Editor != nil {
				return responses.NewEditor(node.Editor)
			}
			return nil
		}(),
		Features: func() []*responses.NodeFeature {
			var features []*responses.NodeFeature
			for _, feature := range node.Features {
				features = append(features, responses.NewNodeFeature(feature))
			}
			return features
		}(),
		Authorized: func() *responses.NodeAuthorized {
			if node.Authorized != nil {
				return responses.NewNodeAuthorized(&node.AnahitaNode)
			}
			return nil
		}(),
		Permissions: node.Permissions,
	}
}

// MapNodeItem maps a Person entity to a PersonNodeItem response.
func (r *PersonResponse) MapNodeItem(node *entities.Person) *PersonNodeItem {
	if node == nil {
		return nil
	}

	return &PersonNodeItem{
		ActorNodeItem:  *r.ActorResponse.MapNodeItem(&node.AnahitaNode),
		PersonType:     node.PersonType,
		PersonPronouns: node.PersonPronouns,
		Language:       node.Language,
	}
}

func (r *PersonResponse) MapNodeItems(nodes []*entities.Person) []*PersonNodeItem {
	if nodes == nil {
		return nil
	}

	items := make([]*PersonNodeItem, 0, len(nodes))
	for _, node := range nodes {
		items = append(items, r.MapNodeItem(node))
	}

	return items
}
