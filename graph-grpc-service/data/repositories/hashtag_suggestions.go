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
	Suggest(req *graphpb.HashtagSuggestRequest) ([]*graphpb.HashtagSuggestion, error)
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
// written; {{order}} puts first what matches best.
const suggestHashtags = `
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
JOIN edges AS tagged ON tagged.node_a_id = hashtag.id AND tagged.type = ?
JOIN nodes AS post ON post.id = tagged.node_b_id
LEFT JOIN nodes AS owner ON owner.id = post.owner_id
WHERE hashtag.type = ?
  AND hashtag.deleted_at IS NULL
  AND {{match}}
  AND {{exclude}}
  AND post.enabled = 1
  AND post.deleted_at IS NULL
  AND (
      post.created_by = ?
      OR (
          post.access = 'public'
          AND post.draft = 0
          AND (owner.id IS NULL OR owner.access IS NULL OR owner.access = 'public')
      )
  )
GROUP BY hashtag.id, hashtag.alias, hashtag.name
HAVING uses >= ?
ORDER BY followed DESC, {{order}}, uses DESC, hashtag.alias
LIMIT ?`

func (r *hashtagSuggestionsRepo) Suggest(req *graphpb.HashtagSuggestRequest) ([]*graphpb.HashtagSuggestion, error) {
	viewerID := req.GetViewer().GetId()
	limit := req.GetLimit()
	if limit == 0 || limit > maxHashtagSuggestions {
		limit = maxHashtagSuggestions
	}

	var (
		match   string
		order   string
		minUses = 1
		args    = []any{feeds.NodeType, viewerID, constants.EdgeTagHashtagType, constants.NodeBaseHashtagEntityType}
		after   []any
	)

	prefix := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(req.GetPrefix(), "#")))
	words := lowered(req.GetWords(), maxSuggestionWords)

	switch {
	case prefix != "":
		// Begins with what was typed; the hashtag that is just that first.
		match = "hashtag.alias LIKE ? ESCAPE '!'"
		args = append(args, likePrefix(prefix))
		order = "(hashtag.alias = ?) DESC"
		after = append(after, prefix)
	case len(words) > 0:
		// Is one of the words, the one used most in the text first.
		match = "hashtag.alias IN (" + placeholders(len(words)) + ")"
		order = "FIELD(hashtag.alias, " + placeholders(len(words)) + ")"
		for _, word := range words {
			args = append(args, word)
			after = append(after, word)
		}
		minUses = minUsesOfAWord
	default:
		return []*graphpb.HashtagSuggestion{}, nil
	}

	exclude := "TRUE"
	if written := lowered(req.GetExclude(), constants.MaxHashtagsOfAPost*10); len(written) > 0 {
		exclude = "hashtag.alias NOT IN (" + placeholders(len(written)) + ")"
		for _, alias := range written {
			args = append(args, alias)
		}
	}

	args = append(args, viewerID, minUses)
	args = append(args, after...)
	args = append(args, limit)

	query := strings.NewReplacer("{{match}}", match, "{{exclude}}", exclude, "{{order}}", order).Replace(suggestHashtags)

	ctx, cancel := context.WithTimeout(context.Background(), hashtagSuggestTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	suggestions := []*graphpb.HashtagSuggestion{}
	for rows.Next() {
		each := &graphpb.HashtagSuggestion{}
		if err := rows.Scan(&each.Id, &each.Alias, &each.Name, &each.Uses, &each.Followed); err != nil {
			return nil, err
		}
		suggestions = append(suggestions, each)
	}
	return suggestions, rows.Err()
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
