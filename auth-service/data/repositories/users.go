package repositories

import (
	"anahita-libs/constants"
	"auth/data/entities"
	"auth/requests"
	"context"
	"database/sql"
	"errors"
	"log"

	sq "github.com/Masterminds/squirrel"
)

// UserInterface is the user repository interface
type UsersInterface interface {
	Browse(req *requests.UserBrowseRequest) ([]*entities.User, error)
	ReadByID(id int64) (*entities.User, error)
	ReadByEmail(email string) (*entities.User, error)
	ReadByUsername(username string) (*entities.User, error)
	ReadByEmailOrUsername(identifier string) (*entities.User, error)
	Add(user *entities.User) (*entities.User, error)
	Edit(user *entities.User) (*entities.User, error)
	EditPassword(password string, id int64) error
	EditEmail(email string, id int64) error
	EditTosConsent(id int64, consent *entities.Consent) error
	EditPrivacyConsent(id int64, consent *entities.Consent) error
	MarkEmailVerified(id int64) error
	MarkOnboarded(id int64) error
	Delete(id int64) error
	IsEmail(email string) (bool, error)
	IsUsername(username string) (bool, error)
	CountSuperAdmins() (int, error)
	CountInstanceUsage() (*InstanceUsage, error)
}

// NewUsers creates a new user repository
func NewUsers(db *sql.DB) UsersInterface {
	return &Users{db: db}
}

// Users is the user repository
type Users struct {
	db *sql.DB
}

