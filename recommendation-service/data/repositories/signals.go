package repositories

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	an_const "anahita-libs/constants"
	"anahita-libs/feeds"

	"recommendation/constants"
	"recommendation/scoring"
)

// The signals: each finds profiles the viewer might follow, for one reason,
// with how strongly. Reading only; this service writes nothing but its own
// two tables.
//
// EVERY SIGNAL IS KEPT TO PROFILES THAT MAY BE SUGGESTED, by the one clause
// below. What is left to the graph afterwards is who may SEE a profile,
// which is its own rule and is asked of it, not copied here.

// The candidate, `c`, may be suggested to the viewer: it is of the kind
// asked for, is somebody else, is there and in use, is not followed or asked
// to be followed already (for an event: not answered), has no block with the
// viewer either way, was not turned down by them, has not asked to be left
// out of suggestions, and, for a person, has confirmed their email address.
//
// {{kind}} is one more condition that depends on the kind asked for, and
// {{elsewhere}} is who else counts as that kind: see peopleElsewhere.
const eligibleCandidate = `(c.type = ?{{elsewhere}})
  AND c.id <> ?
  AND c.enabled = 1
  AND c.deleted_at IS NULL
  AND c.archived_at IS NULL
  {{kind}}
  AND NOT EXISTS (
      SELECT 1
      FROM edges AS held
      WHERE held.type IN (?, ?)
        AND held.node_a_id = ?
        AND held.node_b_id = c.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM edges AS blocked
      WHERE blocked.type = ?
        AND (
            (blocked.node_a_id = ? AND blocked.node_b_id = c.id)
            OR (blocked.node_a_id = c.id AND blocked.node_b_id = ?)
        )
  )
  AND NOT EXISTS (
      SELECT 1
      FROM recommendation_dismissals AS dismissed
      WHERE dismissed.person_id = ?
        AND dismissed.actor_id = c.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM recommendation_preferences AS hidden
      WHERE hidden.person_id = c.id
        AND hidden.discoverable = 0
  )
  AND (
      c.type <> ?
      OR EXISTS (
          SELECT 1
          FROM auth_accounts AS account
          WHERE account.node_id = c.id
            AND account.email_verified_at IS NOT NULL
      )
  )`

// A person or a group has a picture, or says something of itself: an empty
// profile is nothing to suggest.
const eligibleProfile = `AND (c.avatar_filename <> '' OR c.body <> '')`

// An event is still to come or on, and has not been called off. It has no
// picture of its own, and is not asked to say anything.
const eligibleEvent = `AND c.ends_at > NOW()
  AND NOT EXISTS (
      SELECT 1
      FROM events AS called_off
      WHERE called_off.node_id = c.id
        AND called_off.cancelled_at IS NOT NULL
  )`

// People on other networks are people too, to whoever is asking who to
// follow: somebody on Bluesky, or on a server of the fediverse, who is
// kept here because somebody here follows them or they did something
// here. Only the ones kept here can be suggested; this installation knows
// of nobody else.
//
// Somebody on Bluesky can be followed by anybody here. Somebody on the
// fediverse can be followed only by a person who is on the fediverse
// themselves, since the other server has to be able to find out who is
// asking: to anybody else they are not suggested, as a follow of them
// would be taken away again.
const peopleElsewhere = `
      OR c.type = '` + an_const.NodeActorATProtoPersonEntityType + `'
      OR (
          c.type = '` + an_const.NodeActorActivityPubPersonEntityType + `'
          AND EXISTS (
              SELECT 1
              FROM activitypub_accounts AS me
              WHERE me.person_id = {{viewer}}
                AND me.enabled = 1
          )
      )
  `

// eligibleFor is the clause for one kind, for one viewer.
func eligibleFor(kind string, viewerID int64) string {
	extra := eligibleProfile
	if kind == constants.KindEvent {
		extra = eligibleEvent
	}
	elsewhere := ""
	if kind == constants.KindPerson {
		elsewhere = strings.Replace(peopleElsewhere, "{{viewer}}", strconv.FormatInt(viewerID, 10), 1)
	}

	clause := strings.Replace(eligibleCandidate, "{{kind}}", extra, 1)
	return strings.Replace(clause, "{{elsewhere}}", elsewhere, 1)
}

