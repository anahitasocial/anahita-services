package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// ErrNoInvite is answering, or taking back, an invitation that is not
// there: never made, already answered, withdrawn or lapsed.
var ErrNoInvite = errors.New("there is no such invitation waiting")

// Invite is one invitation: a person asked to follow an actor.
type Invite struct {
	ActorID   int64
	InviteeID int64
	InviterID int64
	Status    string
	CreatedAt time.Time
	EndsAt    time.Time
}

// InvitesRepoInterface is inviting people to follow an actor.
//
// The repository answers what the DATA allows: who is already a follower,
// who has blocked whom, who refused lately. Whether the inviter may invite
// to this actor at all is asked before it is called.
type InvitesRepoInterface interface {
	// Add invites each of these people, and says what became of each one.
	// maxWaiting is how many invitations to this actor may be waiting.
	Add(actor *entities.AnahitaNode, inviterID int64, inviteeIDs []int64, maxWaiting uint64) (map[int64]string, error)

	// Read returns the invitation waiting for this person to this actor, or
	// nil.
	Read(actorID int64, inviteeID int64) (*Invite, error)

	// ListForActor returns the invitations waiting to an actor, newest
	// first, and how many there are.
	ListForActor(actorID int64, limit uint64, offset uint64) ([]*Invite, uint64, error)

	// ListForInvitee returns the invitations waiting for a person.
	ListForInvitee(inviteeID int64, limit uint64, offset uint64) ([]*Invite, uint64, error)

	// Accept makes the person a follower and removes the invitation. It
	// returns the invitation it answered, or ErrNoInvite.
	Accept(actorID int64, inviteeID int64) (*Invite, error)

	// Decline keeps the invitation, marked refused, for the cooldown.
	Decline(actorID int64, inviteeID int64) (*Invite, error)

	// Delete withdraws an invitation that is waiting.
	Delete(actorID int64, inviteeID int64) (*Invite, error)

	// CountMadeLately is how many invitations this person made in the last
	// day that are still waiting.
	CountMadeLately(inviterID int64) (int, error)

	// Sweep removes what has run out: invitations nobody answered in time,
	// which it returns, and refusals whose cooldown is over.
	Sweep() ([]*Invite, error)

	// SweepClosedEvents removes the invitations still waiting to events
	// that are over or were called off, and returns them: there is nothing
	// left to say yes to.
	SweepClosedEvents() ([]*Invite, error)
}

func NewInvitesRepo(db *sql.DB) InvitesRepoInterface {
	return &invitesRepo{db: db, dbTimeout: constants.DBTimeout}
}

type invitesRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

const inviteColumns = `
  node_b_id,
  node_a_id,
  COALESCE(created_by, 0),
  COALESCE(meta, ''),
  COALESCE(created_at, NOW()),
  COALESCE(ends_at, NOW())`

const selectInviteWaiting = `
SELECT` + inviteColumns + `
FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND node_a_id = ?
  AND meta = ?
  AND ends_at > NOW()`

const selectInvitesForActor = `
SELECT` + inviteColumns + `
FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND meta = ?
  AND ends_at > NOW()
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?`

const countInvitesForActor = `
SELECT COUNT(*)
FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND meta = ?
  AND ends_at > NOW()`

const selectInvitesForInvitee = `
SELECT` + inviteColumns + `
FROM edges
WHERE type = ?
  AND node_a_id = ?
  AND meta = ?
  AND ends_at > NOW()
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?`

const countInvitesForInvitee = `
SELECT COUNT(*)
FROM edges
WHERE type = ?
  AND node_a_id = ?
  AND meta = ?
  AND ends_at > NOW()`

const countInvitesMadeLately = `
SELECT COUNT(*)
FROM edges
WHERE type = ?
  AND created_by = ?
  AND meta = ?
  AND created_at > NOW() - INTERVAL 1 DAY`

const insertInvite = `
INSERT INTO edges (
  type,
  node_a_id,
  node_a_type,
  node_b_id,
  node_b_type,
  meta,
  created_at,
  created_by,
  updated_at,
  modified_by,
  ends_at
) VALUES (?, ?, ?, ?, ?, ?, NOW(), ?, NOW(), ?, ?)`

const deleteInvite = `
DELETE FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND node_a_id = ?`

