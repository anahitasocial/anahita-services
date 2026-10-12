package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"log"
	"strings"

	sq "github.com/Masterminds/squirrel"
)

// sanitizeSearchString strips the characters that mean something to LIKE.
//
// The search term itself is always bound, so this is not about SQL injection —
// it is about pattern injection. Inside "%term%", a "%" the searcher typed is
// a second wildcard and a "_" matches any single character, so searching for
// "%" returns the whole table and "_" matches everything of the right length.
// Against a text column that is also a cheap way to make the database do a
// great deal of work for one request.
//
// The backslash goes too, and that one is easy to miss: it is MySQL's default
// LIKE escape character, so a term ending in one escapes the closing "%" of
// the pattern the caller wrapped around it. "abc\" becomes "%abc\%", which
// looks for a literal percent sign and matches nothing.
func sanitizeSearchString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "%", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "\\", "")
	return s
}

func GetBaseSelectQuery() sq.SelectBuilder {
	fields := []string{
		"node.id",
		"node.type",
		"node.service",
		"COALESCE(node.name, '') AS name",
		"COALESCE(node.alias, '') AS alias",
		"COALESCE(node.body, '') AS body",
		"COALESCE(node.starts_at, NOW()) AS starts_at",
		"COALESCE(node.ends_at, NOW()) AS ends_at",
		"COALESCE(node.owner_type, '') AS owner_type",
		"COALESCE(node.owner_id, 0) AS owner_id",
		"COALESCE(node.comment_status, false) AS comment_status",
		"COALESCE(node.comment_count, 0) AS comment_count",
		"COALESCE(node.repost_count, 0) AS repost_count",
		"COALESCE(node.quote_count, 0) AS quote_count",
		"node.enabled",
		"node.verified",
		// COALESCE: nullable column, non-pointer string. A NULL would fail the
		// scan for every actor that has not set one, which is most of them.
		"COALESCE(node.website_url, '') AS website_url",
		"node.pinned",
		"COALESCE(node.meta, '') AS meta",
		"COALESCE(node.view_count, 0) AS view_count",
		"COALESCE(node.parent_id, 0) AS parent_id",
		"COALESCE(node.parent_type, '') AS parent_type",
		"COALESCE(node.root_id, 0) AS root_id",
		"COALESCE(node.reply_access, '') AS reply_access",
		"COALESCE(node.quote_policy, '') AS quote_policy",
		"COALESCE(node.quote_id, 0) AS quote_id",
		"(node.quote_detached_at IS NOT NULL) AS quote_detached",
		"COALESCE(node.geo_latitude, 0) AS geo_latitude",
		"COALESCE(node.geo_longitude, 0) AS geo_longitude",
		"COALESCE(node.geo_address, '') AS geo_address",
		"COALESCE(node.geo_city, '') AS geo_city",
		"COALESCE(node.geo_state_province, '') AS geo_state_province",
		"COALESCE(node.geo_country, '') AS geo_country",
		"COALESCE(node.geo_postalcode, '') AS geo_postalcode",
		"COALESCE(node.created_at, NOW()) AS created_at",
		"COALESCE(node.created_by, 0) AS created_by",
		"COALESCE(node.updated_at, NOW()) AS updated_at",
		"COALESCE(node.modified_by, 0) AS modified_by",
		"COALESCE(node.filename, '') AS filename",
		"COALESCE(node.avatar_filename, '') AS avatar_filename",
		"COALESCE(node.cover_filename, '') AS cover_filename",
		"COALESCE(node.follower_count, 0) AS follower_count",
		"COALESCE(node.leader_count, 0) AS leader_count",
		"COALESCE(node.allow_follow_request, false) AS allow_follow_request",
		"COALESCE(node.show_reposts_on_profile, false) AS show_reposts_on_profile",
		"COALESCE(node.allow_download, false) AS allow_download",
		"COALESCE(node.subscriber_count, 0) AS subscriber_count",
		"COALESCE(node.vote_up_count, 0) AS vote_up_count",
		"COALESCE(node.excerpt, '') AS excerpt",
		"COALESCE(node.mimetype, '') AS mimetype",
		"COALESCE(node.language, '') AS language",
		"COALESCE(node.access, '') AS access",
		"COALESCE(node.features, '') AS features",
		"COALESCE(node.person_type, '') AS person_type",
		"COALESCE(node.person_pronouns, '') AS person_pronouns",
		// Not COALESCEd: NULL means "not deleted" and a zero time would mean
		// every actor was scheduled for deletion at the start of the epoch.
		"node.deleted_at",
		// Likewise: NULL means never seen, not seen at the epoch.
		"node.last_visited_at",
		// Likewise: NULL means not featured.
		"node.featured_at",
	}

	// Owner fields
	fields = append(fields, []string{
		"COALESCE(owner.id, 0) AS owner_owner_id",
		"COALESCE(owner.type, '') AS owner_type",
		"COALESCE(owner.service, '') AS owner_service",
		"COALESCE(owner.access, '') AS owner_access",
		"COALESCE(owner.name, '') AS owner_name",
		"COALESCE(owner.alias, '') AS owner_alias",
		"COALESCE(owner.avatar_filename, '') AS owner_avatar_filename",
		"COALESCE(owner.meta, '') AS owner_meta",
		"COALESCE(owner.subscriber_count, 0) AS owner_subscriber_count",
	}...)

	// Parent fields
	fields = append(fields, []string{
		"COALESCE(parent.id, 0) AS parent_parent_id",
		"COALESCE(parent.type, '') AS parent_type",
		"COALESCE(parent.service, '') AS parent_service",
		"COALESCE(parent.name, '') AS parent_name",
		"COALESCE(parent.alias, '') AS parent_alias",
		"COALESCE(parent.body, '') AS parent_body",
		"COALESCE(parent.owner_id, 0) AS parent_owner_id",
		"COALESCE(parent.filename, '') AS parent_filename",
		"COALESCE(parent.cover_filename, '') AS parent_cover_filename",
		"COALESCE(parent.comment_status, 0) AS parent_comment_status",
		"COALESCE(parent.comment_count, 0) AS parent_comment_count",
		"COALESCE(parent.vote_up_count, 0) AS parent_vote_up_count",
		"COALESCE(parent.repost_count, 0) AS parent_repost_count",
		"COALESCE(parent.quote_count, 0) AS parent_quote_count",
		// When what it answers or reposts was written. Never NULL, so it
		// scans like the row's own date; a row with no parent does not use it.
		"COALESCE(parent.created_at, node.created_at, NOW()) AS parent_created_at",
		// The card a reposted note carries is in its meta.
		"COALESCE(parent.meta, '') AS parent_meta",
		// Who may see what is reposted, for whether its link is worth sending
		// beyond the site.
		"COALESCE(parent.access, '') AS parent_access",
	}...)

	// Parent Owner fields
	fields = append(fields, []string{
		"COALESCE(parent_owner.id, 0) AS parent_owner_id",
		"COALESCE(parent_owner.type, '') AS parent_owner_type",
		"COALESCE(parent_owner.service, '') AS parent_owner_service",
		"COALESCE(parent_owner.name, '') AS parent_owner_name",
		"COALESCE(parent_owner.alias, '') AS parent_owner_alias",
		"COALESCE(parent_owner.avatar_filename, '') AS parent_owner_avatar_filename",
		"COALESCE(parent_owner.subscriber_count, 0) AS parent_owner_subscriber_count",
	}...)

	// Parent Author fields
	fields = append(fields, []string{
		"COALESCE(parent_author.id, 0) AS parent_author_id",
		"COALESCE(parent_author.type, '') AS parent_author_type",
		"COALESCE(parent_author.service, '') AS parent_author_service",
		"COALESCE(parent_author.name, '') AS parent_author_name",
		"COALESCE(parent_author.alias, '') AS parent_author_alias",
		"COALESCE(parent_author.avatar_filename, '') AS parent_author_avatar_filename",
		"COALESCE(parent_author.meta, '') AS parent_author_meta",
	}...)

	// Author fields
	fields = append(fields, []string{
		"COALESCE(author.id, 0) AS author_id",
		"COALESCE(author.type, '') AS author_type",
		"COALESCE(author.service, '') AS author_service",
		"COALESCE(author.name, '') AS author_name",
		"COALESCE(author.alias, '') AS author_alias",
		"COALESCE(author.avatar_filename, '') AS author_avatar_filename",
		"COALESCE(author.meta, '') AS author_meta",
	}...)

	// Editor fields
	fields = append(fields, []string{
		"COALESCE(editor.id, 0) AS editor_id",
		"COALESCE(editor.type, '') AS editor_type",
		"COALESCE(editor.service, '') AS editor_service",
		"COALESCE(editor.name, '') AS editor_name",
		"COALESCE(editor.alias, '') AS editor_alias",
		"COALESCE(editor.avatar_filename, '') AS editor_avatar_filename",
		"COALESCE(editor.meta, '') AS editor_meta",
	}...)

	query := sq.Select(fields...).From(
		"nodes AS node",
	).LeftJoin(
		"nodes AS owner ON node.owner_id = owner.id",
	).LeftJoin(
		"nodes AS parent ON node.parent_id = parent.id",
	).LeftJoin(
		"nodes AS parent_owner ON parent.owner_id = parent_owner.id",
	).LeftJoin(
		"nodes AS parent_author ON parent.created_by = parent_author.id",
	).LeftJoin(
		"nodes AS author ON node.created_by = author.id",
	).LeftJoin(
		"nodes AS editor ON node.modified_by = editor.id",
	)

	return query
}