func eligibleArgs(kindType string, viewerID int64) []any {
	return []any{
		kindType,
		viewerID,
		an_const.EdgeFollowEntityType, an_const.EdgeFollowRequestEntityType,
		viewerID,
		an_const.EdgeBlockEntityType,
		viewerID, viewerID,
		viewerID,
		constants.PersonType,
	}
}

// They follow the viewer, who does not follow them.
const signalFollowsYou = `
SELECT c.id, 1, ''
FROM edges AS follows
JOIN nodes AS c ON c.id = follows.node_a_id
WHERE follows.type = ?
  AND follows.node_b_id = ?
  AND {{eligible}}
LIMIT ?`

// People the viewer follows, follow them. With which of those people.
const signalFollowedByLeaders = `
SELECT c.id, COUNT(DISTINCT mine.node_b_id), GROUP_CONCAT(DISTINCT mine.node_b_id ORDER BY mine.node_b_id)
FROM edges AS mine
JOIN nodes AS leader ON leader.id = mine.node_b_id
JOIN edges AS theirs ON theirs.node_a_id = mine.node_b_id
JOIN nodes AS c ON c.id = theirs.node_b_id
WHERE mine.type = ?
  AND mine.node_a_id = ?
  AND leader.type = ?
  AND theirs.type = ?
  AND {{eligible}}
GROUP BY c.id
ORDER BY 2 DESC, c.id
LIMIT ?`

// Whoever invited the viewer, and whoever the viewer invited.
const signalInviter = `
SELECT c.id, 1, ''
FROM auth_accounts AS mine
JOIN nodes AS c ON c.id = mine.invited_by
WHERE mine.node_id = ?
  AND {{eligible}}
UNION
SELECT c.id, 1, ''
FROM auth_accounts AS theirs
JOIN nodes AS c ON c.id = theirs.node_id
WHERE theirs.invited_by = ?
  AND {{eligible}}
LIMIT ?`

// A group the viewer follows is hosting it. With which groups.
const signalHostedByLeader = `
SELECT c.id, COUNT(DISTINCT hosted.node_a_id), GROUP_CONCAT(DISTINCT hosted.node_a_id ORDER BY hosted.node_a_id)
FROM edges AS mine
JOIN edges AS hosted ON hosted.node_a_id = mine.node_b_id
JOIN nodes AS c ON c.id = hosted.node_b_id
WHERE mine.type = ?
  AND mine.node_a_id = ?
  AND hosted.type = ?
  AND {{eligible}}
GROUP BY c.id
ORDER BY 2 DESC, c.id
LIMIT ?`

// Their public posts carry hashtags the viewer has used or liked. With
// which hashtags. Only public posts are looked at: the hashtags come back
// as the reason, and a reason must not say what is in a post the viewer
// could not read.
const signalSharedInterest = `
SELECT c.id, COUNT(DISTINCT theirs.node_a_id), GROUP_CONCAT(DISTINCT theirs.node_a_id ORDER BY theirs.node_a_id)
FROM edges AS theirs
JOIN nodes AS post ON post.id = theirs.node_b_id
JOIN nodes AS c ON c.id = post.owner_id
WHERE theirs.type = ?
  AND post.created_at > ?
  AND post.access = 'public'
  AND post.deleted_at IS NULL
  AND theirs.node_a_id IN (
      SELECT interest.node_a_id
      FROM edges AS interest
      JOIN nodes AS mine ON mine.id = interest.node_b_id
      WHERE interest.type = ?
        AND mine.created_at > ?
        AND (
            mine.created_by = ?
            OR EXISTS (
                SELECT 1
                FROM edges AS liked
                WHERE liked.type = ?
                  AND liked.node_a_id = ?
                  AND liked.node_b_id = mine.id
            )
        )
  )
  AND {{eligible}}
GROUP BY c.id
ORDER BY 2 DESC, c.id
LIMIT ?`