func (repo *Users) Browse(req *requests.UserBrowseRequest) ([]*entities.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := sq.Select(userBrowseColumns()...).From(
		"auth_accounts",
	).LeftJoin(
		"nodes ON auth_accounts.node_id = nodes.id",
	)

	if req.Disabled {
		query = query.Where(sq.Eq{"nodes.enabled": 0})
	} else {
		query = query.Where(sq.Eq{"nodes.enabled": 1})
	}

	if req.Keywords != "" {
		query = query.Where(sq.Or{
			sq.Like{"nodes.alias": "%" + req.Keywords + "%"},
			sq.Like{"auth_accounts.email": "%" + req.Keywords + "%"},
		})
	}

	if req.PersonType != "" {
		query = query.Where(sq.Eq{"nodes.person_type": req.PersonType})
	}

	// The plural form, which is not a query parameter — no endpoint sets it.
	// It exists for internal callers that need a set rather than one value,
	// and today that is the invite-accept notifier asking for every admin and
	// super-admin so it can tell them somebody joined.
	//
	// The field was here and unread, which is a quiet failure rather than a
	// loud one: the notifier set it, the filter never happened, and the
	// "new member joined" email went to every account on the site instead of
	// to the handful of administrators it names in its own comment.
	if len(req.PersonTypes) > 0 {
		query = query.Where(sq.Eq{"nodes.person_type": req.PersonTypes})
	}

	// One clause, not two. This block appeared twice — once bare and once with
	// a direction — so a browse with an ordering emitted
	// "ORDER BY created_at, created_at DESC".
	//
	// Checked with safeOrderColumn as well as by the request's oneof tag,
	// because ORDER BY is concatenated rather than bound. See ordering.go.
	//
	// Qualified with the table, because every one of the four sortable
	// columns lives on `nodes` now and this select joins two tables. A bare
	// `person_type` or `created_at` is an unknown column here, and the read
	// fails outright rather than sorting oddly.
	if column := safeOrderColumn(req.Ordering); column != "" {
		if req.Direction == "asc" {
			query = query.OrderBy("nodes." + column + " ASC")
		} else {
			query = query.OrderBy("nodes." + column + " DESC")
		}
	}

	query = query.Limit(req.Limit).Offset(req.Offset)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	log.Println(sql, args)

	rows, err := repo.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*entities.User
	for rows.Next() {
		item, err := scanBrowsedUser(rows)

		if err != nil {
			return nil, err
		}

		err = repo.afterRead(item)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	return items, nil
}

// ReadByID returns a user by id
func (repo *Users) ReadByID(id int64) (*entities.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := sq.Select(userColumns()...).From(
		"auth_accounts",
	).LeftJoin(
		"nodes ON auth_accounts.node_id = nodes.id",
	).Where(
		sq.Eq{"auth_accounts.node_id": id},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewUser()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	item, err = scanUser(row)

	if err != nil {
		return nil, err
	}

	err = repo.afterRead(item)
	if err != nil {
		return nil, err
	}

	return item, nil
}

// ReadByEmail returns a user by email
func (repo *Users) ReadByEmail(email string) (*entities.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := sq.Select(userColumns()...).From(
		"auth_accounts",
	).LeftJoin(
		"nodes ON auth_accounts.node_id = nodes.id",
	).Where(
		sq.Eq{"auth_accounts.email": email},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewUser()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	item, err = scanUser(row)

	if err != nil {
		return nil, err
	}

	err = repo.afterRead(item)
	if err != nil {
		return nil, err
	}

	return item, nil
}

// ReadByUsername returns a user by username
func (repo *Users) ReadByUsername(username string) (*entities.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := sq.Select(userColumns()...).From(
		"auth_accounts",
	).LeftJoin(
		"nodes ON auth_accounts.node_id = nodes.id",
	).Where(
		sq.Eq{"nodes.alias": username},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewUser()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	item, err = scanUser(row)

	if err != nil {
		return nil, err
	}

	err = repo.afterRead(item)
	if err != nil {
		return nil, err
	}

	return item, nil
}

// ReadByEmailOrUsername returns a user by email or username
func (repo *Users) ReadByEmailOrUsername(emailOrUsername string) (*entities.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := sq.Select(userColumns()...).From(
		"auth_accounts",
	).LeftJoin(
		"nodes ON auth_accounts.node_id = nodes.id",
	).Where(
		// Both sides qualified. `email` is unambiguous today but the join
		// brings two tables into scope, and the handle moved: it is
		// nodes.alias now, not a username column on this side.
		sq.Or{
			sq.Eq{"auth_accounts.email": emailOrUsername},
			sq.Eq{"nodes.alias": emailOrUsername},
		},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewUser()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	item, err = scanUser(row)

	if err != nil {
		return nil, err
	}

	err = repo.afterRead(item)
	if err != nil {
		return nil, err
	}

	return item, nil
}

// Add creates a new user
func (repo *Users) Add(user *entities.User) (*entities.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// The node is NOT created here any more.
	//
	// This used to INSERT INTO nodes directly, inside a transaction with the
	// row below. `nodes` belongs to graph-grpc-service, and writing it from
	// here meant signup, invite-accept and username change all bypassed every
	// validator, default and event the graph applies on actor creation — a
	// person node created by this path got no default features and published
	// no "actor added" event.
	//
	// The caller now creates the actor through ActorsGRPC and passes a user
	// whose ID is already set. See signup.go and invite_accept.go, which also
	// carry the compensating delete: the two writes are no longer one
	// transaction and cannot be, so a failure here has to unwind the node
	// rather than roll back.
	if user.ID == 0 {
		return nil, errors.New("user.ID must be set by the caller: create the actor through ActorsGRPC first")
	}

	user.Type = constants.NodeActorPersonEntityType
	user.Service = constants.PersonService

	// Only the credential half, plus what the person agreed to. The node
	// already exists — the caller created it through ActorsGRPC — and carries
	// the name, the handle and the authority level, none of which are copied
	// here any more.
	//
	// Consent is written by the same statement that creates the account rather
	// than by a later update, because an account that exists without a record
	// of what it accepted is the gap this closes. If the trail is empty, it is
	// empty for a reason the caller can explain.
	query := `
		INSERT INTO auth_accounts (
			node_id,
			email,
			password,
			email_verified_at,
			invited_by,
			tos_accepted,
			tos_accepted_at,
			tos_accepted_version,
			tos_accepted_ip,
			tos_accepted_user_agent,
			tos_accepted_os,
			pp_accepted,
			pp_accepted_at,
			pp_accepted_version,
			pp_accepted_ip,
			pp_accepted_user_agent,
			pp_accepted_os
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?
		)`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email,
		user.Password,
		// Set by the caller, not here. An invited person proved control of
		// their address by following the link in the invitation, so
		// invite-accept marks it verified; a self-serve signup has proved
		// nothing yet and leaves it nil.
		user.EmailVerifiedAt,
		// Also set by the caller: invite-accept from the token, approval from
		// the request. Zero is written as NULL, so a self-serve account does
		// not point at an actor 0 that does not exist.
		nullableID(user.InvitedBy),
		user.TermsOfService.Accepted,
		user.TermsOfService.AcceptedAt,
		user.TermsOfService.Version,
		user.TermsOfService.IP,
		user.TermsOfService.UserAgent,
		user.TermsOfService.OS,
		user.PrivacyPolicy.Accepted,
		user.PrivacyPolicy.AcceptedAt,
		user.PrivacyPolicy.Version,
		user.PrivacyPolicy.IP,
		user.PrivacyPolicy.UserAgent,
		user.PrivacyPolicy.OS,
	)
	if err != nil {
		return nil, err
	}

	return repo.ReadByID(user.ID)
}

// Edit updates a user
func (repo *Users) Edit(user *entities.User) (*entities.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// Email, username/alias and USERTYPE are none of them in this SET
	// clause. Each moves only through its own single-purpose method —
	// EditEmail, EditUsername — which the step-up-gated change flows
	// call. Usertype has no such method yet, and that is the point: until
	// a deliberate promote/demote flow exists, nothing should be able to
	// change a privilege level at all.
	//
	// Usertype was here until 2026-09-07 and had the same shape of hazard
	// as the other two. Nothing routed a change to it, so it was never
	// exploitable — but Edit is called while enrolling TOTP, verifying a
	// passcode and signing in, and a privilege level that can be rewritten
	// as a side effect of any of those is one bad read away from being
	// rewritten wrongly. It is exactly the trap totp_secret fell into: a
	// read that omits a column produces a zero value, and the next Edit
	// persists the zero over the real one.
	//
	// Email is what password reset delivers to, so whoever can set it
	// can take the account. Username is a sign-in identifier
	// (ReadByEmailOrUsername) and the profile slug, so whoever can set
	// it can be mistaken for somebody else.
	//
	// Leaving them here was a live hazard rather than a stylistic one:
	// Edit writes the whole row from the struct it is handed, so any
	// caller holding a User read before a concurrent change — a TOTP
	// verify, a recovery-code burn — would silently write the old
	// values back over the new ones.
	// `nodes` is no longer in this statement, and neither is `enabled`.
	//
	// The join existed to write nodes.enabled and nodes.updated_at alongside
	// the credential columns. Nothing routed a change to either: every caller
	// of Edit — TOTP enrolment, passcode verification, recovery-code burn,
	// sign-in — sets only TOTP fields, so `enabled` was written back with
	// whatever value the caller's read happened to hold.
	//
	// That is the same hazard the comment above describes for usertype and
	// email, one step from becoming real: a read that omits the column yields
	// false, and the next Edit disables the account as a side effect of
	// verifying a passcode. Removing the column removes the class.
	//
	// Enabling and disabling an actor is ActorService.SetEnabled, which is a
	// single-purpose RPC that says what it does.
	query := `
		UPDATE auth_accounts
		SET
			password = ?,
			totp_enabled = ?,
			totp_recovery_codes = ?,
			totp_last_verified_at = ?,
			totp_secret = ?
		WHERE
			node_id = ?`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		user.Password,
		user.TotpEnabled,
		user.TotpRecoveryCodes,
		user.TotpLastVerifiedAt,
		user.TotpSecret,
		user.ID,
	)

	if err != nil {
		return nil, err
	}

	return repo.ReadByID(user.ID)
}

// EditPassword updates a user's password
func (repo *Users) EditPassword(password string, id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE auth_accounts
		SET
			password = ?
		WHERE
			node_id = ?`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		password,
		id,
	)

	if err != nil {
		return err
	}

	return nil
}

// EditTosConsent records acceptance of the Terms of Service.
//
// One document per write. The two are versioned separately, so a revision to
// the Privacy Policy alone must re-ask for that and nothing else — and a single
// write covering both would re-stamp the Terms with a fresh date every time
// somebody accepted a privacy update, rewriting when they agreed to a document
// they had not been shown again.
//
// Single-purpose, like EditEmail and EditPassword beside it: Edit writes the
// whole row from whatever struct it was handed, so a caller holding a stale
// read would otherwise put an old acceptance back. This is the only thing that
// moves a consent record, which keeps the audit trail auditable.
func (repo *Users) EditTosConsent(id int64, consent *entities.Consent) error {
	return repo.editConsent(id, "tos", consent)
}

// EditPrivacyConsent records acceptance of the Privacy Policy. See
// EditTosConsent.
func (repo *Users) EditPrivacyConsent(id int64, consent *entities.Consent) error {
	return repo.editConsent(id, "pp", consent)
}

// editConsent writes one document's six columns.
//
// The prefix is chosen from two constants by the two callers above and is
// never caller-supplied, which is the only reason concatenating it into the
// statement is safe. Do not export this.
func (repo *Users) editConsent(id int64, prefix string, consent *entities.Consent) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE auth_accounts
		SET
			` + prefix + `_accepted            = ?,
			` + prefix + `_accepted_at         = ?,
			` + prefix + `_accepted_version    = ?,
			` + prefix + `_accepted_ip         = ?,
			` + prefix + `_accepted_user_agent = ?,
			` + prefix + `_accepted_os         = ?
		WHERE
			node_id = ?`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		consent.Accepted,
		consent.AcceptedAt,
		consent.Version,
		consent.IP,
		consent.UserAgent,
		consent.OS,
		id,
	)

	return err
}

// MarkEmailVerified stamps an account's address as confirmed.
//
// Single-purpose, like EditEmail and EditConsent beside it, and for the same
// reason: Edit writes the whole row from whatever struct it was handed, so a
// caller holding a stale read could unverify an address as a side effect of
// something unrelated.
//
// Idempotent, and deliberately does NOT re-stamp an address that is already
// verified — following a confirmation link twice should not move the date, and
// the date is the only record of when it happened.
func (repo *Users) MarkEmailVerified(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE auth_accounts
		SET email_verified_at = NOW()
		WHERE node_id = ? AND email_verified_at IS NULL`

	_, err := repo.db.ExecContext(ctx, query, id)

	return err
}

// MarkOnboarded records that a person has been through onboarding.
//
// Single-purpose for the same reason as MarkEmailVerified above it: Edit
// writes the whole row from the struct it is handed, and this is not a column
// anything else should be able to move as a side effect.
//
// Set once. A second call — the flow finished in two tabs, or a retried
// request — does not move the date, and a profile that later becomes
// incomplete does not re-open onboarding by clearing it; the web app's
// dashboard nudge covers that.
func (repo *Users) MarkOnboarded(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE auth_accounts
		SET onboarded_at = NOW()
		WHERE node_id = ? AND onboarded_at IS NULL`

	_, err := repo.db.ExecContext(ctx, query, id)

	return err
}

// CountSuperAdmins reports how many live super administrators the installation
// has. Used at boot to decide whether the first one needs seeding.
//
// Reads `nodes` rather than joining through auth_accounts on purpose: the
// question is whether the INSTALLATION has an owner, and a super-administrator
// node whose account row is missing still means somebody owns this site — the
// seed must not hand it to a stranger because the join came up empty.
//
// Disabled and soft-deleted rows are excluded. Both are states an
// administrator can be put into and taken out of, but neither can currently
// sign in, and an installation whose only super administrator is disabled has
// no one who can re-enable them.
func (repo *Users) CountSuperAdmins() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// Narrowed to PERSON nodes, not merely to the person_type value.
	//
	// person_type is a column on `nodes`, and every node has one — an article,
	// a photo, a hashtag. Nothing stops a stray write putting a value in one,
	// and an UPDATE narrowed on `alias` did exactly that during development: a
	// hashtag that happened to share a person's handle was counted here as a
	// super administrator.
	//
	// It matters because this is the guard deciding whether an installation
	// already has an owner. A false positive means the seed silently declines
	// to create the first administrator, leaving a site nobody can administer
	// and no message explaining why.
	query := `
		SELECT COUNT(*)
		FROM nodes
		WHERE type = ?
		  AND person_type = ?
		  AND deleted_at IS NULL
		  AND enabled = 1`

	var count int
	if err := repo.db.QueryRowContext(
		ctx, query,
		constants.NodeActorPersonEntityType,
		constants.PersonTypeSuperAdmin,
	).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// InstanceUsage is the population of this installation, for NodeInfo.
type InstanceUsage struct {
	Total          int
	ActiveMonth    int
	ActiveHalfYear int
	LocalPosts     int
}

// CountInstanceUsage totals what NodeInfo publishes about this installation.
//
// One statement rather than four round trips, because this is answered for any
// fediverse server that asks and there is no reason to make it four queries.
//
// The "active" figures come from nodes.last_visited_at, which is only
// meaningful because that column is now written on sign-in. Before that it
// held the date the account was created, so any active-user figure derived
// from it would have been a count of recent SIGNUPS reported to the wider
// network as engagement. Worth knowing if the numbers look wrong for the first
// six months after this ships: they are honest, and they start low because the
// column only began telling the truth recently.
//
// Deleted and disabled people are excluded from every figure. A count of
// members should mean people who could sign in today.
func (repo *Users) CountInstanceUsage() (*InstanceUsage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		SELECT
			COUNT(*),
			SUM(last_visited_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)),
			SUM(last_visited_at >= DATE_SUB(NOW(), INTERVAL 180 DAY))
		FROM nodes
		WHERE type = ?
		  AND deleted_at IS NULL
		  AND enabled = 1`

	usage := &InstanceUsage{}

	// SUM over no rows is NULL, and SUM over rows whose last_visited_at is all
	// NULL is NULL too — a brand-new installation, and one where nobody has
	// signed in since the column started being written.
	var activeMonth, activeHalfYear sql.NullInt64

	if err := repo.db.QueryRowContext(
		ctx, query, constants.NodeActorPersonEntityType,
	).Scan(&usage.Total, &activeMonth, &activeHalfYear); err != nil {
		return nil, err
	}

	usage.ActiveMonth = int(activeMonth.Int64)
	usage.ActiveHalfYear = int(activeHalfYear.Int64)

	// Posts are media nodes — notes, articles, photos. Counted separately
	// because it narrows on a prefix rather than an exact type and cannot
	// share the statement above.
	//
	// Only what was written here: a post kept from another server has a
	// uri, and is that server's to count. People need no such line, since
	// somebody on another server is not of the person type.
	postQuery := `
		SELECT COUNT(*)
		FROM nodes
		WHERE type LIKE CONCAT(?, '%')
		  AND uri IS NULL
		  AND deleted_at IS NULL`

	if err := repo.db.QueryRowContext(
		ctx, postQuery, constants.NodeMediumPrefixEntityType,
	).Scan(&usage.LocalPosts); err != nil {
		return nil, err
	}

	return usage, nil
}

// EditEmail updates a user's email
func (repo *Users) EditEmail(email string, id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE auth_accounts
		SET
			email = ?
		WHERE
			node_id = ?`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		email,
		id,
	)

	if err != nil {
		return err
	}

	return nil
}

