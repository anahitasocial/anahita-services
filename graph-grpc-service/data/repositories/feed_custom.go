package repositories

import (
	"database/sql"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/feeds"

	sq "github.com/Masterminds/squirrel"
)

// A custom feed: the posts a definition asks for, newest first.
//
// The query is built from a definition's fields and from nothing else. No
// text of the definition becomes text of the query: every value is a
// parameter, and the kinds of post and of profile are looked up in the
// lists anahita-libs/feeds keeps.
//
// THE RULE THIS KEEPS: a feed never widens what anybody may see. Whatever
// the definition selects is then put through the gate every feed uses, for
// whoever is READING, not for whoever made the feed. A public feed built on
// a private group shows that group's posts to its followers and to nobody
// else.

type FeedCustomRepoInterface interface {
	// Browse returns a page of the feed owned by ownerID, older than the
	// cursor, as viewer may see it.
	Browse(def *feeds.Definition, ownerID int64, before feeds.Cursor, limit uint64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)
}

func NewFeedCustomRepo(db *sql.DB) FeedCustomRepoInterface {
	return &FeedCustomRepo{db: db}
}

type FeedCustomRepo struct {
	db *sql.DB
}

// The owner of the post is somebody the feed's owner follows.
const feedCustomFollowed = `EXISTS (
	SELECT 1
	FROM edges AS followed
	WHERE followed.type = ?
	  AND followed.node_a_id = ?
	  AND followed.node_b_id = node.owner_id
)`

// The post carries one of these hashtags. A tag runs from the hashtag to
// what it is on, and a hashtag is found by its alias. The list of aliases is
// written in as placeholders.
const feedCustomTagged = `EXISTS (
	SELECT 1
	FROM edges AS tagged
	JOIN nodes AS hashtag ON hashtag.id = tagged.node_a_id
	WHERE tagged.type = ?
	  AND tagged.node_b_id = node.id
	  AND hashtag.type = ?
	  AND hashtag.alias IN (`

// The post was tagged with one of these places.
const feedCustomPlaced = `EXISTS (
	SELECT 1
	FROM edges AS placed
	WHERE placed.type = ?
	  AND placed.node_b_id = node.id
	  AND placed.node_a_id IN (`

// Within a distance of a point: tied to a place there, or carrying a
// location of its own there. The point twice, then the distance in metres,
// twice. ST_Distance_Sphere takes longitude first.
const feedCustomNear = `(
	EXISTS (
		SELECT 1
		FROM edges AS placed
		JOIN nodes AS place ON place.id = placed.node_a_id
		WHERE placed.type = ?
		  AND placed.node_b_id = node.id
		  AND place.geo_latitude IS NOT NULL
		  AND ST_Distance_Sphere(POINT(place.geo_longitude, place.geo_latitude), POINT(?, ?)) <= ?
	)
	OR (
		node.geo_latitude IS NOT NULL
		AND ST_Distance_Sphere(POINT(node.geo_longitude, node.geo_latitude), POINT(?, ?)) <= ?
	)
)`

// Older than the cursor: written before it, or in the same second with a
// smaller id.
const feedCustomOlder = `(
	node.created_at < ?
	OR (node.created_at = ? AND node.id < ?)
)`

func (repo *FeedCustomRepo) Browse(def *feeds.Definition, ownerID int64, before feeds.Cursor, limit uint64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	if viewer == nil {
		viewer = entities.NewViewer()
	}
	if limit == 0 || limit > feeds.MaxPage {
		limit = feeds.PageSize
	}

	query := GetBaseSelectQuery().
		Where(sq.Eq{"node.type": def.MediaEntityTypes()})

	query = feedCustomMatching(query, def, ownerID)
	query = feedCustomFilters(query, def)
	query = feedCustomExclusions(query, def)

	// Replies are read in their thread, unless the feed asks for them.
	if !def.IncludeReplies {
		query = query.Where("node.root_id IS NULL")
	}

	// What may be seen, by whoever is reading. A feed is sent to people, so
	// what was archived is left out of it as well.
	query = SetFeedAccessQuery(query, viewer)
	query = SetMutedQuery(query, viewer)
	query = SetLifecycleQuery(query, viewer)
	query = SetArchiveQuery(query, viewer)

	if !before.IsZero() {
		query = query.Where(feedCustomOlder, before.CreatedAt, before.CreatedAt, before.ID)
	}

	query = query.
		OrderBy("node.created_at DESC", "node.id DESC").
		Limit(limit)

	return FetchNodes(repo.db, query)
}

// feedCustomMatching keeps the posts that match what the feed asks for:
// where they come from (whoever its owner follows, the profiles chosen by
// hand), their hashtags, their locations, their words and their area. With
// Match all, each kind asked for narrows the rest; with Match any (roadmap
// row 149), a post matching one of them is in. With no source, the whole
// site, which the definition allows only through something else to match.
func feedCustomMatching(query sq.SelectBuilder, def *feeds.Definition, ownerID int64) sq.SelectBuilder {
	matches, unreadable := feedCustomMatches(def, ownerID)

	if def.MatchesAny() {
		if len(matches) == 0 && unreadable {
			// Nothing it asks for can be read: nothing, and never the site.
			return query.Where("1 = 0")
		}
		if len(matches) == 0 {
			// It asks for nothing to match: what was written here, which
			// the filters say (roadmap row 175), as with all.
			return query
		}
		return query.Where(sq.Or(matches))
	}

	if unreadable {
		return query.Where("1 = 0")
	}
	for _, match := range matches {
		query = query.Where(match)
	}
	return query
}

