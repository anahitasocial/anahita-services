package repositories

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// The administration list of accounts: every person or every group, with how
// much each has written and when it was last active.
//
// It exists for site cleanup. A super administrator filters down to the
// accounts nobody uses — never onboarded, nothing written, quiet for years —
// and removes them.
//
// READ ONLY, and it reads across what several services own: the actor in
// `nodes`, what it wrote in `nodes` again, its administrators in `edges`, and
// the sign-in record that is this service's own. There is no other place the
// question "who has not been here since 2019" can be answered in one query,
// and answering it in several would mean filtering and paging by hand.

const (
	AccountKindPerson = "person"
	AccountKindGroup  = "group"

	AccountTierEmpty   = "empty"
	AccountTierDormant = "dormant"
	AccountTierActive  = "active"

	AccountStateActive   = "active"
	AccountStateDisabled = "disabled"
	AccountStateArchived = "archived"
	AccountStateDeleted  = "deleted"

	AccountSortLastActive = "last_active_at"
	AccountSortCreated    = "created_at"
	AccountSortName       = "name"

	accountPersonType = "node.actor.person-service.person.v1"
	accountGroupType  = "node.actor.group-service.group.v1"
	accountAdminEdge  = "edge.actor.admin.v1"

	accountsTimeout = 10 * time.Second
)

// Account is one row of the list.
type Account struct {
	ID         int64
	Type       string
	Alias      string
	Name       string
	PersonType string
	// State is active, disabled, archived or deleted. Deleted means waiting
	// out the thirty days; a purged account is not listed at all.
	State string
	// Tier is empty, dormant or active. See AccountFilter.DormantBefore.
	Tier          string
	CreatedAt     time.Time
	LastActiveAt  time.Time
	EmailVerified bool
	Onboarded     bool
	ContentCount  int64
	FollowerCount int64
	LeaderCount   int64
	AdminCount    int64
}

// AccountFilter narrows the list. The zero value of each field means "do not
// filter on this".
type AccountFilter struct {
	// Kind is AccountKindPerson or AccountKindGroup. Required.
	Kind string

	Search string
	Tier   string
	State  string

	LastActiveBefore *time.Time
	CreatedBefore    *time.Time

	// OnlyUnverified and OnlyNotOnboarded are for people.
	OnlyUnverified   bool
	OnlyNotOnboarded bool

	// ContentMax keeps accounts with at most this much content. Negative
	// means no limit, because zero is a limit worth asking for.
	ContentMax int64

	// OnlyWithoutAdmin is for groups.
	OnlyWithoutAdmin bool

	// DormantBefore is the line between dormant and active: an account that
	// has content or follows, and was last active before this, is dormant.
	DormantBefore time.Time

	Sort       string
	Descending bool
	Limit      uint64
	Offset     uint64
}

type AccountsInterface interface {
	Browse(filter *AccountFilter) ([]*Account, error)
	Count(filter *AccountFilter) (uint64, error)
}

func NewAccounts(db *sql.DB) AccountsInterface {
	return &accounts{db: db}
}

type accounts struct {
	db *sql.DB
}

// What an actor has written, and when it last wrote. For a person: what they
// are the author of, anywhere.
const accountContentWritten = `
  SELECT
    created_by       AS actor_id,
    COUNT(*)         AS total,
    MAX(created_on)  AS last_at
  FROM nodes
  WHERE created_by IS NOT NULL
    AND (type LIKE 'node.medium.%' OR type LIKE 'node.base.%')
  GROUP BY created_by`

// For a group: what was posted in it, by anybody.
const accountContentOwned = `
  SELECT
    owner_id         AS actor_id,
    COUNT(*)         AS total,
    MAX(created_on)  AS last_at
  FROM nodes
  WHERE owner_id IS NOT NULL
    AND type LIKE 'node.medium.%'
  GROUP BY owner_id`