// Restore is gone from this repository.
//
// It cleared nodes.deleted_at and set nodes.enabled with an UPDATE from here.
// That is graph's lifecycle: the same transition there writes the
// node_deletion_logs entry and publishes the restore event, so doing it from
// this side brought an account back with no audit trail and nothing notified.
//
// oauth_restore.go calls ActorsGRPC.Restore instead. Removed rather than left
// as a stub that always errors — an interface method nothing can call is a
// trap for the next person to reach for it.

// EditUsername is gone.
//
// The handle is nodes.alias and nothing else: auth_accounts has no username
// column, because the credential half never needed one. username_change.go
// moves the alias through ActorsGRPC, which owns the actor namespace and is
// the only thing that can refuse a handle already taken.
//
// Removed rather than left as a no-op — a method that writes nothing while
// looking like it writes something is worse than no method at all.

// Delete removes a person's CREDENTIALS row, and deliberately not their node.
//
// Called by the sweeper's purge, once the grace period has expired. Until then
// this row has to stay: it is what the person signs in with to restore
// themselves, and deleting it early would make the grace period unreachable.
//
// THE NODES ROW SURVIVES, as a tombstone. This used to delete both in one
// transaction, which released the handle — and a released handle is one whose
// next owner inherits the previous person's followers and mentions on every
// ActivityPub instance that cached them. graph-grpc's TombstoneActor strips
// that row of everything except its identity; what is left is the reservation.
//
// It also keeps `created_by` and `modified_by` across the database pointing
// at something, which matters in a schema with no foreign
// keys to tidy up behind a missing row.
func (repo *Users) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `DELETE FROM auth_accounts WHERE node_id = ?`
	if _, err := repo.db.ExecContext(ctx, query, id); err != nil {
		return err
	}

	return nil
}