// feedCustomMatches is one condition for each kind of thing the feed asks
// a post to match. unreadable says something it asks for cannot be read:
// whoever its owner follows, with no owner; an area around wherever its
// reader is, with nobody saying where. Left out of the list, it matches
// nothing: with all, the feed is empty; with any, the rest still match.
func feedCustomMatches(def *feeds.Definition, ownerID int64) (matches []sq.Sqlizer, unreadable bool) {
	sources := sq.Or{}
	if def.Sources.Following && ownerID > 0 {
		sources = append(sources, sq.Expr(feedCustomFollowed, constants.EdgeFollowEntityType, ownerID))
	}
	if len(def.Sources.Actors) > 0 {
		sources = append(sources, sq.Eq{"node.owner_id": def.Sources.Actors})
	}
	switch {
	case len(sources) > 0:
		matches = append(matches, sources)
	case def.HasSources():
		unreadable = true
	}

	if len(def.Filters.Hashtags) > 0 {
		matches = append(matches, feedCustomTaggedWith(def.Filters.Hashtags))
	}

	if len(def.Filters.Locations) > 0 {
		matches = append(matches, feedCustomPlacedAt(def.Filters.Locations))
	}

	if mentions := feedCustomMentions(def.Filters.Keywords); len(mentions) > 0 {
		matches = append(matches, mentions)
	}

	if def.NeedsPoint() {
		unreadable = true
	} else if near := def.Filters.Near; near != nil {
		metres := near.RadiusKm * 1000
		matches = append(matches, sq.Expr(feedCustomNear,
			constants.EdgeTagLocationType, near.Longitude, near.Latitude, metres,
			near.Longitude, near.Latitude, metres))
	}

	return matches, unreadable
}

// feedCustomFilters is what a post must be, whatever it matches: on a kind
// of profile, in a language, written here or elsewhere.
func feedCustomFilters(query sq.SelectBuilder, def *feeds.Definition) sq.SelectBuilder {
	if types := def.ActorEntityTypes(); len(types) > 0 {
		query = query.Where(sq.Eq{"owner.type": types})
	}

	if def.Filters.Language != "" {
		query = query.Where(sq.Eq{"node.language": def.Filters.Language})
	}

	// Where it was written: here, or on another server, which a post kept
	// from elsewhere says by its address there.
	switch def.Filters.Origin {
	case feeds.OriginHere:
		query = query.Where("(node.uri IS NULL OR node.uri = '')")
	case feeds.OriginElsewhere:
		query = query.Where("node.uri <> ''")
	}

	return query
}

// feedCustomExclusions leaves out what the feed says to, whatever else is
// true of the post.
func feedCustomExclusions(query sq.SelectBuilder, def *feeds.Definition) sq.SelectBuilder {
	if len(def.Exclude.Actors) > 0 {
		query = query.
			Where(sq.NotEq{"node.owner_id": def.Exclude.Actors}).
			Where(sq.Or{sq.Eq{"node.created_by": nil}, sq.NotEq{"node.created_by": def.Exclude.Actors}})
	}

	if len(def.Exclude.Hashtags) > 0 {
		tagged, args, _ := feedCustomTaggedWith(def.Exclude.Hashtags).ToSql()
		query = query.Where("NOT "+tagged, args...)
	}

	for _, match := range feedCustomMentions(def.Exclude.Keywords) {
		not, args, _ := match.ToSql()
		query = query.Where("NOT ("+not+")", args...)
	}

	return query
}

func feedCustomTaggedWith(hashtags []string) sq.Sqlizer {
	args := []any{constants.EdgeTagHashtagType, constants.NodeBaseHashtagEntityType}
	for _, hashtag := range hashtags {
		args = append(args, hashtag)
	}

	return sq.Expr(feedCustomTagged+sq.Placeholders(len(hashtags))+"))", args...)
}

func feedCustomPlacedAt(locations []int64) sq.Sqlizer {
	args := []any{constants.EdgeTagLocationType}
	for _, location := range locations {
		args = append(args, location)
	}

	return sq.Expr(feedCustomPlaced+sq.Placeholders(len(locations))+"))", args...)
}

// feedCustomMentions is one condition for each keyword: the post's title or
// its text holds it. LIKE's own characters are taken out of a keyword first,
// as search does, so "%" is not every post.
func feedCustomMentions(keywords []string) sq.Or {
	matches := sq.Or{}

	for _, keyword := range keywords {
		keyword = sanitizeSearchString(keyword)
		if keyword == "" {
			continue
		}

		matches = append(matches, sq.Or{
			sq.Like{"node.name": "%" + keyword + "%"},
			sq.Like{"node.body": "%" + keyword + "%"},
		})
	}

	return matches
}