const declineInvite = `
UPDATE edges
SET meta = ?,
    updated_at = NOW(),
    modified_by = ?
WHERE type = ?
  AND node_b_id = ?
  AND node_a_id = ?
  AND meta = ?
  AND ends_at > NOW()`

const insertFollowFromInvite = `
INSERT IGNORE INTO edges (
  type,
  node_a_id,
  node_a_type,
  node_b_id,
  node_b_type,
  created_at,
  created_by,
  updated_at,
  modified_by
)
SELECT ?, person.id, person.type, actor.id, actor.type, NOW(), ?, NOW(), ?
FROM nodes AS person
JOIN nodes AS actor ON actor.id = ?
WHERE person.id = ?`

const deleteFollowRequest = `
DELETE FROM edges
WHERE type = ?
  AND node_a_id = ?
  AND node_b_id = ?`

const selectInvitesRunOut = `
SELECT` + inviteColumns + `
FROM edges
WHERE type = ?
  AND meta = ?
  AND ends_at <= NOW()`

const deleteInvitesRunOut = `
DELETE FROM edges
WHERE type = ?
  AND meta = ?
  AND ends_at <= NOW()`

const deleteRefusalsPastCooldown = `
DELETE FROM edges
WHERE type = ?
  AND meta = ?
  AND updated_at <= ?`

func inviteStatus(meta string) string {
	if meta == constants.InviteMetaDeclined {
		return constants.InviteStatusDeclined
	}
	return constants.InviteStatusPending
}

func scanInvite(row interface{ Scan(...any) error }) (*Invite, error) {
	var (
		invite = &Invite{}
		meta   string
	)
	if err := row.Scan(&invite.ActorID, &invite.InviteeID, &invite.InviterID, &meta, &invite.CreatedAt, &invite.EndsAt); err != nil {
		return nil, err
	}
	invite.Status = inviteStatus(meta)

	return invite, nil
}

func (r *invitesRepo) list(query string, args ...any) ([]*Invite, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invites := []*Invite{}
	for rows.Next() {
		invite, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		invites = append(invites, invite)
	}

	return invites, rows.Err()
}

// idSet runs a query that returns one column of ids.
func (r *invitesRepo) idSet(query string, args ...any) (map[int64]bool, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	set := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		set[id] = true
	}

	return set, rows.Err()
}

