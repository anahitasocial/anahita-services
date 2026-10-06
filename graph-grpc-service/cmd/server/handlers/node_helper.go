package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func extractNodeIDs(nodes []*entities.AnahitaNode) []int64 {
	ids := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

func nodesToGrpc(nodes []*entities.AnahitaNode) []*graphpb.Node {
	grpcNodes := make([]*graphpb.Node, 0, len(nodes))
	for _, node := range nodes {
		grpcNodes = append(grpcNodes, nodeToGrpc(node))
	}
	return grpcNodes
}

func nodeToGrpc(node *entities.AnahitaNode) *graphpb.Node {
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
		AvatarFilename:       node.AvatarFilename,
		RootId:               node.RootID,
		ReplyAccess:          node.ReplyAccess,
		CoverFilename:        node.CoverFilename,
		Meta:                 string(node.Meta),
		CreatedAt:            timestamppb.New(node.CreatedAt),
		UpdatedAt:            timestamppb.New(node.UpdatedAt),
		FollowerCount:        node.FollowerCount,
		LeaderCount:          node.LeaderCount,
		SubscriberCount:      node.SubscriberCount,
		CommentStatus:        node.CommentStatus,
		CommentCount:         node.CommentCount,
		LikesCount:           node.LikesCount,
		IsLikedByViewer:      node.IsLikedByViewer,
		RepostCount:          node.RepostCount,
		QuoteCount:           node.QuoteCount,
		IsSubscribedByViewer: node.IsSubscribedByViewer,
		IsRepostedByViewer:   node.IsRepostedByViewer,
		IsFollowingViewer:    node.IsFollowingViewer,
		IsLeadingViewer:      node.IsLeadingViewer,
		IsAdminedByViewer:    node.IsAdminedByViewer,
		AllowFollowRequest:   node.AllowFollowRequest,
		HideRepostsOnProfile: node.HideRepostsOnProfile,
		StartsAt:             timestamppb.New(node.StartsAt),
		EndsAt:               timestamppb.New(node.EndsAt),
		OwnerType:            node.OwnerType,
		OwnerId:              node.OwnerID,
		Verified:             node.Verified,
		WebsiteUrl:           node.WebsiteURL,
		PersonType:           node.PersonType,
		PersonPronouns:       node.PersonPronouns,
		Pinned:               node.Pinned,
		ViewCount:            node.ViewCount,
		ParentId:             node.ParentID,
		ParentType:           node.ParentType,
		GeoLatitude:          node.GeoLatitude,
		GeoLongitude:         node.GeoLongitude,
		GeoAddress:           node.GeoAddress,
		GeoCity:              node.GeoCity,
		GeoStateProvince:     node.GeoStateProvince,
		GeoCountry:           node.GeoCountry,
		GeoPostalcode:        node.GeoPostalcode,
		CreatedBy:            node.CreatedBy,
		ModifiedBy:           node.ModifiedBy,
		Mimetype:             node.Mimetype,
		Language:             node.Language,
		EdgeCount:            node.EdgeCount,
		FeaturesRaw:          string(node.FeaturesRaw),
	}

	// Only set when the actor is actually scheduled for deletion.
	// timestamppb.New on a nil-derived zero time would send the epoch, which a
	// client reads as "deleted long ago" — the opposite of the truth.
	if node.DeletedAt != nil {
		grpcNode.DeletedAt = timestamppb.New(*node.DeletedAt)
	}

	// Likewise only when featured: the epoch would read as "featured long ago"
	// and put every actor in the list.
	if node.FeaturedAt != nil {
		grpcNode.FeaturedAt = timestamppb.New(*node.FeaturedAt)
	}

	// Only create Owner if it has data
	if node.Owner != nil && node.Owner.ID != 0 {
		grpcNode.Owner = nodeToGrpc(node.Owner)
	}

	// Only create Parent if it has data
	if node.Parent != nil && node.Parent.ID != 0 {
		grpcNode.Parent = nodeToGrpc(node.Parent)
	}

	// Only create Author if it has data
	if node.Author != nil && node.Author.ID != 0 {
		grpcNode.Author = nodeToGrpc(node.Author)
	}

	// Only create Editor if it has data
	if node.Editor != nil && node.Editor.ID != 0 {
		grpcNode.Editor = nodeToGrpc(node.Editor)
	}

	return grpcNode
}

func edgesToGrpc(edges []*entities.AnahitaEdge) []*graphpb.Edge {
	grpcEdges := make([]*graphpb.Edge, 0, len(edges))
	for _, edge := range edges {
		grpcEdges = append(grpcEdges, edgeToGrpc(edge))
	}
	return grpcEdges
}

func edgeToGrpc(edge *entities.AnahitaEdge) *graphpb.Edge {
	grpcEdge := &graphpb.Edge{
		Id:         edge.ID,
		Type:       edge.Type,
		NodeAId:    edge.NodeAID,
		NodeAType:  edge.NodeAType,
		NodeBId:    edge.NodeBID,
		NodeBType:  edge.NodeBType,
		Meta:       edge.Meta,
		CreatedAt:  timestamppb.New(edge.CreatedAt),
		CreatedBy:  edge.CreatedBy,
		UpdatedAt:  timestamppb.New(edge.UpdatedAt),
		ModifiedBy: edge.ModifiedBy,
		Ordering:   edge.Ordering,
		StartsAt:   timestamppb.New(edge.StartsAt),
		EndsAt:     timestamppb.New(edge.EndsAt),
	}
	return grpcEdge
}
