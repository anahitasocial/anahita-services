package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func grpcToGraphNodes(nodes []*graphpb.Node) []*entities.AnahitaNode {
	if nodes == nil {
		return nil
	}

	result := make([]*entities.AnahitaNode, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, grpcToGraphNode(n))
	}
	return result
}

func grpcToGraphNode(m *graphpb.Node) *entities.AnahitaNode {
	if m == nil {
		return nil
	}

	n := entities.NewAnahitaNode()

	// Basic fields
	n.ID = m.Id
	n.Type = m.Type
	n.Service = m.Service
	n.Enabled = m.Enabled
	n.Access = m.Access
	n.Name = m.Name
	n.Alias = m.Alias
	n.Body = m.Body
	n.Excerpt = m.Excerpt
	n.CoverFilename = m.CoverFilename
	n.Filename = m.Filename
	n.Filesize = m.Filesize
	n.Meta = []byte(m.Meta)
	n.EdgeCount = m.EdgeCount

	// Only when the actor is actually scheduled for deletion. The proto field
	// is absent otherwise, and an absent timestamp must stay nil rather than
	// becoming the epoch, which reads as "deleted long ago".
	if m.DeletedAt != nil {
		deletedAt := m.DeletedAt.AsTime()
		n.DeletedAt = &deletedAt
	}

	// Time fields
	if m.CreatedOn != nil {
		n.CreationTime = m.CreatedOn.AsTime()
	}
	if m.ModifiedOn != nil {
		n.UpdateTime = m.ModifiedOn.AsTime()
	}
	if m.StartDate != nil {
		n.StartDate = m.StartDate.AsTime()
	}
	if m.EndDate != nil {
		n.EndDate = m.EndDate.AsTime()
	}
	if m.StatusUpdateTime != nil {
		n.StatusUpdateTime = m.StatusUpdateTime.AsTime()
	}

	if m.LastCommentOn != nil {
		n.LastCommentOn = m.LastCommentOn.AsTime()
	}

	// Counts and social fields
	n.FollowerCount = m.FollowerCount
	n.LeaderCount = m.LeaderCount
	n.MutualCount = m.MutualCount
	n.SubscriberCount = m.SubscriberCount
	n.RepostCount = m.RepostCount
	n.QuoteCount = m.QuoteCount
	n.CommentStatus = m.CommentStatus
	n.CommentCount = m.CommentCount
	n.LikesCount = m.LikesCount
	n.DislikesCount = m.DislikesCount

	// Viewer interaction flags
	n.IsLikedByViewer = m.IsLikedByViewer
	n.IsSubscribedByViewer = m.IsSubscribedByViewer
	n.IsRepostedByViewer = m.IsRepostedByViewer
	n.IsFollowingViewer = m.IsFollowingViewer
	n.IsLeadingViewer = m.IsLeadingViewer
	n.IsAdminedByViewer = m.IsAdminedByViewer
	n.IsBlockedByViewer = m.IsBlockedByViewer

	// Additional fields
	n.OwnerType = m.OwnerType
	n.OwnerID = m.OwnerId
	n.LastCommentID = m.LastCommentId
	n.LastCommentBy = m.LastCommentBy
	n.Ordering = m.Ordering
	n.Verified = m.Verified
	n.WebsiteURL = m.WebsiteUrl
	n.Pinned = m.Pinned
	n.IsDefault = m.IsDefault
	n.HitsCount = m.HitsCount
	n.ParentID = m.ParentId
	n.ParentType = m.ParentType
	n.AllowFollowRequest = m.AllowFollowRequest
	n.CreatedBy = m.CreatedBy
	n.ModifiedBy = m.ModifiedBy
	n.CoverFilesize = m.CoverFilesize
	n.CoverMimetype = m.CoverMimetype
	n.Status = m.Status
	n.Mimetype = m.Mimetype
	n.Timezone = m.Timezone
	n.Language = m.Language

	// Geo fields
	n.GeoLatitude = m.GeoLatitude
	n.GeoLongitude = m.GeoLongitude
	n.GeoAddress = m.GeoAddress
	n.GeoCity = m.GeoCity
	n.GeoStateProvince = m.GeoStateProvince
	n.GeoCountry = m.GeoCountry
	n.GeoPostalcode = m.GeoPostalcode

	// Raw features
	n.FeaturesRaw = []byte(m.FeaturesRaw)

	nodeFeatures := make([]*entities.AnahitaNodeFeature, 0, len(m.Features))
	for _, af := range m.Features {
		nodeTypes := af.NodeTypes
		if nodeTypes == nil {
			nodeTypes = []string{}
		}

		composers := af.Composers
		if composers == nil {
			composers = []string{}
		}

		addPermissions := make([]entities.AnahitaNodeAddPermission, 0, len(af.AddPermissions))
		for _, ap := range af.AddPermissions {
			addPermissions = append(addPermissions, entities.AnahitaNodeAddPermission{
				Entity: ap.Entity,
				Access: ap.Access,
			})
		}

		nodeFeatures = append(nodeFeatures, &entities.AnahitaNodeFeature{
			Service:        af.Service,
			NodeTypes:      nodeTypes,
			Composers:      composers,
			Optional:       af.Optional,
			Enabled:        af.Enabled,
			AddPermissions: addPermissions,
			Ordering:       af.Ordering,
		})
	}
	n.Features = nodeFeatures

	// --- Owner ---
	if m.Owner != nil {
		n.Owner = grpcToGraphNode(m.Owner)
	}

	// --- Parent ---
	if m.Parent != nil {
		n.Parent = grpcToGraphNode(m.Parent)
	}

	// --- Parent Owner ---
	if m.Parent != nil && m.Parent.Owner != nil {
		n.Parent.Owner = grpcToGraphNode(m.Parent.Owner)
	}

	// --- Author ---
	if m.Author != nil {
		n.Author = grpcToGraphNode(m.Author)
	}

	// --- Editor ---
	if m.Editor != nil {
		n.Editor = grpcToGraphNode(m.Editor)
	}

	// --- Derived assets ---
	if n.Filename != "" {
		if strings.Contains(n.Type, "actor") {
			n.GetAvatars()
		} else {
			n.GetPortraits()
		}
	}
	if n.CoverFilename != "" {
		n.GetCovers()
	}

	n.GetHashtags()
	n.GetMentions()

	if n.Owner != nil && n.Owner.ID != 0 {
		n.Owner.GetAvatars()
	}
	if n.Parent != nil && n.Parent.Owner != nil && n.Parent.Owner.ID != 0 {
		n.Parent.Owner.GetAvatars()
	}
	if n.Parent != nil && n.Parent.Author != nil && n.Parent.Author.ID != 0 {
		n.Parent.Author.GetAvatars()
	}
	if n.Parent != nil && n.Parent.Filename != "" {
		n.Parent.GetPortraits()
	}
	if n.Parent != nil && n.Parent.CoverFilename != "" {
		n.Parent.GetCovers()
	}
	if n.Author != nil && n.Author.ID != 0 {
		n.Author.GetAvatars()
	}
	if n.Editor != nil && n.Editor.ID != 0 {
		n.Editor.GetAvatars()
	}

	return n
}