func idPlaceholders(ids []int64) (string, []any) {
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

func (r *invitesRepo) Read(actorID int64, inviteeID int64) (*Invite, error) {
	invite, err := scanInvite(r.db.QueryRow(selectInviteWaiting,
		constants.EdgeActorInviteEntityType, actorID, inviteeID, constants.InviteMetaPending))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return invite, err
}

func (r *invitesRepo) ListForActor(actorID int64, limit uint64, offset uint64) ([]*Invite, uint64, error) {
	var total uint64
	if err := r.db.QueryRow(countInvitesForActor,
		constants.EdgeActorInviteEntityType, actorID, constants.InviteMetaPending).Scan(&total); err != nil {
		return nil, 0, err
	}

	invites, err := r.list(selectInvitesForActor,
		constants.EdgeActorInviteEntityType, actorID, constants.InviteMetaPending, limit, offset)

	return invites, total, err
}

func (r *invitesRepo) ListForInvitee(inviteeID int64, limit uint64, offset uint64) ([]*Invite, uint64, error) {
	var total uint64
	if err := r.db.QueryRow(countInvitesForInvitee,
		constants.EdgeActorInviteEntityType, inviteeID, constants.InviteMetaPending).Scan(&total); err != nil {
		return nil, 0, err
	}

	invites, err := r.list(selectInvitesForInvitee,
		constants.EdgeActorInviteEntityType, inviteeID, constants.InviteMetaPending, limit, offset)

	return invites, total, err
}

func (r *invitesRepo) CountMadeLately(inviterID int64) (int, error) {
	var total int
	err := r.db.QueryRow(countInvitesMadeLately,
		constants.EdgeActorInviteEntityType, inviterID, constants.InviteMetaPending).Scan(&total)

	return total, err
}

// Add invites each person, in the order given, and says what became of each.
//
// A person can be invited when they are a person who is there, they follow
// the inviter, they do not follow the actor already and have not asked to,
// nobody among the three has blocked another, and they have not refused this
// actor within the cooldown. An invitation that is already waiting is left
// as it is.
func (r *invitesRepo) Add(actor *entities.AnahitaNode, inviterID int64, inviteeIDs []int64, maxWaiting uint64) (map[int64]string, error) {
	results := map[int64]string{}
	if actor == nil || len(inviteeIDs) == 0 {
		return results, nil
	}

	in, ids := idPlaceholders(inviteeIDs)

	people, err := r.idSet(`
SELECT id
FROM nodes
WHERE id IN (`+in+`)
  AND type = ?
  AND enabled = 1
  AND deleted_at IS NULL
  AND archived_at IS NULL`,
		append(ids, constants.NodeActorPersonEntityType)...)
	if err != nil {
		return nil, err
	}

	followInviter, err := r.idSet(`
SELECT node_a_id
FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND node_a_id IN (`+in+`)`,
		append([]any{constants.EdgeFollowEntityType, inviterID}, ids...)...)
	if err != nil {
		return nil, err
	}

	followActor, err := r.idSet(`
SELECT node_a_id
FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND node_a_id IN (`+in+`)`,
		append([]any{constants.EdgeFollowEntityType, actor.ID}, ids...)...)
	if err != nil {
		return nil, err
	}

	asked, err := r.idSet(`
SELECT node_a_id
FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND node_a_id IN (`+in+`)`,
		append([]any{constants.EdgeFollowRequestEntityType, actor.ID}, ids...)...)
	if err != nil {
		return nil, err
	}

	// Blocked either way, with the inviter or with the actor.
	blockArgs := append([]any{constants.EdgeBlockEntityType}, ids...)
	blockArgs = append(blockArgs, inviterID, actor.ID, constants.EdgeBlockEntityType, inviterID, actor.ID)
	blockArgs = append(blockArgs, ids...)
	blocked, err := r.idSet(`
SELECT node_a_id
FROM edges
WHERE type = ?
  AND node_a_id IN (`+in+`)
  AND node_b_id IN (?, ?)
UNION
SELECT node_b_id
FROM edges
WHERE type = ?
  AND node_a_id IN (?, ?)
  AND node_b_id IN (`+in+`)`,
		blockArgs...)
	if err != nil {
		return nil, err
	}

	// What there is already between each person and this actor.
	type standing struct {
		meta    string
		ended   bool
		refused time.Time
	}
	existing := map[int64]standing{}
	rows, err := r.db.Query(`
SELECT node_a_id,
       COALESCE(meta, ''),
       COALESCE(ends_at, NOW()) <= NOW(),
       COALESCE(updated_at, NOW())
FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND node_a_id IN (`+in+`)`,
		append([]any{constants.EdgeActorInviteEntityType, actor.ID}, ids...)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			id int64
			s  standing
		)
		if err := rows.Scan(&id, &s.meta, &s.ended, &s.refused); err != nil {
			rows.Close()
			return nil, err
		}
		existing[id] = s
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var waiting uint64
	if err := r.db.QueryRow(countInvitesForActor,
		constants.EdgeActorInviteEntityType, actor.ID, constants.InviteMetaPending).Scan(&waiting); err != nil {
		return nil, err
	}

	endsAt := time.Now().Add(constants.InviteLifetime)

	for _, id := range inviteeIDs {
		if _, done := results[id]; done {
			continue
		}

		switch {
		case id == inviterID || id == actor.ID || !people[id]:
			results[id] = constants.InviteResultUnavailable
			continue
		case blocked[id]:
			results[id] = constants.InviteResultBlocked
			continue
		case followActor[id]:
			results[id] = constants.InviteResultAlreadyFollower
			continue
		case asked[id]:
			results[id] = constants.InviteResultRequested
			continue
		case !followInviter[id]:
			results[id] = constants.InviteResultNotFollower
			continue
		}

		if had, ok := existing[id]; ok {
			if had.meta == constants.InviteMetaPending && !had.ended {
				results[id] = constants.InviteResultPending
				continue
			}
			if had.meta == constants.InviteMetaDeclined && time.Since(had.refused) < constants.InviteCooldown {
				results[id] = constants.InviteResultCooldown
				continue
			}
			// Lapsed, or refused long enough ago: it gives way to a new one.
			if _, err := r.db.Exec(deleteInvite, constants.EdgeActorInviteEntityType, actor.ID, id); err != nil {
				return nil, err
			}
		}

		if waiting >= maxWaiting {
			results[id] = constants.InviteResultFull
			continue
		}

		if _, err := r.db.Exec(insertInvite,
			constants.EdgeActorInviteEntityType,
			id, constants.NodeActorPersonEntityType,
			actor.ID, actor.Type,
			constants.InviteMetaPending,
			inviterID, inviterID,
			endsAt,
		); err != nil {
			return nil, err
		}

		waiting++
		results[id] = constants.InviteResultInvited
	}

	return results, nil
}

// Accept makes the follow and removes the invitation, together or not at
// all. A request to follow that was waiting from the same person goes too:
// it has been answered.
func (r *invitesRepo) Accept(actorID int64, inviteeID int64) (*Invite, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	invite, err := scanInvite(tx.QueryRowContext(ctx, selectInviteWaiting+"\nFOR UPDATE",
		constants.EdgeActorInviteEntityType, actorID, inviteeID, constants.InviteMetaPending))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoInvite
	}
	if err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, insertFollowFromInvite,
		constants.EdgeFollowEntityType, inviteeID, inviteeID, actorID, inviteeID); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, deleteInvite, constants.EdgeActorInviteEntityType, actorID, inviteeID); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, deleteFollowRequest, constants.EdgeFollowRequestEntityType, inviteeID, actorID); err != nil {
		return nil, err
	}

	return invite, tx.Commit()
}

