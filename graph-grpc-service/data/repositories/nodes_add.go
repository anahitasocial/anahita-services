package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"encoding/json"
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

	now := time.Now()
	created := addedAt(node.CreatedAt, now)

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
		"root_id",
		"reply_access",
		"quote_policy",
		"allow_download",
		"library_only",
		"draft",
		"quote_id",
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
		"mimetype",
		"language",
		"access",
		"features",
	).Values(
		node.Type,
		node.Service,
		node.Name,
		node.Alias,
		newlines(node.Body),
		// An event's start and end, and nothing on anything else.
		eventTime(node, node.StartsAt),
		eventTime(node, node.EndsAt),
		node.OwnerType,
		nullableID(node.OwnerID),
		node.CommentStatus,
		node.CommentCount,
		node.Enabled,
		node.Verified,
		node.WebsiteURL,
		node.PersonType,
		node.PersonPronouns,
		node.Pinned,
		text(node.Meta),
		node.ViewCount,
		nullableID(node.ParentID),
		node.ParentType,
		// NULL, not 0, on anything that is not a reply: "top level" is
		// asked for as root_id IS NULL.
		nullableID(node.RootID),
		node.ReplyAccess,
		node.QuotePolicy,
		node.AllowDownload,
		node.LibraryOnly,
		node.Draft,
		nullableID(node.QuoteID),
		// No coordinates is no place, not a point in the sea off Africa.
		nullablePlace(node.GeoLatitude, node.GeoLongitude),
		nullablePlace(node.GeoLongitude, node.GeoLatitude),
		node.GeoAddress,
		node.GeoCity,
		node.GeoStateProvince,
		node.GeoCountry,
		node.GeoPostalcode,
		created,
		nullableID(node.CreatedBy),
		now,
		nullableID(node.ModifiedBy),
		node.Filename,
		node.AvatarFilename,
		node.CoverFilename,
		node.AllowFollowRequest,
		node.Mimetype,
		node.Language,
		node.Access,
		text(node.FeaturesRaw),
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

// nullableID is the id, or NULL for none.
func nullableID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

// nullablePlace is a coordinate, or NULL when the node has no place: both
// coordinates 0. Given the other as well, so latitude and longitude are
// NULL together.
func nullablePlace(coordinate, other float64) any {
	if coordinate == 0 && other == 0 {
		return nil
	}
	return coordinate
}

// eventTime is an event's start or end, or NULL on anything that is not an
// event, or an event with none.
func eventTime(node *entities.AnahitaNode, at time.Time) any {
	if node.Type != constants.NodeActorEventEntityType || at.IsZero() {
		return nil
	}
	return at
}

// text is JSON kept as text: the empty string when there is none, as the
// column says.
func text(raw json.RawMessage) string {
	return string(raw)
}

// addedAt is when a new node says it was made: when it was written, for a
// post a service brings from elsewhere (a website's article keeps its item's
// date), unless it says nothing (a time never set arrives as the year 1, or
// 1970) or says the future. As addRemote has it.
func addedAt(said, now time.Time) time.Time {
	if said.Unix() <= 0 || said.After(now) {
		return now
	}
	return said
}
