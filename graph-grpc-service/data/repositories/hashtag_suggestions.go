package repositories

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/feeds"
	"anahita-libs/proto/graphpb"
)

// Hashtags in use, offered to somebody writing (roadmap row 78): those that
// begin with what they have typed after a #, or those that are words of
// what they wrote.
//
// A hashtag is counted by the posts it is on that anybody may see (public,
// published, switched on, on a profile anybody may see) and by the
// writer's own, so that what is offered says nothing about what is kept
// from them. One they follow, in one of their custom feeds, comes first.
type HashtagSuggestionsRepoInterface interface {
	Suggest(req *graphpb.HashtagSuggestRequest) ([]*graphpb.HashtagSuggestion, uint64, error)
}

type hashtagSuggestionsRepo struct {
	db *sql.DB
}

func NewHashtagSuggestionsRepo(db *sql.DB) HashtagSuggestionsRepoInterface {
	return &hashtagSuggestionsRepo{db: db}
}

const (
	// The most offered at once, and the most words of a text looked at.
	maxHashtagSuggestions = 20
	maxSuggestionWords    = 200

	// A word of a text is offered as a hashtag only when it is one on more
	// than a single post: a word somebody once tagged is not yet a
	// hashtag people use.
	minUsesOfAWord = 2

	hashtagSuggestTimeout = 3 * time.Second
)

// {{match}} is which hashtags; {{exclude}} leaves out those already
// written. A post is counted, in the join itself, only when anybody may see
// it or it is the viewer's own; a hashtag no such post carries is counted
// as on none, and is listed only where none is enough: the ones the viewer
// follows.
const hashtagsWithUses = `
SELECT hashtag.id,
       hashtag.alias,
       COALESCE(NULLIF(hashtag.name, ''), hashtag.alias),
       COUNT(DISTINCT post.id) AS uses,
       LOWER(hashtag.alias) IN (
           SELECT LOWER(picked.tag)
           FROM nodes AS feed,
                JSON_TABLE(feed.meta, '$.definition.filters.hashtags_any[*]' COLUMNS (tag VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci PATH '$')) AS picked
           WHERE feed.type = ?
             AND feed.owner_id = ?
             AND feed.deleted_at IS NULL
       ) AS followed
FROM nodes AS hashtag
LEFT JOIN edges AS tagged
       ON tagged.node_a_id = hashtag.id
      AND tagged.type = ?
LEFT JOIN nodes AS post
       ON post.id = tagged.node_b_id
      AND post.enabled = 1
      AND post.deleted_at IS NULL
      AND (
          post.created_by = ?
          OR (
              post.access = 'public'
              AND post.draft = 0
              AND NOT EXISTS (
                  SELECT 1
                  FROM nodes AS owner
                  WHERE owner.id = post.owner_id
                    AND owner.access IS NOT NULL
                    AND owner.access <> 'public'
              )
          )
      )
WHERE hashtag.type = ?
  AND hashtag.deleted_at IS NULL
  AND {{match}}
  AND {{exclude}}
GROUP BY hashtag.id, hashtag.alias, hashtag.name
HAVING uses >= ?`

// The orders a listing may be in.
const (
	listByName = "name"
	listByUses = "uses"
)