// Their public posts carry hashtags the viewer follows: the hashtags of the
// custom feeds the viewer owns, which is what following a hashtag makes,
// and what the Interests step of signing up does with what is picked. A
// choice the viewer made, so it is counted even when they asked that what
// they do not be, and for somebody who has just joined.
const signalFollowedHashtags = `
SELECT c.id, COUNT(DISTINCT theirs.node_a_id), GROUP_CONCAT(DISTINCT theirs.node_a_id ORDER BY theirs.node_a_id)
FROM edges AS theirs
JOIN nodes AS hashtag ON hashtag.id = theirs.node_a_id
JOIN nodes AS post ON post.id = theirs.node_b_id
JOIN nodes AS c ON c.id = post.owner_id
WHERE theirs.type = ?
  AND post.created_at > ?
  AND post.access = 'public'
  AND post.deleted_at IS NULL
  AND LOWER(hashtag.alias) IN (
      SELECT LOWER(picked.tag)
      FROM nodes AS feed,
           JSON_TABLE(feed.meta, '$.definition.filters.hashtags_any[*]' COLUMNS (tag VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci PATH '$')) AS picked
      WHERE feed.type = ?
        AND feed.owner_id = ?
        AND feed.deleted_at IS NULL
  )
  AND {{eligible}}
GROUP BY c.id
ORDER BY 2 DESC, c.id
LIMIT ?`

// The hashtags on the most public posts lately: what somebody who has just
// joined is offered to pick from. Only public posts, of profiles anybody
// may see, so that a hashtag offered says nothing about what is kept from
// view. Counted by posts, and by each spelling of a hashtag; spellings
// that differ only in case are put together afterwards. A hashtag's
// `enabled` means nothing: the services make every hashtag with it off, and
// asking for it on left only hashtags from before 2023.
const popularHashtags = `
SELECT hashtag.id, hashtag.name, hashtag.alias, COUNT(DISTINCT post.id)
FROM edges AS tagged
JOIN nodes AS hashtag ON hashtag.id = tagged.node_a_id
JOIN nodes AS post ON post.id = tagged.node_b_id
LEFT JOIN nodes AS owner ON owner.id = post.owner_id
WHERE tagged.type = ?
  AND post.created_at > ?
  AND post.access = 'public'
  AND post.enabled = 1
  AND post.deleted_at IS NULL
  AND (owner.id IS NULL OR owner.access IS NULL OR owner.access = 'public')
  AND hashtag.deleted_at IS NULL
GROUP BY hashtag.id, hashtag.name, hashtag.alias
ORDER BY 4 DESC, hashtag.id
LIMIT ?`

// Interest is a hashtag offered to pick, and on how many public posts it
// was lately.
type Interest struct {
	// The hashtag's id.
	ID int64 `json:"id"`
	// The hashtag, as it is shown.
	Name string `json:"name"`
	// What it is found by.
	Alias string `json:"alias"`
	// How many posts it was on, lately.
	Count int `json:"count"`
}

// The viewer has liked what is posted on their profile, lately.
const signalYouEngaged = `
SELECT c.id, COUNT(*), ''
FROM edges AS liked
JOIN nodes AS post ON post.id = liked.node_b_id
JOIN nodes AS c ON c.id = post.owner_id
WHERE liked.type = ?
  AND liked.node_a_id = ?
  AND liked.created_at > ?
  AND {{eligible}}
GROUP BY c.id
ORDER BY 2 DESC, c.id
LIMIT ?`

// Many people have followed them lately.
const signalPopular = `
SELECT c.id, COUNT(*), ''
FROM edges AS follows
JOIN nodes AS c ON c.id = follows.node_b_id
WHERE follows.type = ?
  AND follows.created_at > ?
  AND {{eligible}}
GROUP BY c.id
ORDER BY 2 DESC, c.id
LIMIT ?`

