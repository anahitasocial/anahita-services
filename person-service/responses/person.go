package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"time"
)

type PersonNodeItem struct {
	responses.ActorNodeItem

	UserType   string `json:"usertype,omitempty"`
	Email      string `json:"email,omitempty"`
	Username   string `json:"username,omitempty"`
	GivenName  string `json:"given_name,omitempty"`
	FamilyName string `json:"family_name,omitempty"`
	Gender     string `json:"gender,omitempty"`
}

type PersonNode struct {
	PersonNodeItem

	UpdateTime  time.Time                 `json:"update_time,omitempty"`
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
		UpdateTime:     node.UpdateTime,
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
		ActorNodeItem: *r.ActorResponse.MapNodeItem(&node.AnahitaNode),
		UserType:      node.UserType,
		Email:         node.Email,
		Username:      node.Username,
		GivenName:     node.GivenName,
		FamilyName:    node.FamilyName,
		Gender:        node.Gender,
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