// Suggest is the hashtags offered to somebody writing, or, with `list`,
// those listed on the Hashtags page (HashtagSuggestRequest), and how many
// there are in all for a listing.
func (r *hashtagSuggestionsRepo) Suggest(req *graphpb.HashtagSuggestRequest) ([]*graphpb.HashtagSuggestion, uint64, error) {
	viewerID := req.GetViewer().GetId()
	limit := req.GetLimit()
	if limit == 0 || limit > maxHashtagSuggestions {
		limit = maxHashtagSuggestions
	}

	var (
		match   string
		order   string
		minUses = 1
		args    = []any{feeds.NodeType, viewerID, constants.EdgeTagHashtagType, viewerID, constants.NodeBaseHashtagEntityType}
		after   []any
	)

	prefix := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(req.GetPrefix(), "#")))
	words := lowered(req.GetWords(), maxSuggestionWords)

	switch {
	case req.GetList():
		// The Hashtags page: those the viewer follows, or every one on a
		// post anybody may see, perhaps beginning with what was searched.
		match = "TRUE"
		if prefix != "" {
			match = "hashtag.alias LIKE ? ESCAPE '!'"
			args = append(args, likePrefix(prefix))
		}
		if req.GetFollowedOnly() {
			if viewerID <= 0 {
				return []*graphpb.HashtagSuggestion{}, 0, nil
			}
			match += " HAVING_FOLLOWED"
			minUses = 0
		}
		// By the name as it is shown, whatever its capitals.
		byName := "LOWER(COALESCE(NULLIF(hashtag.name, ''), hashtag.alias))"
		order = "uses DESC, " + byName
		if req.GetOrder() == listByName {
			order = byName
		}
	case prefix != "":
		// Begins with what was typed: followed first, then the hashtag that
		// is just that, then the most used.
		match = "hashtag.alias LIKE ? ESCAPE '!'"
		args = append(args, likePrefix(prefix))
		order = "followed DESC, (hashtag.alias = ?) DESC, uses DESC, hashtag.alias"
		after = append(after, prefix)
	case len(words) > 0:
		// Is one of the words: followed first, then the one used most in
		// the text, then the most used.
		match = "hashtag.alias IN (" + placeholders(len(words)) + ")"
		order = "followed DESC, FIELD(hashtag.alias, " + placeholders(len(words)) + "), uses DESC, hashtag.alias"
		for _, word := range words {
			args = append(args, word)
			after = append(after, word)
		}
		minUses = minUsesOfAWord
	default:
		return []*graphpb.HashtagSuggestion{}, 0, nil
	}

	exclude := "TRUE"
	if written := lowered(req.GetExclude(), constants.MaxHashtagsOfAPost*10); len(written) > 0 {
		exclude = "hashtag.alias NOT IN (" + placeholders(len(written)) + ")"
		for _, alias := range written {
			args = append(args, alias)
		}
	}

	args = append(args, minUses)

	// Only the followed: a condition on the grouped row, after HAVING.
	having := ""
	if strings.HasSuffix(match, " HAVING_FOLLOWED") {
		match = strings.TrimSuffix(match, " HAVING_FOLLOWED")
		having = "\n   AND followed = 1"
	}
	base := strings.NewReplacer("{{match}}", match, "{{exclude}}", exclude).Replace(hashtagsWithUses) + having

	ctx, cancel := context.WithTimeout(context.Background(), hashtagSuggestTimeout)
	defer cancel()

	var total uint64
	if req.GetList() {
		count := "SELECT COUNT(*) FROM (" + base + "\n) AS listed"
		if err := r.db.QueryRowContext(ctx, count, args...).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	query := base + "\nORDER BY " + order + "\nLIMIT ? OFFSET ?"
	pageArgs := append(append(append([]any{}, args...), after...), limit, req.GetOffset())

	rows, err := r.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	suggestions := []*graphpb.HashtagSuggestion{}
	for rows.Next() {
		each := &graphpb.HashtagSuggestion{}
		if err := rows.Scan(&each.Id, &each.Alias, &each.Name, &each.Uses, &each.Followed); err != nil {
			return nil, 0, err
		}
		suggestions = append(suggestions, each)
	}
	return suggestions, total, rows.Err()
}

// lowered is each of a list once, in lower case and in the order given,
// leaving out what is empty, and no more than most.
func lowered(list []string, most int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, each := range list {
		each = strings.ToLower(strings.TrimSpace(each))
		if each == "" || seen[each] {
			continue
		}
		seen[each] = true
		out = append(out, each)
		if len(out) == most {
			break
		}
	}
	return out
}

// likePrefix is a LIKE pattern for what begins with prefix, its own % and
// _ (a letter of hashtags) taken as themselves.
func likePrefix(prefix string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(prefix) + "%"
}