// Others have cited them in the last year: their profile, or a public post
// of theirs or on their profile, shown within a published public article or
// topic. Each citer once, never themselves, and not the viewer, who knows
// whom they cited. Those named are the citers.
const signalCited = `
SELECT c.id, COUNT(DISTINCT citing.created_by),
       GROUP_CONCAT(DISTINCT citing.created_by ORDER BY citing.created_by)
FROM edges AS cite
JOIN nodes AS target ON target.id = cite.node_b_id
JOIN nodes AS citing ON citing.id = cite.node_a_id
JOIN nodes AS c ON c.id IN (target.id, target.owner_id, target.created_by)
WHERE cite.type = ?
  AND cite.created_at > ?
  AND (target.id = c.id OR target.access = 'public')
  AND citing.enabled = 1
  AND citing.deleted_at IS NULL
  AND citing.draft = 0
  AND citing.access = 'public'
  AND citing.created_by <> c.id
  AND citing.owner_id <> c.id
  AND citing.created_by <> ?
  AND {{eligible}}
GROUP BY c.id
ORDER BY 2 DESC, c.id
LIMIT ?`

// The same, by people the viewer follows only.
const signalCitedByLeaders = `
SELECT c.id, COUNT(DISTINCT citing.created_by),
       GROUP_CONCAT(DISTINCT citing.created_by ORDER BY citing.created_by)
FROM edges AS cite
JOIN nodes AS target ON target.id = cite.node_b_id
JOIN nodes AS citing ON citing.id = cite.node_a_id
JOIN nodes AS c ON c.id IN (target.id, target.owner_id, target.created_by)
WHERE cite.type = ?
  AND cite.created_at > ?
  AND citing.created_by IN (
    SELECT leads.node_b_id
    FROM edges AS leads
    WHERE leads.type = ?
      AND leads.node_a_id = ?
  )
  AND (target.id = c.id OR target.access = 'public')
  AND citing.enabled = 1
  AND citing.deleted_at IS NULL
  AND citing.draft = 0
  AND citing.access = 'public'
  AND citing.created_by <> c.id
  AND citing.owner_id <> c.id
  AND citing.created_by <> ?
  AND {{eligible}}
GROUP BY c.id
ORDER BY 2 DESC, c.id
LIMIT ?`

// The installation's administrators have featured them.
const signalFeatured = `
SELECT c.id, 1, ''
FROM nodes AS c
WHERE c.featured_at IS NOT NULL
  AND {{eligible}}
ORDER BY c.featured_at DESC
LIMIT ?`

// When each profile was last alive: visited, posted on, or failing both,
// made. A group that was started yesterday is alive.
const lastActiveOf = `
SELECT c.id, COALESCE(c.last_visited_at, c.created_at), (
    SELECT MAX(post.created_at)
    FROM nodes AS post
    WHERE post.owner_id = c.id
)
FROM nodes AS c
WHERE c.id IN (`

type SignalsInterface interface {
	// Find runs every signal that applies and returns what each found, by
	// the reason's name.
	Find(viewerID int64, kind string, useActivity, onlyColdStart bool, now time.Time) (map[string][]scoring.Hit, error)
	// LastActive says when each of these profiles was last visited or
	// posted on, or when it was made.
	LastActive(ids []int64) (map[int64]time.Time, error)
	// Interests is the hashtags on the most public posts since `since`, at
	// most `limit`, each on at least `fewest` posts, the most used first.
	Interests(since time.Time, fewest, limit int) ([]Interest, error)
}

type Signals struct {
	db      *sql.DB
	timeout time.Duration
}

func NewSignals(db *sql.DB) SignalsInterface {
	return &Signals{db: db, timeout: an_const.DBTimeout}
}

// KindType is the type a kind stands for, or "" for one there is not.
func KindType(kind string) string {
	switch kind {
	case constants.KindPerson:
		return constants.PersonType
	case constants.KindGroup:
		return constants.GroupType
	case constants.KindEvent:
		return constants.EventType
	}
	return ""
}