func (r *invitesRepo) Decline(actorID int64, inviteeID int64) (*Invite, error) {
	invite, err := r.Read(actorID, inviteeID)
	if err != nil {
		return nil, err
	}
	if invite == nil {
		return nil, ErrNoInvite
	}

	result, err := r.db.Exec(declineInvite,
		constants.InviteMetaDeclined, inviteeID,
		constants.EdgeActorInviteEntityType, actorID, inviteeID, constants.InviteMetaPending)
	if err != nil {
		return nil, err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return nil, ErrNoInvite
	}

	invite.Status = constants.InviteStatusDeclined

	return invite, nil
}

func (r *invitesRepo) Delete(actorID int64, inviteeID int64) (*Invite, error) {
	invite, err := r.Read(actorID, inviteeID)
	if err != nil {
		return nil, err
	}
	if invite == nil {
		return nil, ErrNoInvite
	}

	if _, err = r.db.Exec(deleteInvite, constants.EdgeActorInviteEntityType, actorID, inviteeID); err != nil {
		return nil, err
	}

	return invite, nil
}

func (r *invitesRepo) Sweep() ([]*Invite, error) {
	lapsed, err := r.list(selectInvitesRunOut, constants.EdgeActorInviteEntityType, constants.InviteMetaPending)
	if err != nil {
		return nil, err
	}

	if _, err = r.db.Exec(deleteInvitesRunOut, constants.EdgeActorInviteEntityType, constants.InviteMetaPending); err != nil {
		return nil, err
	}

	if _, err = r.db.Exec(deleteRefusalsPastCooldown,
		constants.EdgeActorInviteEntityType, constants.InviteMetaDeclined,
		time.Now().Add(-constants.InviteCooldown)); err != nil {
		return nil, err
	}

	return lapsed, nil
}

// The invitations waiting to an event nobody can go to any more.
const invitesToClosedEvents = `
FROM edges
WHERE type = ?
  AND meta = ?
  AND node_b_id IN (
    SELECT closed.id FROM (
      SELECT node.id
      FROM nodes AS node
      JOIN events AS event ON event.node_id = node.id
      WHERE node.type = ?
        AND (node.ends_at <= NOW() OR event.cancelled_at IS NOT NULL)
    ) AS closed
  )`

func (r *invitesRepo) SweepClosedEvents() ([]*Invite, error) {
	args := []any{constants.EdgeActorInviteEntityType, constants.InviteMetaPending, constants.NodeActorEventEntityType}

	gone, err := r.list("SELECT"+inviteColumns+invitesToClosedEvents, args...)
	if err != nil {
		return nil, err
	}
	if len(gone) == 0 {
		return gone, nil
	}

	if _, err = r.db.Exec("DELETE"+invitesToClosedEvents, args...); err != nil {
		return nil, err
	}

	return gone, nil
}
