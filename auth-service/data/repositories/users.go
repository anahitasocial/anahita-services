package repositories

import (
	"anahita-libs/constants"
	"auth/data/entities"
	"auth/requests"
	"context"
	"database/sql"
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
	Delete(id int64) error
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

	query := sq.Select(
		"id",
		"type",
		"enabled",
		"given_name",
		"family_name",
		"username",
		"email",
		"usertype",
		"service",
		"filename",
		"last_visit_date",
		"created_on",
		"modified_on",
	).From(
		"people_people",
	).LeftJoin(
		"nodes ON people_people.node_id = nodes.id",
	)

	if req.Disabled {
		query = query.Where(sq.Eq{"enabled": 0})
	} else {
		query = query.Where(sq.Eq{"enabled": 1})
	}

	if req.Keywords != "" {
		query = query.Where(sq.Or{
			sq.Like{"username": "%" + req.Keywords + "%"},
			sq.Like{"email": "%" + req.Keywords + "%"},
		})
	}

	if req.UserType != "" {
		query = query.Where(sq.Eq{"usertype": req.UserType})
	}

	if req.Ordering != "" {
		query = query.OrderBy(req.Ordering)
	}

	if req.Ordering != "" {
		if req.Direction == "asc" {
			query = query.OrderBy(req.Ordering + " ASC")
		} else {
			query = query.OrderBy(req.Ordering + " DESC")
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
		item := entities.NewUser()
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Enabled,
			&item.GivenName,
			&item.FamilyName,
			&item.Username,
			&item.Email,
			&item.UserType,
			&item.Service,
			&item.Filename,
			&item.LastVisitDate,
			&item.CreationTime,
			&item.UpdateTime,
		)

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

	query := sq.Select(
		"id",
		"type",
		"enabled",
		"given_name",
		"family_name",
		"username",
		"email",
		"password",
		"usertype",
		"service",
		"filename",
		"last_visit_date",
		"created_on",
		"modified_on",
		"totp_enabled",
		"COALESCE(totp_recovery_codes, '') AS totp_recovery_codes",
		"COALESCE(totp_last_verified_at, NOW()) AS totp_last_verified_at",
	).From(
		"people_people",
	).LeftJoin(
		"nodes ON people_people.node_id = nodes.id",
	).Where(
		sq.Eq{"node_id": id},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewUser()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	err = row.Scan(
		&item.ID,
		&item.Type,
		&item.Enabled,
		&item.GivenName,
		&item.FamilyName,
		&item.Username,
		&item.Email,
		&item.Password,
		&item.UserType,
		&item.Service,
		&item.Filename,
		&item.LastVisitDate,
		&item.CreationTime,
		&item.UpdateTime,
		&item.TotpEnabled,
		&item.TotpRecoveryCodes,
		&item.TotpLastVerifiedAt,
	)

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

	query := sq.Select(
		"id",
		"type",
		"enabled",
		"given_name",
		"family_name",
		"username",
		"email",
		"password",
		"usertype",
		"service",
		"filename",
		"last_visit_date",
		"created_on",
		"modified_on",
		"totp_enabled",
		"COALESCE(totp_recovery_codes, '') AS totp_recovery_codes",
		"COALESCE(totp_last_verified_at, NOW()) AS totp_last_verified_at",
	).From(
		"people_people",
	).LeftJoin(
		"nodes ON people_people.node_id = nodes.id",
	).Where(
		sq.Eq{"email": email},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewUser()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	err = row.Scan(
		&item.ID,
		&item.Type,
		&item.Enabled,
		&item.GivenName,
		&item.FamilyName,
		&item.Username,
		&item.Email,
		&item.Password,
		&item.UserType,
		&item.Service,
		&item.Filename,
		&item.LastVisitDate,
		&item.CreationTime,
		&item.UpdateTime,
		&item.TotpEnabled,
		&item.TotpRecoveryCodes,
		&item.TotpLastVerifiedAt,
	)

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

	query := sq.Select(
		"id",
		"type",
		"enabled",
		"given_name",
		"family_name",
		"username",
		"email",
		"password",
		"usertype",
		"service",
		"filename",
		"last_visit_date",
		"created_on",
		"modified_on",
		"totp_enabled",
		"COALESCE(totp_recovery_codes, '') AS totp_recovery_codes",
		"COALESCE(totp_last_verified_at, NOW()) AS totp_last_verified_at",
	).From(
		"people_people",
	).LeftJoin(
		"nodes ON people_people.node_id = nodes.id",
	).Where(
		sq.Eq{"username": username},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewUser()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	err = row.Scan(
		&item.ID,
		&item.Type,
		&item.Enabled,
		&item.GivenName,
		&item.FamilyName,
		&item.Username,
		&item.Email,
		&item.Password,
		&item.UserType,
		&item.Service,
		&item.Filename,
		&item.LastVisitDate,
		&item.CreationTime,
		&item.UpdateTime,
		&item.TotpEnabled,
		&item.TotpRecoveryCodes,
		&item.TotpLastVerifiedAt,
	)

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

	query := sq.Select(
		"id",
		"type",
		"enabled",
		"given_name",
		"family_name",
		"username",
		"email",
		"password",
		"usertype",
		"service",
		"filename",
		"last_visit_date",
		"created_on",
		"modified_on",
		"totp_enabled",
		"COALESCE(totp_recovery_codes, '') AS totp_recovery_codes",
		"COALESCE(totp_last_verified_at, NOW()) AS totp_last_verified_at",
	).From(
		"people_people",
	).LeftJoin(
		"nodes ON people_people.node_id = nodes.id",
	).Where(
		sq.Or{
			sq.Eq{"email": emailOrUsername},
			sq.Eq{"username": emailOrUsername},
		},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewUser()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	err = row.Scan(
		&item.ID,
		&item.Type,
		&item.Enabled,
		&item.GivenName,
		&item.FamilyName,
		&item.Username,
		&item.Email,
		&item.Password,
		&item.UserType,
		&item.Service,
		&item.Filename,
		&item.LastVisitDate,
		&item.CreationTime,
		&item.UpdateTime,
		&item.TotpEnabled,
		&item.TotpRecoveryCodes,
		&item.TotpLastVerifiedAt,
	)

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

	user.Type = constants.NodeActorPersonEntityType
	user.Service = constants.PersonService

	// Get a Tx for making transaction requests.
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO nodes (
			type,
			service,
			alias,
			enabled,
			created_on,
			modified_on
		) VALUES (
			?,
			?,
			?,
			?,
			?,
			?
		)`

	result, err := tx.ExecContext(
		ctx,
		query,
		user.Type,
		user.Service,
		user.Username,
		user.Enabled,
		user.CreationTime,
		user.UpdateTime,
	)
	if err != nil {
		return nil, err
	}

	user.ID, err = result.LastInsertId()
	if err != nil {
		return nil, err
	}

	query = `
		INSERT INTO people_people (
			node_id,
			given_name,
			family_name,
			username,
			email,
			password,
			usertype,
			last_visit_date
		) VALUES (
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?
		)`

	_, err = tx.ExecContext(
		ctx,
		query,
		user.ID,
		user.GivenName,
		user.FamilyName,
		user.Username,
		user.Email,
		user.Password,
		user.UserType,
		user.LastVisitDate,
	)
	if err != nil {
		return nil, err
	}

	// Commit the transaction.
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return repo.ReadByID(user.ID)
}

// Edit updates a user
func (repo *Users) Edit(user *entities.User) (*entities.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE nodes
		INNER JOIN people_people ON people_people.node_id = nodes.id
		SET
			alias = ?,
			enabled = ?,
			usertype = ?,
			email = ?,
			username = ?,
			password = ?,
			modified_on = ?,
			totp_enabled = ?,
			totp_recovery_codes = ?,
			totp_last_verified_at = ?
		WHERE
			nodes.id = ? AND
			people_people.node_id = ?`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		user.Username,
		user.Enabled,
		user.UserType,
		user.Email,
		user.Username,
		user.Password,
		user.UpdateTime,
		user.TotpEnabled,
		user.TotpRecoveryCodes,
		user.TotpLastVerifiedAt,
		user.ID,
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
		UPDATE people_people
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

// Delete deletes a user
func (repo *Users) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// Get a Tx for making transaction requests.
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `DELETE FROM people_people WHERE node_id = ?`
	_, err = tx.ExecContext(ctx, query, id)

	if err != nil {
		return err
	}

	query = `DELETE FROM nodes WHERE id = ?`
	_, err = tx.ExecContext(ctx, query, id)

	if err != nil {
		return err
	}

	// Commit the transaction.
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

// afterRead is a callback that is called after reading a user
func (repo *Users) afterRead(user *entities.User) error {
	user.GetAvatars()

	return nil
}