// accountRows is one row per actor of a type, with everything the list shows
// or filters on worked out. The %s is one of the two content queries above.
//
// last_active_at is the latest of four things, because none of them can be
// trusted alone: the last visit (which for accounts carried over from the old
// site holds the day they joined), the last sign-in, the last thing written,
// and the day the account was created, which is the floor.
//
// The COALESCEs on the actor's own columns are for accounts carried over from
// the old site, where a counter nobody ever touched is NULL rather than zero.
//
// Placeholders, in order: the admin edge type, the actor type.
const accountRows = `
SELECT
  actor.id,
  actor.type,
  COALESCE(actor.alias, '')                AS alias,
  COALESCE(actor.name, '')                 AS name,
  COALESCE(actor.person_type, '')          AS person_type,
  CASE
    WHEN actor.deleted_at IS NOT NULL  THEN 'deleted'
    WHEN actor.enabled = 0             THEN 'disabled'
    WHEN actor.archived_at IS NOT NULL THEN 'archived'
    ELSE 'active'
  END                                      AS state,
  actor.created_on                         AS created_at,
  GREATEST(
    actor.created_on,
    COALESCE(actor.last_visit_date, actor.created_on),
    COALESCE(signin.last_at, actor.created_on),
    COALESCE(content.last_at, actor.created_on)
  )                                        AS last_active_at,
  account.email_verified_at IS NOT NULL    AS email_verified,
  account.onboarded_at IS NOT NULL         AS onboarded,
  COALESCE(content.total, 0)               AS content_count,
  COALESCE(actor.follower_count, 0)        AS follower_count,
  COALESCE(actor.leader_count, 0)          AS leader_count,
  COALESCE(admins.total, 0)                AS admin_count
FROM nodes actor
LEFT JOIN auth_accounts account
  ON account.node_id = actor.id
LEFT JOIN (
  SELECT
    created_by       AS actor_id,
    MAX(created_on)  AS last_at
  FROM auth_logs
  GROUP BY created_by
) signin
  ON signin.actor_id = actor.id
LEFT JOIN (%s
) content
  ON content.actor_id = actor.id
LEFT JOIN (
  SELECT
    node_b_id  AS actor_id,
    COUNT(*)   AS total
  FROM edges
  WHERE type = ?
  GROUP BY node_b_id
) admins
  ON admins.actor_id = actor.id
WHERE actor.type = ?
  AND actor.purged_at IS NULL
  AND actor.created_on IS NOT NULL`

// accountTiers adds the tier to each row. The %s is accountRows.
//
// Empty: nothing written, follows nobody, and nobody follows it. A group is
// allowed one follower, since whoever starts a group follows it.
//
// Placeholders, ahead of accountRows': the followers an empty account may
// have, then the dormant line.
const accountTiers = `
SELECT
  listed.*,
  CASE
    WHEN listed.content_count = 0
     AND listed.leader_count = 0
     AND listed.follower_count <= ?  THEN 'empty'
    WHEN listed.last_active_at < ?   THEN 'dormant'
    ELSE 'active'
  END AS tier
FROM (%s
) listed`

// accountFilters keeps the rows a filter asks for. Each line is one filter,
// and each is switched off by its first placeholder.
const accountFilters = `
WHERE (? = '' OR found.tier = ?)
  AND (? = '' OR found.state = ?)
  AND (? IS NULL OR found.last_active_at < ?)
  AND (? IS NULL OR found.created_at < ?)
  AND (? = 0 OR found.email_verified = 0)
  AND (? = 0 OR found.onboarded = 0)
  AND (? < 0 OR found.content_count <= ?)
  AND (? = 0 OR found.admin_count = 0)
  AND (? = '' OR found.name LIKE ? OR found.alias LIKE ?)`

