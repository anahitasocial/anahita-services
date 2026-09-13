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
	Delete(id int64) error
	IsEmail(email string) (bool, error)
	IsUsername(username string) (bool, error)
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
	// "ORDER BY created_on, created_on DESC".
	//
	// Checked with safeOrderColumn as well as by the request's oneof tag,
	// because ORDER BY is concatenated rather than bound. See ordering.go.
	//
	// Qualified with the table, because every one of the four sortable
	// columns lives on `nodes` now and this select joins two tables. A bare
	// `person_type` or `created_on` is an unknown column here, and the read
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

	// Only the credential half. The node already exists — the caller created
	// it through ActorsGRPC — and carries the name, the handle and the
	// authority level, none of which are copied here any more.
	query := `
		INSERT INTO auth_accounts (
			node_id,
			email,
			password
		) VALUES (
			?,
			?,
			?
		)`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email,
		user.Password,
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
	// The join existed to write nodes.enabled and nodes.modified_on alongside
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
// It also keeps `created_by`, `modified_by` and `last_comment_by` across the
// database pointing at something, which matters in a schema with no foreign
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
