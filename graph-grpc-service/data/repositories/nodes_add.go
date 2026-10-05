package repositories

import (
	"anahita-libs/data/entities"
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
)

// Add creates a new node based on the provided add request and viewer context.
func (r *nodesRepo) Add(node *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if err := r.validateAdd(node); err != nil {
		return nil, err
	}

	// Ensure StartsAt and EndsAt have default values if zero value
	if node.StartsAt.IsZero() {
		node.StartsAt = time.Now()
	}
	if node.EndsAt.IsZero() {
		// Default end date: same as start date (adjust if different logic needed)
		node.EndsAt = node.StartsAt
	}

	query := sq.Insert("nodes").Columns(
		"type",
		"service",
		"name",
		"alias",
		"body",
		"starts_at",
		"ends_at",
		"owner_type",
		"owner_id",
		"comment_status",
		"comment_count",
		"enabled",
		"verified",
		"website_url",
		"person_type",
		"person_pronouns",
		"pinned",
		"meta",
		"view_count",
		"parent_id",
		"parent_type",
		"geo_latitude",
		"geo_longitude",
		"geo_address",
		"geo_city",
		"geo_state_province",
		"geo_country",
		"geo_postalcode",
		"created_at",
		"created_by",
		"updated_at",
		"modified_by",
		"filename",
		"avatar_filename",
		"cover_filename",
		"allow_follow_request",
		"excerpt",
		"mimetype",
		"language",
		"access",
		"features",
	).Values(
		node.Type,
		node.Service,
		node.Name,
		node.Alias,
		node.Body,
		node.StartsAt,
		node.EndsAt,
		node.OwnerType,
		node.OwnerID,
		node.CommentStatus,
		node.CommentCount,
		node.Enabled,
		node.Verified,
		node.WebsiteURL,
		node.PersonType,
		node.PersonPronouns,
		node.Pinned,
		node.Meta,
		node.ViewCount,
		node.ParentID,
		node.ParentType,
		node.GeoLatitude,
		node.GeoLongitude,
		node.GeoAddress,
		node.GeoCity,
		node.GeoStateProvince,
		node.GeoCountry,
		node.GeoPostalcode,
		time.Now(),
		node.CreatedBy,
		time.Now(),
		node.ModifiedBy,
		node.Filename,
		node.AvatarFilename,
		node.CoverFilename,
		node.AllowFollowRequest,
		node.Excerpt,
		node.Mimetype,
		node.Language,
		node.Access,
		node.FeaturesRaw,
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	result, err := r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	node.ID = id

	return r.Read(node.ID)
}

// BatchAdd creates multiple new nodes based on the provided entities.
func (r *nodesRepo) BatchAdd(nodes []*entities.AnahitaNode) ([]*entities.AnahitaNode, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	var addedNodes []*entities.AnahitaNode
	for _, node := range nodes {
		addedNode, err := r.Add(node)
		if err != nil {
			return nil, err
		}
		addedNodes = append(addedNodes, addedNode)
	}

	return addedNodes, nil
}
