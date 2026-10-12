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

	query = feedCustomSources(query, def, ownerID)
	query = feedCustomFilters(query, def)
	query = feedCustomExclusions(query, def)

	// Replies are read in their thread, unless the feed asks for them.
	if !def.IncludeReplies {
		query = query.Where("node.root_id IS NULL")
	}

	// What may be seen, by whoever is reading. A feed is sent to people, so
	// what was archived is left out of it as well.
	query = SetFeedAccessQuery(query, viewer)
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

// feedCustomSources keeps the feed to where its posts come from: whoever its
// owner follows, the profiles chosen by hand, or both. With neither, the
// whole site, which the definition allows only through a narrowing filter.
func feedCustomSources(query sq.SelectBuilder, def *feeds.Definition, ownerID int64) sq.SelectBuilder {
	sources := sq.Or{}

	if def.Sources.Following && ownerID > 0 {
		sources = append(sources, sq.Expr(feedCustomFollowed, constants.EdgeFollowEntityType, ownerID))
	}
	if len(def.Sources.Actors) > 0 {
		sources = append(sources, sq.Eq{"node.owner_id": def.Sources.Actors})
	}

	switch {
	case len(sources) > 0:
		return query.Where(sources)
	case def.HasSources():
		// It names a source, and none of them can be read: whoever its
		// owner follows, with no owner. Nothing, and never the site.
		return query.Where("1 = 0")
	}

	return query
}

func feedCustomFilters(query sq.SelectBuilder, def *feeds.Definition) sq.SelectBuilder {
	if types := def.ActorEntityTypes(); len(types) > 0 {
		query = query.Where(sq.Eq{"owner.type": types})
	}

	if len(def.Filters.Hashtags) > 0 {
		query = query.Where(feedCustomTaggedWith(def.Filters.Hashtags))
	}

	if len(def.Filters.Locations) > 0 {
		query = query.Where(feedCustomPlacedAt(def.Filters.Locations))
	}

	if matches := feedCustomMentions(def.Filters.Keywords); len(matches) > 0 {
		query = query.Where(matches)
	}

	if def.Filters.Language != "" {
		query = query.Where(sq.Eq{"node.language": def.Filters.Language})
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
			sq.Like{"COALESCE(node.name, '')": "%" + keyword + "%"},
			sq.Like{"COALESCE(node.body, '')": "%" + keyword + "%"},
		})
	}

	return matches
}
