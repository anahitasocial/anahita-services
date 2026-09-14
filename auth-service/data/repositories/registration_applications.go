package repositories

import (
	"context"
	"database/sql"
	"time"

	"auth/data/entities"
)

// RegistrationApplicationsInterface is the approval queue.
type RegistrationApplicationsInterface interface {
	Add(app *entities.RegistrationApplication) (*entities.RegistrationApplication, error)
	ReadByID(id int64) (*entities.RegistrationApplication, error)
	ReadByEmail(email string) (*entities.RegistrationApplication, error)
	BrowsePending(limit, offset uint64) ([]*entities.RegistrationApplication, error)
	CountPending() (int, error)
	MarkEmailVerified(id int64) error
	Decide(id int64, status string, decidedBy int64, note string) error
	Delete(id int64) error
}

func NewRegistrationApplications(db *sql.DB) RegistrationApplicationsInterface {
	return &registrationApplications{db: db}
}

type registrationApplications struct {
	db *sql.DB
}

// applicationColumns is the select list and scan order, in one place.
//
// Same discipline as userColumns: this file keeps a column list and a
// positional Scan in step, and a column added to one and not the other fails
// every read at runtime rather than at build time.
func applicationColumns() string {
	return `id, username, email, password,
		COALESCE(name, ''), COALESCE(body, ''), COALESCE(website_url, ''),
		status, email_verified_at,
		COALESCE(invited_by, 0), COALESCE(decided_by, 0), decided_at,
		COALESCE(decision_note, ''),
		COALESCE(created_ip, ''), COALESCE(created_user_agent, ''),
		created_at, updated_at`
}

func scanApplication(row rowScanner) (*entities.RegistrationApplication, error) {
	app := entities.NewRegistrationApplication()

	var verifiedAt, decidedAt sql.NullTime

	if err := row.Scan(
		&app.ID, &app.Username, &app.Email, &app.Password,
		&app.Name, &app.Body, &app.WebsiteURL,
		&app.Status, &verifiedAt,
		&app.InvitedBy, &app.DecidedBy, &decidedAt,
		&app.DecisionNote,
		&app.CreatedIP, &app.CreatedUserAgent,
		&app.CreatedAt, &app.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if verifiedAt.Valid {
		t := verifiedAt.Time
		app.EmailVerifiedAt = &t
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		app.DecidedAt = &t
	}

	return app, nil
}

func (repo *registrationApplications) Add(
	app *entities.RegistrationApplication,
) (*entities.RegistrationApplication, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now()

	query := `
		INSERT INTO registration_applications (
			username, email, password, name, body, website_url,
			status, invited_by, created_ip, created_user_agent,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := repo.db.ExecContext(ctx, query,
		app.Username, app.Email, app.Password,
		app.Name, app.Body, app.WebsiteURL,
		app.Status, nullableID(app.InvitedBy),
		app.CreatedIP, app.CreatedUserAgent,
		now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return repo.ReadByID(id)
}

func (repo *registrationApplications) ReadByID(id int64) (*entities.RegistrationApplication, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `SELECT ` + applicationColumns() + ` FROM registration_applications WHERE id = ?`

	return scanApplication(repo.db.QueryRowContext(ctx, query, id))
}

// ReadByEmail finds an application by address, whatever its status.
//
// Every status, deliberately. Signup uses this to refuse a second application
// from the same address, and a rejected one still counts — otherwise a
// refusal is a thing somebody re-submits their way past.
func (repo *registrationApplications) ReadByEmail(email string) (*entities.RegistrationApplication, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `SELECT ` + applicationColumns() +
		` FROM registration_applications WHERE LOWER(email) = LOWER(?)`

	return scanApplication(repo.db.QueryRowContext(ctx, query, email))
}

// BrowsePending lists the queue, oldest first.
//
// Oldest first, unlike every other listing in this service. A queue is worked
// through, and newest-first means the people who have waited longest are the
// ones nobody reaches.
func (repo *registrationApplications) BrowsePending(
	limit, offset uint64,
) ([]*entities.RegistrationApplication, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if limit == 0 {
		limit = 50
	}

	query := `SELECT ` + applicationColumns() + `
		FROM registration_applications
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?`

	rows, err := repo.db.QueryContext(
		ctx, query, entities.ApplicationStatusPending, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.RegistrationApplication
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, app)
	}

	return results, rows.Err()
}

// CountPending is how many people are waiting.
//
// `pending` only: an unverified application is not in the queue, and counting
// it would tell administrators there is work that they cannot see and must not
// act on.
func (repo *registrationApplications) CountPending() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `SELECT COUNT(*) FROM registration_applications WHERE status = ?`

	var count int
	err := repo.db.QueryRowContext(
		ctx, query, entities.ApplicationStatusPending).Scan(&count)

	return count, err
}

// MarkEmailVerified moves an application into the queue.
//
// The two happen together on purpose — confirming the address IS what puts
// somebody in front of an administrator, and a version that stamped the date
// without moving the status would leave applications verified and invisible.
//
// Narrowed to `unverified`, so following the link twice does not drag a
// decided application back into the queue.
func (repo *registrationApplications) MarkEmailVerified(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE registration_applications
		SET email_verified_at = NOW(), status = ?, updated_at = NOW()
		WHERE id = ? AND status = ?`

	_, err := repo.db.ExecContext(ctx, query,
		entities.ApplicationStatusPending, id, entities.ApplicationStatusUnverified)

	return err
}

// Decide records an approval or a rejection.
//
// Narrowed to `pending`, which is what makes a decision final: two
// administrators clearing the queue at once cannot both succeed, and the
// second gets zero rows rather than overwriting the first one's answer.
func (repo *registrationApplications) Decide(
	id int64, status string, decidedBy int64, note string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE registration_applications
		SET status = ?, decided_by = ?, decided_at = NOW(),
		    decision_note = ?, updated_at = NOW()
		WHERE id = ? AND status = ?`

	result, err := repo.db.ExecContext(ctx, query,
		status, decidedBy, note, id, entities.ApplicationStatusPending)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (repo *registrationApplications) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx,
		`DELETE FROM registration_applications WHERE id = ?`, id)

	return err
}

// nullableID writes a zero id as NULL, so "nobody" is absent rather than
// pointing at actor 0.
func nullableID(id int64) any {
	if id == 0 {
		return nil
	}

	return id
}
