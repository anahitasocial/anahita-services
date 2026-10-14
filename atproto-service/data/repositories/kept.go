package repositories

import (
	"context"
	"database/sql"
)

// What is kept here of what people on Bluesky did to somebody here: their
// likes and reposts of posts that went out from here, their answers to
// them, and their follows of whoever connected an account. Read from the
// graph, where each is kept as anybody's is, so that each can be looked
// for on Bluesky again and found to have been taken back.

// KeptReaction is one like or repost: who by, and of which post.
type KeptReaction struct {
	ActorID  int64
	ActorDID string
	PostID   int64
	// The post as it is named on Bluesky.
	PostDID  string
	PostRKey string
}

// KeptReply is an answer from Bluesky under something that went out from
// here.
type KeptReply struct {
	ActorID int64
	URI     string
}

// KeptFollow is somebody on Bluesky following somebody here.
type KeptFollow struct {
	ActorID   int64
	ActorDID  string
	PersonID  int64
	PersonDID string
}

type KeptInterface interface {
	Likes(limit int) ([]*KeptReaction, error)
	Reposts(limit int) ([]*KeptReaction, error)
	Replies(limit int) ([]*KeptReply, error)
	Follows(limit int) ([]*KeptFollow, error)
}

type Kept struct {
	db *sql.DB
}

func NewKept(db *sql.DB) *Kept {
	return &Kept{db: db}
}

// The newest first, and not what was made in the last few minutes:
// Bluesky's reader takes a moment to know of something new, and what it
// does not know of yet is not gone.
const keptLikes = `
SELECT actor.node_id, actor.did, sent.local_id, sent.did, sent.rkey
FROM edges AS liked
JOIN atproto_actors AS actor ON actor.node_id = liked.node_a_id
JOIN atproto_sent AS sent ON sent.local_id = liked.node_b_id AND sent.kind = 'post'
WHERE liked.type = 'edge.base.voteup.v1'
  AND liked.created_at < NOW() - INTERVAL 3 MINUTE
ORDER BY liked.id DESC
LIMIT ?`

const keptReposts = `
SELECT actor.node_id, actor.did, sent.local_id, sent.did, sent.rkey
FROM nodes AS repost
JOIN atproto_actors AS actor ON actor.node_id = repost.owner_id
JOIN atproto_sent AS sent ON sent.local_id = repost.parent_id AND sent.kind = 'post'
WHERE repost.type = 'node.base.repost-service.repost.v1'
  AND repost.created_at < NOW() - INTERVAL 3 MINUTE
ORDER BY repost.id DESC
LIMIT ?`

func (repo *Kept) Likes(limit int) ([]*KeptReaction, error) {
	return repo.reactions(keptLikes, limit)
}

func (repo *Kept) Reposts(limit int) ([]*KeptReaction, error) {
	return repo.reactions(keptReposts, limit)
}

func (repo *Kept) reactions(query string, limit int) ([]*KeptReaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kept []*KeptReaction
	for rows.Next() {
		each := &KeptReaction{}
		if err := rows.Scan(&each.ActorID, &each.ActorDID, &each.PostID, &each.PostDID, &each.PostRKey); err != nil {
			return nil, err
		}
		kept = append(kept, each)
	}
	return kept, rows.Err()
}

// Answers from Bluesky under what went out from here, by people nobody
// here follows: of those who are followed, what is taken back is heard.
const keptReplies = `
SELECT actor.node_id, reply.uri
FROM nodes AS reply
JOIN atproto_actors AS actor ON actor.node_id = reply.owner_id
JOIN atproto_sent AS sent ON sent.local_id = reply.parent_id AND sent.kind = 'post'
WHERE reply.uri IS NOT NULL
  AND reply.deleted_at IS NULL
  AND reply.created_at < NOW() - INTERVAL 3 MINUTE
ORDER BY reply.id DESC
LIMIT ?`

func (repo *Kept) Replies(limit int) ([]*KeptReply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, keptReplies, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kept []*KeptReply
	for rows.Next() {
		each := &KeptReply{}
		if err := rows.Scan(&each.ActorID, &each.URI); err != nil {
			return nil, err
		}
		kept = append(kept, each)
	}
	return kept, rows.Err()
}

const keptFollows = `
SELECT actor.node_id, actor.did, account.person_id, account.did
FROM edges AS follow
JOIN atproto_actors AS actor ON actor.node_id = follow.node_a_id
JOIN atproto_accounts AS account ON account.person_id = follow.node_b_id
WHERE follow.type = 'edge.actor.follow.v1'
  AND follow.created_at < NOW() - INTERVAL 3 MINUTE
ORDER BY follow.id DESC
LIMIT ?`

func (repo *Kept) Follows(limit int) ([]*KeptFollow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, keptFollows, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kept []*KeptFollow
	for rows.Next() {
		each := &KeptFollow{}
		if err := rows.Scan(&each.ActorID, &each.ActorDID, &each.PersonID, &each.PersonDID); err != nil {
			return nil, err
		}
		kept = append(kept, each)
	}
	return kept, rows.Err()
}
