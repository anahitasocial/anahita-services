package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/responses"
)

type LocationNodeItem struct {
	ID               int64                     `json:"id"`
	Type             string                    `json:"type"`
	Name             string                    `json:"name,omitempty"`
	Alias            string                    `json:"alias,omitempty"`
	GeoLatitude      float64                   `json:"latitude,omitempty"`
	GeoLongitude     float64                   `json:"longitude,omitempty"`
	GeoAddress       string                    `json:"address,omitempty"`
	GeoCity          string                    `json:"city,omitempty"`
	GeoStateProvince string                    `json:"state_province,omitempty"`
	GeoCountry       string                    `json:"country,omitempty"`
	GeoPostalcode    string                    `json:"postalcode,omitempty"`
	Authorized       *responses.NodeAuthorized `json:"authorized,omitempty"`
}

type LocationNode struct {
	LocationNodeItem
	EdgeCount int64 `json:"edge_count,omitempty"`
}

type LocationNodeResponse struct{}

func (r *LocationNodeResponse) MapLocationNodeItem(node *entities.AnahitaNode) *LocationNodeItem {
	if node == nil {
		return nil
	}

	return &LocationNodeItem{
		ID:               node.ID,
		Type:             node.Type,
		Name:             node.Name,
		Alias:            node.Alias,
		GeoLatitude:      node.GeoLatitude,
		GeoLongitude:     node.GeoLongitude,
		GeoAddress:       node.GeoAddress,
		GeoCity:          node.GeoCity,
		GeoStateProvince: node.GeoStateProvince,
		GeoCountry:       node.GeoCountry,
		GeoPostalcode:    node.GeoPostalcode,
		Authorized: func() *responses.NodeAuthorized {
			if node.Authorized != nil {
				return responses.NewNodeAuthorized(node)
			}
			return nil
		}(),
	}
}

func (r *LocationNodeResponse) MapLocationNode(node *entities.AnahitaNode) *LocationNode {
	if node == nil {
		return nil
	}

	locationNodeItem := r.MapLocationNodeItem(node)

	return &LocationNode{
		LocationNodeItem: *locationNodeItem,
		EdgeCount:        node.EdgeCount,
	}
}

func (r *LocationNodeResponse) MapLocationNodeItems(nodes []*entities.AnahitaNode) []*LocationNodeItem {
	if nodes == nil {
		return nil
	}

	mapped := make([]*LocationNodeItem, 0, len(nodes))
	for _, node := range nodes {
		mapped = append(mapped, r.MapLocationNodeItem(node))
	}
	return mapped
}