// FetchNode retrieves a single node based on the provided query builder.
func FetchNode(db *sql.DB, query sq.SelectBuilder) (*entities.AnahitaNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DBTimeout)
	defer cancel()

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	// log.Println("FetchNode Query:", sql, args)

	item := entities.NewAnahitaNode()
	err = db.QueryRowContext(ctx, sql, args...).Scan(
		&item.ID,
		&item.Type,
		&item.Service,
		&item.Name,
		&item.Alias,
		&item.Body,
		&item.StartsAt,
		&item.EndsAt,
		&item.OwnerType,
		&item.OwnerID,
		&item.CommentStatus,
		&item.CommentCount,
		&item.RepostCount,
		&item.QuoteCount,
		&item.Enabled,
		&item.Verified,
		&item.WebsiteURL,
		&item.Pinned,
		&item.Meta,
		&item.ViewCount,
		&item.ParentID,
		&item.ParentType,
		&item.RootID,
		&item.ReplyAccess,
		&item.QuotePolicy,
		&item.QuoteID,
		&item.QuoteDetached,
		&item.GeoLatitude,
		&item.GeoLongitude,
		&item.GeoAddress,
		&item.GeoCity,
		&item.GeoStateProvince,
		&item.GeoCountry,
		&item.GeoPostalcode,
		&item.CreatedAt,
		&item.CreatedBy,
		&item.UpdatedAt,
		&item.ModifiedBy,
		&item.Filename,
		&item.AvatarFilename,
		&item.CoverFilename,
		&item.FollowerCount,
		&item.LeaderCount,
		&item.AllowFollowRequest,
		&item.ShowRepostsOnProfile,
		&item.AllowDownload,
		&item.SubscriberCount,
		&item.LikesCount,
		&item.Excerpt,
		&item.Mimetype,
		&item.Language,
		&item.Access,
		&item.FeaturesRaw,
		&item.PersonType,
		&item.PersonPronouns,
		&item.DeletedAt,
		&item.LastVisitedAt,
		&item.FeaturedAt,
		// Owner fields
		&item.Owner.ID,
		&item.Owner.Type,
		&item.Owner.Service,
		&item.Owner.Access,
		&item.Owner.Name,
		&item.Owner.Alias,
		&item.Owner.AvatarFilename,
		&item.Owner.Meta,
		&item.Owner.SubscriberCount,
		// Parent fields
		&item.Parent.ID,
		&item.Parent.Type,
		&item.Parent.Service,
		&item.Parent.Name,
		&item.Parent.Alias,
		&item.Parent.Body,
		&item.Parent.OwnerID,
		&item.Parent.Filename,
		&item.Parent.CoverFilename,
		&item.Parent.CommentStatus,
		&item.Parent.CommentCount,
		&item.Parent.LikesCount,
		&item.Parent.RepostCount,
		&item.Parent.QuoteCount,
		&item.Parent.CreatedAt,
		&item.Parent.Meta,
		&item.Parent.Access,
		// Parent Owner fields
		&item.Parent.Owner.ID,
		&item.Parent.Owner.Type,
		&item.Parent.Owner.Service,
		&item.Parent.Owner.Name,
		&item.Parent.Owner.Alias,
		&item.Parent.Owner.AvatarFilename,
		&item.Parent.Owner.SubscriberCount,
		// Parent Author fields
		&item.Parent.Author.ID,
		&item.Parent.Author.Type,
		&item.Parent.Author.Service,
		&item.Parent.Author.Name,
		&item.Parent.Author.Alias,
		&item.Parent.Author.AvatarFilename,
		&item.Parent.Author.Meta,
		// Author fields
		&item.Author.ID,
		&item.Author.Type,
		&item.Author.Service,
		&item.Author.Name,
		&item.Author.Alias,
		&item.Author.AvatarFilename,
		&item.Author.Meta,
		// Editor fields
		&item.Editor.ID,
		&item.Editor.Type,
		&item.Editor.Service,
		&item.Editor.Name,
		&item.Editor.Alias,
		&item.Editor.AvatarFilename,
		&item.Editor.Meta,
	)
	if err != nil {
		return nil, err
	}

	return item, nil
}

