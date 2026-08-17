package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// FilterValidIDs removes zero or negative values and deduplicates the slice.
func FilterValidIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}) // optional: dedupe

	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		out = append(out, id)
	}

	return out
}

// FilterValidStrings removes empty strings and deduplicates the slice.
func FilterValidStrings(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}) // optional: dedupe

	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		out = append(out, id)
	}

	return out
}

// ConvertRequestNodesToEntities converts request nodes to AnahitaNode entities.
func ProtoNodeToAnahitaNode(n *graphpb.Node) *entities.AnahitaNode {
	node := entities.NewAnahitaNode()
	node.ID = n.Id
	node.Type = n.Type
	node.Service = n.Service
	node.Name = n.Name
	node.Alias = n.Alias
	node.Body = n.Body
	node.Excerpt = n.Excerpt
	node.StartDate = n.StartDate.AsTime()
	node.EndDate = n.EndDate.AsTime()
	node.OwnerID = n.OwnerId
	node.OwnerType = n.OwnerType
	node.ParentID = n.ParentId
	node.ParentType = n.ParentType
	node.Access = n.Access
	node.Filename = n.Filename
	node.Filesize = n.Filesize
	node.CoverFilename = n.CoverFilename
	node.CoverFilesize = n.CoverFilesize
	node.CoverMimetype = n.CoverMimetype
	node.Mimetype = n.Mimetype
	node.Timezone = n.Timezone
	node.Language = n.Language
	node.Status = n.Status
	node.StatusUpdateTime = n.StatusUpdateTime.AsTime()
	node.Ordering = n.Ordering
	node.Enabled = n.Enabled
	node.Verified = n.Verified
	node.Pinned = n.Pinned
	node.IsDefault = n.IsDefault
	node.HitsCount = n.HitsCount
	node.GeoLatitude = n.GeoLatitude
	node.GeoLongitude = n.GeoLongitude
	node.GeoAddress = n.GeoAddress
	node.GeoCity = n.GeoCity
	node.GeoStateProvince = n.GeoStateProvince
	node.GeoCountry = n.GeoCountry
	node.GeoPostalcode = n.GeoPostalcode
	node.Meta = []byte(n.Meta)
	node.FeaturesRaw = []byte(n.FeaturesRaw)
	node.CreationTime = n.CreationTime.AsTime()
	node.UpdateTime = n.UpdateTime.AsTime()
	node.CreatedBy = n.CreatedBy
	node.ModifiedBy = n.ModifiedBy
	node.AllowFollowRequest = n.AllowFollowRequest

	return node
}
