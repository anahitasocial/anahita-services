package responses

import "anahita-libs/data/entities"

type NodeAddPermission struct {
	Entity string `json:"entity"`
	Access string `json:"access"`
}

type NodeFeature struct {
	Service        string               `json:"service"`
	NodeTypes      []string             `json:"node_types"`
	Composers      []string             `json:"composers"`
	Optional       bool                 `json:"optional"`
	Enabled        bool                 `json:"enabled"`
	AddPermissions []*NodeAddPermission `json:"add_permissions"`
	Ordering       int32                `json:"ordering"`
}

func NewNodeFeature(feature *entities.AnahitaNodeFeature) *NodeFeature {
	if feature == nil {
		return nil
	}

	// Initialize empty slices to avoid null in JSON
	nodeTypes := feature.NodeTypes
	if nodeTypes == nil {
		nodeTypes = []string{}
	}

	composers := feature.Composers
	if composers == nil {
		composers = []string{}
	}

	addPermissions := make([]*NodeAddPermission, 0, len(feature.AddPermissions))
	for _, perm := range feature.AddPermissions {
		addPermissions = append(addPermissions, &NodeAddPermission{
			Entity: perm.Entity,
			Access: perm.Access,
		})
	}

	return &NodeFeature{
		Service:        feature.Service,
		NodeTypes:      nodeTypes,
		Composers:      composers,
		Optional:       feature.Optional,
		Enabled:        feature.Enabled,
		AddPermissions: addPermissions,
		Ordering:       feature.Ordering,
	}
}

func NewNodeFeatures(features []*entities.AnahitaNodeFeature) []*NodeFeature {
	if features == nil {
		return []*NodeFeature{}
	}

	result := make([]*NodeFeature, 0, len(features))
	for _, f := range features {
		if nf := NewNodeFeature(f); nf != nil {
			result = append(result, nf)
		}
	}

	return result
}