// FetchNodes retrieves multiple nodes based on the provided query builder.
func FetchNodes(db *sql.DB, query sq.SelectBuilder) ([]*entities.AnahitaNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DBTimeout)
	defer cancel()

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	log.Println("FetchNode Query:", sql, args)

	rows, err := db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*entities.AnahitaNode
	for rows.Next() {
		item := entities.NewAnahitaNode()
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Service,
			&item.Name,
			&item.Alias,
			&item.Body,
			&item.StartsAt,
			&item.EndsAt,
			&item.OwnerType,
			&item.OwnerID,
			&item.CommentStatus,
			&item.CommentCount,
			&item.RepostCount,
			&item.QuoteCount,
			&item.Enabled,
			&item.Verified,
			&item.WebsiteURL,
			&item.Pinned,
			&item.Meta,
			&item.ViewCount,
			&item.ParentID,
			&item.ParentType,
			&item.RootID,
			&item.ReplyAccess,
			&item.QuotePolicy,
			&item.QuoteID,
			&item.QuoteDetached,
			&item.GeoLatitude,
			&item.GeoLongitude,
			&item.GeoAddress,
			&item.GeoCity,
			&item.GeoStateProvince,
			&item.GeoCountry,
			&item.GeoPostalcode,
			&item.CreatedAt,
			&item.CreatedBy,
			&item.UpdatedAt,
			&item.ModifiedBy,
			&item.Filename,
			&item.AvatarFilename,
			&item.CoverFilename,
			&item.FollowerCount,
			&item.LeaderCount,
			&item.AllowFollowRequest,
			&item.ShowRepostsOnProfile,
			&item.AllowDownload,
			&item.SubscriberCount,
			&item.LikesCount,
			&item.Excerpt,
			&item.Mimetype,
			&item.Language,
			&item.Access,
			&item.FeaturesRaw,
			&item.PersonType,
			&item.PersonPronouns,
			&item.DeletedAt,
			&item.LastVisitedAt,
			&item.FeaturedAt,
			// Owner fields
			&item.Owner.ID,
			&item.Owner.Type,
			&item.Owner.Service,
			&item.Owner.Access,
			&item.Owner.Name,
			&item.Owner.Alias,
			&item.Owner.AvatarFilename,
			&item.Owner.Meta,
			&item.Owner.SubscriberCount,
			// Parent fields
			&item.Parent.ID,
			&item.Parent.Type,
			&item.Parent.Service,
			&item.Parent.Name,
			&item.Parent.Alias,
			&item.Parent.Body,
			&item.Parent.OwnerID,
			&item.Parent.Filename,
			&item.Parent.CoverFilename,
			&item.Parent.CommentStatus,
			&item.Parent.CommentCount,
			&item.Parent.LikesCount,
			&item.Parent.RepostCount,
			&item.Parent.QuoteCount,
			&item.Parent.CreatedAt,
			&item.Parent.Meta,
			&item.Parent.Access,
			// Parent Owner fields
			&item.Parent.Owner.ID,
			&item.Parent.Owner.Type,
			&item.Parent.Owner.Service,
			&item.Parent.Owner.Name,
			&item.Parent.Owner.Alias,
			&item.Parent.Owner.AvatarFilename,
			&item.Parent.Owner.SubscriberCount,
			// Parent Author fields
			&item.Parent.Author.ID,
			&item.Parent.Author.Type,
			&item.Parent.Author.Service,
			&item.Parent.Author.Name,
			&item.Parent.Author.Alias,
			&item.Parent.Author.AvatarFilename,
			&item.Parent.Author.Meta,
			// Author fields
			&item.Author.ID,
			&item.Author.Type,
			&item.Author.Service,
			&item.Author.Name,
			&item.Author.Alias,
			&item.Author.AvatarFilename,
			&item.Author.Meta,
			// Editor fields
			&item.Editor.ID,
			&item.Editor.Type,
			&item.Editor.Service,
			&item.Editor.Name,
			&item.Editor.Alias,
			&item.Editor.AvatarFilename,
			&item.Editor.Meta,
		)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	return items, nil
}