func (r *Signals) Find(viewerID int64, kind string, useActivity, onlyColdStart bool, now time.Time) (map[string][]scoring.Hit, error) {
	kindType := KindType(kind)
	found := map[string][]scoring.Hit{}
	if kindType == "" || viewerID <= 0 {
		return found, nil
	}

	eligible := eligibleArgs(kindType, viewerID)
	limit := constants.MaxCandidatesFromASignal
	isPerson := kind == constants.KindPerson

	type signal struct {
		reason string
		query  string
		args   []any
		// Left out for somebody who has just joined and follows nobody.
		needsFollows bool
		// Left out when the viewer has said not to use what they do.
		needsActivity bool
		// Only people follow back, and only people are invited.
		peopleOnly bool
		// Only an event has a group hosting it.
		eventsOnly bool
	}

	with := func(before []any, after ...any) []any {
		args := append([]any{}, before...)
		args = append(args, eligible...)
		return append(args, after...)
	}

	signals := []signal{
		{
			reason:       constants.ReasonFollowsYou,
			query:        signalFollowsYou,
			args:         with([]any{an_const.EdgeFollowEntityType, viewerID}, limit),
			needsFollows: true,
			peopleOnly:   true,
		},
		{
			reason:       constants.ReasonFollowedByLeaders,
			query:        signalFollowedByLeaders,
			args:         with([]any{an_const.EdgeFollowEntityType, viewerID, constants.PersonType, an_const.EdgeFollowEntityType}, limit),
			needsFollows: true,
		},
		{
			reason: constants.ReasonInviter,
			query:  signalInviter,
			args: append(
				with([]any{viewerID}),
				with([]any{viewerID}, limit)...,
			),
			peopleOnly: true,
		},
		{
			reason:       constants.ReasonHostedByLeader,
			query:        signalHostedByLeader,
			args:         with([]any{an_const.EdgeFollowEntityType, viewerID, an_const.EdgeActorHostEntityType}, limit),
			needsFollows: true,
			eventsOnly:   true,
		},
		{
			reason: constants.ReasonSharedInterest,
			query:  signalSharedInterest,
			args: with([]any{
				an_const.EdgeTagHashtagType, now.Add(-constants.InterestWindow),
				an_const.EdgeTagHashtagType, now.Add(-constants.InterestWindow),
				viewerID,
				an_const.EdgeBaseLikeEntityType, viewerID,
			}, limit),
			needsActivity: true,
		},
		{
			reason: constants.ReasonSharedInterest,
			query:  signalFollowedHashtags,
			args: with([]any{
				an_const.EdgeTagHashtagType, now.Add(-constants.InterestWindow),
				feeds.NodeType, viewerID,
			}, limit),
		},
		{
			reason:        constants.ReasonYouEngaged,
			query:         signalYouEngaged,
			args:          with([]any{an_const.EdgeBaseLikeEntityType, viewerID, now.Add(-constants.EngagedWindow)}, limit),
			needsFollows:  true,
			needsActivity: true,
		},
		{
			reason: constants.ReasonPopular,
			query:  signalPopular,
			args:   with([]any{an_const.EdgeFollowEntityType, now.Add(-constants.PopularWindow)}, limit),
		},
		{
			reason: constants.ReasonCited,
			query:  signalCited,
			args:   with([]any{an_const.EdgeMediumCitationType, now.Add(-constants.InterestWindow), viewerID}, limit),
		},
		{
			reason: constants.ReasonCitedByLeaders,
			query:  signalCitedByLeaders,
			args: with([]any{
				an_const.EdgeMediumCitationType, now.Add(-constants.InterestWindow),
				an_const.EdgeFollowEntityType, viewerID,
				viewerID,
			}, limit),
			needsFollows: true,
		},
		{
			reason: constants.ReasonFeatured,
			query:  signalFeatured,
			args:   with(nil, limit),
		},
	}

	for _, each := range signals {
		if each.peopleOnly && !isPerson {
			continue
		}
		if each.eventsOnly && kind != constants.KindEvent {
			continue
		}
		if each.needsActivity && !useActivity {
			continue
		}
		if each.needsFollows && onlyColdStart {
			continue
		}

		hits, err := r.hits(strings.ReplaceAll(each.query, "{{eligible}}", eligibleFor(kind, viewerID)), each.args)
		if err != nil {
			return nil, err
		}
		if len(hits) > 0 {
			// Two signals may give the same reason, as the hashtags somebody
			// used and the ones they follow do: put together.
			found[each.reason] = mergeHits(found[each.reason], hits)
		}
	}

	return found, nil
}