func graphNodeToGrpc(node *entities.AnahitaNode) *graphpb.Node {
	if node == nil {
		return nil
	}

	grpcNode := &graphpb.Node{
		Id:                   node.ID,
		Type:                 node.Type,
		Service:              node.Service,
		Enabled:              node.Enabled,
		Access:               node.Access,
		Name:                 node.Name,
		Alias:                node.Alias,
		Body:                 node.Body,
		Excerpt:              node.Excerpt,
		Filename:             node.Filename,
		CoverFilename:        node.CoverFilename,
		Filesize:             node.Filesize,
		Meta:                 string(node.Meta),
		CreatedOn:            timestamppb.New(node.CreationTime),
		ModifiedOn:           timestamppb.New(node.UpdateTime),
		FollowerCount:        node.FollowerCount,
		LeaderCount:          node.LeaderCount,
		MutualCount:          node.MutualCount,
		SubscriberCount:      node.SubscriberCount,
		CommentStatus:        node.CommentStatus,
		CommentCount:         node.CommentCount,
		LikesCount:           node.LikesCount,
		DislikesCount:        node.DislikesCount,
		IsLikedByViewer:      node.IsLikedByViewer,
		RepostCount:          node.RepostCount,
		QuoteCount:           node.QuoteCount,
		IsSubscribedByViewer: node.IsSubscribedByViewer,
		IsRepostedByViewer:   node.IsRepostedByViewer,
		IsFollowingViewer:    node.IsFollowingViewer,
		IsLeadingViewer:      node.IsLeadingViewer,
		IsAdminedByViewer:    node.IsAdminedByViewer,
		IsBlockedByViewer:    node.IsBlockedByViewer,
		AllowFollowRequest:   node.AllowFollowRequest,
		StartDate:            timestamppb.New(node.StartDate),
		EndDate:              timestamppb.New(node.EndDate),
		OwnerType:            node.OwnerType,
		OwnerId:              node.OwnerID,
		LastCommentId:        node.LastCommentID,
		LastCommentBy:        node.LastCommentBy,
		LastCommentOn:        timestamppb.New(node.LastCommentOn),
		Ordering:             node.Ordering,
		Verified:             node.Verified,
		WebsiteUrl:           node.WebsiteURL,
		Pinned:               node.Pinned,
		IsDefault:            node.IsDefault,
		HitsCount:            node.HitsCount,
		ParentId:             node.ParentID,
		ParentType:           node.ParentType,
		GeoLatitude:          node.GeoLatitude,
		GeoLongitude:         node.GeoLongitude,
		GeoAddress:           node.GeoAddress,
		GeoCity:              node.GeoCity,
		GeoStateProvince:     node.GeoStateProvince,
		GeoCountry:           node.GeoCountry,
		GeoPostalcode:        node.GeoPostalcode,
		CreationTime:         timestamppb.New(node.CreationTime),
		CreatedBy:            node.CreatedBy,
		UpdateTime:           timestamppb.New(node.UpdateTime),
		ModifiedBy:           node.ModifiedBy,
		CoverFilesize:        node.CoverFilesize,
		CoverMimetype:        node.CoverMimetype,
		Status:               node.Status,
		StatusUpdateTime:     timestamppb.New(node.StatusUpdateTime),
		Mimetype:             node.Mimetype,
		Timezone:             node.Timezone,
		Language:             node.Language,
		EdgeCount:            node.EdgeCount,
		FeaturesRaw:          string(node.FeaturesRaw),
	}

	// Only create Owner if it has data
	if node.Owner != nil && node.Owner.ID != 0 {
		grpcNode.Owner = graphNodeToGrpc(node.Owner)
	}

	// Only create Parent if it has data
	if node.Parent != nil && node.Parent.ID != 0 {
		grpcNode.Parent = graphNodeToGrpc(node.Parent)
	}

	// Only create Parent Owner if it has data
	if node.Parent != nil && node.Parent.Owner != nil && node.Parent.Owner.ID != 0 {
		grpcNode.Parent.Owner = graphNodeToGrpc(node.Parent.Owner)
	}

	// Only create Author if it has data
	if node.Author != nil && node.Author.ID != 0 {
		grpcNode.Author = graphNodeToGrpc(node.Author)
	}

	// Only create Editor if it has data
	if node.Editor != nil && node.Editor.ID != 0 {
		grpcNode.Editor = graphNodeToGrpc(node.Editor)
	}

	return grpcNode
}
