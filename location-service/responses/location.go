package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/responses"
)

// LocationNodeItem is a place posts can be tied to.
type LocationNodeItem struct {
	// Its id.
	ID int64 `json:"id"`
	// Always a place's type.
	Type string `json:"type"`
	// What it is called.
	Name string `json:"name,omitempty"`
	// Its short name.
	Alias string `json:"alias,omitempty"`
	// Where it is: latitude, found from its address.
	GeoLatitude float64 `json:"latitude,omitempty"`
	// Where it is: longitude.
	GeoLongitude float64 `json:"longitude,omitempty"`
	// Its street address.
	GeoAddress string `json:"address,omitempty"`
	// Its city.
	GeoCity string `json:"city,omitempty"`
	// Its state or province.
	GeoStateProvince string `json:"state_province,omitempty"`
	// Its country.
	GeoCountry string `json:"country,omitempty"`
	// Its postal code.
	GeoPostalcode string `json:"postalcode,omitempty"`
	// What the viewer may do with it.
	Authorized *responses.NodeAuthorized `json:"authorized,omitempty"`
	// In a trending list: how many people tied a post to it in the window.
	People uint64 `json:"people,omitempty"`
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