// afterRead is a callback that is called after reading a user
func (repo *Users) afterRead(user *entities.User) error {
	user.GetAvatars()

	return nil
}

// IsEmail checks if an email is already taken by another user.
func (repo *Users) IsEmail(email string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// LOWER on both sides rather than trusting the column's collation.
	// A database created with a binary collation would otherwise let two
	// accounts hold the same address in different cases, and the caller
	// lowercases its candidate while stored addresses keep whatever case
	// they were registered with.
	//
	// The unique index on email is not a substitute: it has the same
	// collation dependency, and it would surface a collision as a 500
	// on a page the person cannot act on rather than as the 409 this
	// check produces.
	//
	// Costs the index on email, which is fine — this runs twice per
	// email change and nowhere else.
	query := `
		SELECT COUNT(*)
		FROM auth_accounts
		WHERE LOWER(email) = LOWER(?)`

	var count int
	err := repo.db.QueryRowContext(ctx, query, email).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// IsUsername checks whether a handle is already taken.
//
// The handle lives on nodes.alias now, and the namespace is every actor
// rather than every person: WebFinger resolves acct:name@domain to one
// actor with no type dimension, so a group holding the handle makes it
// unavailable to a person too. That is the same scope as the
// uq_unique_alias index, which is the actual enforcement — this check
// exists to turn a collision into a 409 the person can act on instead of
// a duplicate-key 500.
//
// Tombstoned rows count. Their whole purpose is to keep the handle
// reserved so a later account cannot inherit the previous person's
// followers on every instance that cached them, so there is no
// deleted_at filter here.
//
// Case-insensitive for the same reason IsEmail is, and with more at
// stake: two handles differing only in case are the same handle to a
// reader, which is the whole mechanism of an impersonation.
//
// Note this counts the caller's own row too. The change handler returns
// early when the requested handle is the one already held, so a person
// never collides with themselves here.
func (repo *Users) IsUsername(username string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		SELECT COUNT(*)
		FROM nodes
		WHERE LOWER(alias) = LOWER(?)
		  AND type LIKE 'node.actor.%'`

	var count int
	err := repo.db.QueryRowContext(ctx, query, username).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