// The %s is accountTiers; the columns are in the order scanAccount reads.
const selectAccounts = `
SELECT
  found.id,
  found.type,
  found.alias,
  found.name,
  found.person_type,
  found.state,
  found.tier,
  found.created_at,
  found.last_active_at,
  found.email_verified,
  found.onboarded,
  found.content_count,
  found.follower_count,
  found.leader_count,
  found.admin_count
FROM (%s
) found` + accountFilters

const countAccounts = `
SELECT COUNT(*)
FROM (%s
) found` + accountFilters

// accountOrders are the orderings a caller may ask for. The id breaks ties,
// so paging through accounts created in the same second is stable.
var accountOrders = map[string]string{
	AccountSortLastActive: "found.last_active_at",
	AccountSortCreated:    "found.created_at",
	AccountSortName:       "found.name",
}

// statement fills the pieces into each other and returns the arguments for
// everything up to and including the filters.
func (filter *AccountFilter) statement(outer string) (string, []any) {
	actorType, content, emptyFollowers := accountPersonType, accountContentWritten, 0
	if filter.Kind == AccountKindGroup {
		actorType, content, emptyFollowers = accountGroupType, accountContentOwned, 1
	}

	rows := strings.Replace(accountRows, "%s", content, 1)
	tiers := strings.Replace(accountTiers, "%s", rows, 1)
	query := strings.Replace(outer, "%s", tiers, 1)

	var lastActiveBefore, createdBefore any
	if filter.LastActiveBefore != nil {
		lastActiveBefore = filter.LastActiveBefore.UTC()
	}
	if filter.CreatedBefore != nil {
		createdBefore = filter.CreatedBefore.UTC()
	}

	search := ""
	if filter.Search != "" {
		search = "%" + escapeLike(filter.Search) + "%"
	}

	args := []any{
		// accountTiers. Its placeholders come first: they are in its select
		// list, which is written before the rows it selects from.
		emptyFollowers,
		filter.DormantBefore.UTC(),
		// accountRows
		accountAdminEdge,
		actorType,
		// accountFilters, a line each
		filter.Tier, filter.Tier,
		filter.State, filter.State,
		lastActiveBefore, lastActiveBefore,
		createdBefore, createdBefore,
		filter.OnlyUnverified,
		filter.OnlyNotOnboarded,
		filter.ContentMax, filter.ContentMax,
		filter.OnlyWithoutAdmin,
		search, search, search,
	}

	return query, args
}

// escapeLike makes the characters LIKE treats as wildcards match themselves.
func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func (repo *accounts) Browse(filter *AccountFilter) ([]*Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), accountsTimeout)
	defer cancel()

	query, args := filter.statement(selectAccounts)

	// Never from the caller: the column comes out of accountOrders, and an
	// ordering that is not in it falls back to the default.
	column, known := accountOrders[filter.Sort]
	if !known {
		column = accountOrders[AccountSortLastActive]
	}
	direction := "ASC"
	if filter.Descending {
		direction = "DESC"
	}

	query += "\nORDER BY " + column + " " + direction + ", found.id " + direction +
		"\nLIMIT ? OFFSET ?"
	args = append(args, filter.Limit, filter.Offset)

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found []*Account
	for rows.Next() {
		account := &Account{}
		if err := rows.Scan(
			&account.ID,
			&account.Type,
			&account.Alias,
			&account.Name,
			&account.PersonType,
			&account.State,
			&account.Tier,
			&account.CreatedAt,
			&account.LastActiveAt,
			&account.EmailVerified,
			&account.Onboarded,
			&account.ContentCount,
			&account.FollowerCount,
			&account.LeaderCount,
			&account.AdminCount,
		); err != nil {
			return nil, err
		}
		found = append(found, account)
	}

	return found, rows.Err()
}

func (repo *accounts) Count(filter *AccountFilter) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), accountsTimeout)
	defer cancel()

	query, args := filter.statement(countAccounts)

	var total uint64
	err := repo.db.QueryRowContext(ctx, query, args...).Scan(&total)

	return total, err
}
