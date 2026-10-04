package repositories

import (
	"context"
	"database/sql"
	"time"

	"auth/data/entities"
)

// SignupRequestsInterface is the approval queue.
type SignupRequestsInterface interface {
	Add(sr *entities.SignupRequest) (*entities.SignupRequest, error)
	ReadByID(id int64) (*entities.SignupRequest, error)
	ReadByEmail(email string) (*entities.SignupRequest, error)
	BrowsePending(limit, offset uint64) ([]*entities.SignupRequest, error)
	CountPending() (int, error)
	MarkEmailVerified(id int64) error
	Decide(id int64, status string, decidedBy int64, note string) error
	Delete(id int64) error

	// DeleteFor removes every request made with this address or this
	// username, returning how many rows went. For account purge: a request
	// row holds the address and a password hash, and is kept after it is
	// decided.
	DeleteFor(email, username string) (int64, error)
}

func NewSignupRequests(db *sql.DB) SignupRequestsInterface {
	return &signupRequests{db: db}
}

type signupRequests struct {
	db *sql.DB
}

// signupRequestColumns is the select list and scan order, in one place.
//
// Same discipline as userColumns: this file keeps a column list and a
// positional Scan in step, and a column added to one and not the other fails
// every read at runtime rather than at build time.
func signupRequestColumns() string {
	return `id, username, email, password,
		COALESCE(name, ''), COALESCE(body, ''), COALESCE(website_url, ''),
		status, email_verified_at,
		COALESCE(invited_by, 0), COALESCE(decided_by, 0), decided_at,
		COALESCE(decision_note, ''),
		COALESCE(created_ip, ''), COALESCE(created_user_agent, ''),
		created_at, updated_at`
}

func scanSignupRequest(row rowScanner) (*entities.SignupRequest, error) {
	sr := entities.NewSignupRequest()

	var verifiedAt, decidedAt sql.NullTime

	if err := row.Scan(
		&sr.ID, &sr.Username, &sr.Email, &sr.Password,
		&sr.Name, &sr.Body, &sr.WebsiteURL,
		&sr.Status, &verifiedAt,
		&sr.InvitedBy, &sr.DecidedBy, &decidedAt,
		&sr.DecisionNote,
		&sr.CreatedIP, &sr.CreatedUserAgent,
		&sr.CreatedAt, &sr.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if verifiedAt.Valid {
		t := verifiedAt.Time
		sr.EmailVerifiedAt = &t
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		sr.DecidedAt = &t
	}

	return sr, nil
}

func (repo *signupRequests) Add(
	sr *entities.SignupRequest,
) (*entities.SignupRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now()

	query := `
		INSERT INTO signup_requests (
			username, email, password, name, body, website_url,
			status, invited_by, created_ip, created_user_agent,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := repo.db.ExecContext(ctx, query,
		sr.Username, sr.Email, sr.Password,
		sr.Name, sr.Body, sr.WebsiteURL,
		sr.Status, nullableID(sr.InvitedBy),
		sr.CreatedIP, sr.CreatedUserAgent,
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

func (repo *signupRequests) ReadByID(id int64) (*entities.SignupRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `SELECT ` + signupRequestColumns() + ` FROM signup_requests WHERE id = ?`

	return scanSignupRequest(repo.db.QueryRowContext(ctx, query, id))
}

// ReadByEmail finds a request by address, whatever its status.
//
// Every status, deliberately. Signup uses this to refuse a second request from
// the same address, and a rejected one still counts — otherwise a refusal is a
// thing somebody re-submits their way past.
func (repo *signupRequests) ReadByEmail(email string) (*entities.SignupRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `SELECT ` + signupRequestColumns() +
		` FROM signup_requests WHERE LOWER(email) = LOWER(?)`

	return scanSignupRequest(repo.db.QueryRowContext(ctx, query, email))
}

// BrowsePending lists the queue, oldest first.
//
// Oldest first, unlike every other listing in this service. A queue is worked
// through, and newest-first means the people who have waited longest are the
// ones nobody reaches.
func (repo *signupRequests) BrowsePending(
	limit, offset uint64,
) ([]*entities.SignupRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if limit == 0 {
		limit = 50
	}

	query := `SELECT ` + signupRequestColumns() + `
		FROM signup_requests
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?`

	rows, err := repo.db.QueryContext(
		ctx, query, entities.SignupRequestStatusPending, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.SignupRequest
	for rows.Next() {
		sr, err := scanSignupRequest(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, sr)
	}

	return results, rows.Err()
}

// CountPending is how many people are waiting.
//
// `pending` only: an unverified request is not in the queue, and counting it
// would tell administrators there is work that they cannot see and must not
// act on.
func (repo *signupRequests) CountPending() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `SELECT COUNT(*) FROM signup_requests WHERE status = ?`

	var count int
	err := repo.db.QueryRowContext(
		ctx, query, entities.SignupRequestStatusPending).Scan(&count)

	return count, err
}

// MarkEmailVerified moves a request into the queue.
//
// The two happen together on purpose — confirming the address IS what puts
// somebody in front of an administrator, and a version that stamped the date
// without moving the status would leave requests verified and invisible.
//
// Narrowed to `unverified`, so following the link twice does not drag a
// decided request back into the queue.
func (repo *signupRequests) MarkEmailVerified(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE signup_requests
		SET email_verified_at = NOW(), status = ?, updated_at = NOW()
		WHERE id = ? AND status = ?`

	_, err := repo.db.ExecContext(ctx, query,
		entities.SignupRequestStatusPending, id, entities.SignupRequestStatusUnverified)

	return err
}

// Decide records an approval or a rejection.
//
// Narrowed to `pending`, which is what makes a decision final: two
// administrators clearing the queue at once cannot both succeed, and the
// second gets zero rows rather than overwriting the first one's answer.
func (repo *signupRequests) Decide(
	id int64, status string, decidedBy int64, note string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		UPDATE signup_requests
		SET status = ?, decided_by = ?, decided_at = NOW(),
		    decision_note = ?, updated_at = NOW()
		WHERE id = ? AND status = ?`

	result, err := repo.db.ExecContext(ctx, query,
		status, decidedBy, note, id, entities.SignupRequestStatusPending)
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

func (repo *signupRequests) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx,
		`DELETE FROM signup_requests WHERE id = ?`, id)

	return err
}

func (repo *signupRequests) DeleteFor(email, username string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// An empty value must match nothing, not every row that has none.
	const query = `
DELETE FROM signup_requests
WHERE (? <> '' AND email = ?)
   OR (? <> '' AND username = ?)`

	result, err := repo.db.ExecContext(ctx, query, email, email, username, username)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// nullableID writes a zero id as NULL, so "nobody" is absent rather than
// pointing at actor 0.
func nullableID(id int64) any {
	if id == 0 {
		return nil
	}

	return id
}