func (r *Signals) hits(query string, args []any) ([]scoring.Hit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []scoring.Hit
	for rows.Next() {
		var (
			hit   scoring.Hit
			count int64
			ids   sql.NullString
		)
		if err := rows.Scan(&hit.ID, &count, &ids); err != nil {
			return nil, err
		}
		hit.Count = int(count)
		hit.IDs = parseIDs(ids.String)
		hits = append(hits, hit)
	}

	return hits, rows.Err()
}

func (r *Signals) LastActive(ids []int64) (map[int64]time.Time, error) {
	active := map[int64]time.Time{}
	if len(ids) == 0 {
		return active, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := lastActiveOf + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id      int64
			visited sql.NullTime
			posted  sql.NullTime
		)
		if err := rows.Scan(&id, &visited, &posted); err != nil {
			return nil, err
		}

		last := visited.Time
		if posted.Valid && posted.Time.After(last) {
			last = posted.Time
		}
		if !last.IsZero() {
			active[id] = last
		}
	}

	return active, rows.Err()
}

// parseIDs reads "3,7,12" as ids, leaving out what is not one.
func parseIDs(list string) []int64 {
	if list == "" {
		return nil
	}

	var ids []int64
	for _, part := range strings.Split(list, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}

	return ids
}

// mergeHits puts two lists of hits for one reason together: a profile in
// both counts once, with its count the larger and its ids from both.
func mergeHits(a, b []scoring.Hit) []scoring.Hit {
	if len(a) == 0 {
		return b
	}
	at := make(map[int64]int, len(a))
	merged := append([]scoring.Hit{}, a...)
	for i, hit := range merged {
		at[hit.ID] = i
	}
	for _, hit := range b {
		i, ok := at[hit.ID]
		if !ok {
			at[hit.ID] = len(merged)
			merged = append(merged, hit)
			continue
		}
		if hit.Count > merged[i].Count {
			merged[i].Count = hit.Count
		}
		seen := map[int64]bool{}
		for _, id := range merged[i].IDs {
			seen[id] = true
		}
		for _, id := range hit.IDs {
			if !seen[id] {
				merged[i].IDs = append(merged[i].IDs, id)
				seen[id] = true
			}
		}
	}
	return merged
}

// Interests reads the hashtags most used lately on public posts.
func (r *Signals) Interests(since time.Time, fewest, limit int) ([]Interest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// More than asked for, since spellings are put together after.
	rows, err := r.db.QueryContext(ctx, popularHashtags, an_const.EdgeTagHashtagType, since, limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		found []Interest
		at    = map[string]int{}
	)
	for rows.Next() {
		var each Interest
		var count int64
		if err := rows.Scan(&each.ID, &each.Name, &each.Alias, &count); err != nil {
			return nil, err
		}
		each.Count = int(count)
		key := strings.ToLower(each.Alias)
		if i, ok := at[key]; ok {
			// The spelling used most is the one kept; it came first.
			found[i].Count += each.Count
			continue
		}
		at[key] = len(found)
		found = append(found, each)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Put together, a hashtag may have more posts than one before it.
	sort.SliceStable(found, func(i, j int) bool {
		return found[i].Count > found[j].Count
	})
	kept := make([]Interest, 0, limit)
	for _, each := range found {
		if each.Count >= fewest && len(kept) < limit {
			kept = append(kept, each)
		}
	}
	return kept, nil
}
