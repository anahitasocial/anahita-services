package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	an_const "anahita-libs/constants"

	"github.com/go-sql-driver/mysql"
)

// Imports: a person's lists brought in from a file (anahita-web roadmap row
// 124), and each line of it.

// The states an import goes through.
const (
	ImportReady     = "ready"
	ImportRunning   = "running"
	ImportDone      = "done"
	ImportCancelled = "cancelled"
)

// The kinds of line.
const (
	KindFollow = "follow"
	KindBlock  = "block"
	KindMute   = "mute"
	KindSave   = "save"
)

// Kinds are every kind of line, in the order they are shown.
var Kinds = []string{KindFollow, KindBlock, KindMute, KindSave}

// The states a line goes through.
const (
	LinePending = "pending"
	LineWorking = "working"
	LineDone    = "done"
	LineAlready = "already"
	LineSkipped = "skipped"
	LineFailed  = "failed"
)

// Import is one row of data_imports.
type Import struct {
	ID         string
	PersonID   int64
	Status     string
	Source     string
	FileName   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// Line is one row of data_import_lines.
type Line struct {
	ID                int64
	ImportID          string
	Kind              string
	Address           string
	URI               string
	Name              string
	URL               string
	HideNotifications bool
	EndsAt            *time.Time
	Status            string
	Reason            string
}

// Count is how many lines of one kind are in one state.
type Count struct {
	Kind   string
	Status string
	N      int
}

// ErrImportActive is a second import while one is waiting or running.
var ErrImportActive = errors.New("an import is already waiting or running")

// ErrImportNotInState is a change refused because the import has moved on.
var ErrImportNotInState = errors.New("the import is no longer in that state")

const importColumns = `
  id,
  person_id,
  status,
  source,
  file_name,
  created_at,
  updated_at,
  started_at,
  finished_at`

const lineColumns = `
  id,
  import_id,
  kind,
  address,
  uri,
  name,
  url,
  hide_notifications,
  ends_at,
  status,
  reason`

const importAdd = `
INSERT INTO data_imports (id, person_id, status, source, file_name, created_at, updated_at)
VALUES (?, ?, 'ready', ?, ?, ?, ?)`

const importRead = `
SELECT` + importColumns + `
FROM data_imports
WHERE id = ?`

const importsOfPerson = `
SELECT` + importColumns + `
FROM data_imports
WHERE person_id = ?
ORDER BY created_at DESC
LIMIT 5`

const importCounts = `
SELECT kind, status, COUNT(*)
FROM data_import_lines
WHERE import_id = ?
GROUP BY kind, status`

const importStart = `
UPDATE data_imports
SET status = 'running', started_at = ?, updated_at = ?
WHERE id = ?
  AND status = 'ready'`

const linesSkipUnchosen = `
UPDATE data_import_lines
SET status = 'skipped', touched_at = ?
WHERE import_id = ?
  AND status = 'pending'
  AND kind NOT IN (%s)`

const importCancel = `
UPDATE data_imports
SET status = 'cancelled', finished_at = ?, updated_at = ?
WHERE id = ?
  AND status IN ('ready', 'running')`

const linesSkipLeft = `
UPDATE data_import_lines
SET status = 'skipped', touched_at = ?
WHERE import_id = ?
  AND status IN ('pending', 'working')`

// The next line of the import that has waited longest since the worker
// last took a line of it: several people's imports go forward together,
// a line each in turn.
const lineNext = `
SELECT l.id
FROM data_imports AS i
JOIN data_import_lines AS l
  ON l.import_id = i.id
 AND l.status = 'pending'
WHERE i.status = 'running'
ORDER BY i.claimed_at IS NOT NULL, i.claimed_at, l.id
LIMIT 1
FOR UPDATE SKIP LOCKED`

const lineClaim = `
UPDATE data_import_lines
SET status = 'working', touched_at = ?
WHERE id = ?`

const importTouch = `
UPDATE data_imports
SET claimed_at = ?, updated_at = ?
WHERE id = ?`

const lineRead = `
SELECT` + lineColumns + `
FROM data_import_lines
WHERE id = ?`

const lineFinish = `
UPDATE data_import_lines
SET status = ?, reason = ?, touched_at = ?
WHERE id = ?
  AND status = 'working'`

const linesLeftBehind = `
UPDATE data_import_lines
SET status = 'pending'
WHERE status = 'working'
  AND touched_at < ?`

const importsFinished = `
UPDATE data_imports AS i
SET i.status = 'done', i.finished_at = ?, i.updated_at = ?
WHERE i.status = 'running'
  AND NOT EXISTS (
    SELECT 1
    FROM data_import_lines AS l
    WHERE l.import_id = i.id
      AND l.status IN ('pending', 'working')
  )`

const linesFailed = `
SELECT` + lineColumns + `
FROM data_import_lines
WHERE import_id = ?
  AND status = 'failed'
ORDER BY id`

// ImportsRepo keeps the imports.
type ImportsRepo struct {
	db      *sql.DB
	timeout time.Duration
	now     func() time.Time
	// How long a line may be 'working' before it is taken again.
	WorkFor time.Duration
}

func NewImports(db *sql.DB) *ImportsRepo {
	return &ImportsRepo{
		db:      db,
		timeout: an_const.DBTimeout,
		now:     func() time.Time { return time.Now().UTC() },
		WorkFor: 10 * time.Minute,
	}
}

func (r *ImportsRepo) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.timeout)
}

// Add keeps an import read from its file, with its lines, 'ready' for the
// person to start. ErrImportActive when they have one waiting or running.
func (r *ImportsRepo) Add(imp *Import, lines []*Line) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := r.now()
	imp.Status = ImportReady
	imp.CreatedAt, imp.UpdatedAt = now, now
	_, err = tx.ExecContext(ctx, importAdd, imp.ID, imp.PersonID, imp.Source, imp.FileName, now, now)
	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return ErrImportActive
	}
	if err != nil {
		return err
	}

	// Some hundreds at a time: a file may have 20,000 lines.
	const batch = 500
	for start := 0; start < len(lines); start += batch {
		end := min(start+batch, len(lines))
		query := `
INSERT INTO data_import_lines
  (import_id, kind, address, uri, name, url, hide_notifications, ends_at, status)
VALUES ` + strings.TrimSuffix(strings.Repeat("(?, ?, ?, ?, ?, ?, ?, ?, 'pending'),", end-start), ",")
		args := make([]any, 0, 8*(end-start))
		for _, line := range lines[start:end] {
			args = append(args, imp.ID, line.Kind, line.Address, line.URI, line.Name, line.URL,
				line.HideNotifications, line.EndsAt)
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type importScanner interface {
	Scan(dest ...any) error
}

func scanImport(row importScanner) (*Import, error) {
	i := &Import{}
	var started, finished sql.NullTime
	err := row.Scan(&i.ID, &i.PersonID, &i.Status, &i.Source, &i.FileName,
		&i.CreatedAt, &i.UpdatedAt, &started, &finished)
	if err != nil {
		return nil, err
	}
	if started.Valid {
		i.StartedAt = &started.Time
	}
	if finished.Valid {
		i.FinishedAt = &finished.Time
	}
	return i, nil
}

func scanLine(row importScanner) (*Line, error) {
	l := &Line{}
	var ends sql.NullTime
	err := row.Scan(&l.ID, &l.ImportID, &l.Kind, &l.Address, &l.URI, &l.Name, &l.URL,
		&l.HideNotifications, &ends, &l.Status, &l.Reason)
	if err != nil {
		return nil, err
	}
	if ends.Valid {
		l.EndsAt = &ends.Time
	}
	return l, nil
}

// Read is one import; nil when there is none.
func (r *ImportsRepo) Read(id string) (*Import, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	imp, err := scanImport(r.db.QueryRowContext(ctx, importRead, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return imp, err
}

// OfPerson is a person's latest imports, newest first.
func (r *ImportsRepo) OfPerson(personID int64) ([]*Import, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	rows, err := r.db.QueryContext(ctx, importsOfPerson, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	imports := []*Import{}
	for rows.Next() {
		imp, err := scanImport(rows)
		if err != nil {
			return nil, err
		}
		imports = append(imports, imp)
	}
	return imports, rows.Err()
}

// Counts is how many of an import's lines are of each kind, in each state.
func (r *ImportsRepo) Counts(id string) ([]Count, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	rows, err := r.db.QueryContext(ctx, importCounts, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := []Count{}
	for rows.Next() {
		var c Count
		if err := rows.Scan(&c.Kind, &c.Status, &c.N); err != nil {
			return nil, err
		}
		counts = append(counts, c)
	}
	return counts, rows.Err()
}

// Start sets an import going with the kinds the person chose; the lines of
// the others are skipped.
func (r *ImportsRepo) Start(id string, kinds []string) error {
	ctx, cancel := r.ctx()
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := r.now()
	result, err := tx.ExecContext(ctx, importStart, now, now, id)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return ErrImportNotInState
	}

	args := []any{now, id}
	marks := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		marks = append(marks, "?")
		args = append(args, kind)
	}
	if len(marks) == 0 {
		marks, args = []string{"''"}, []any{now, id}
	}
	query := strings.Replace(linesSkipUnchosen, "%s", strings.Join(marks, ", "), 1)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}

	return tx.Commit()
}

// Cancel gives an import up; what it has not done is skipped.
func (r *ImportsRepo) Cancel(id string) error {
	ctx, cancel := r.ctx()
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := r.now()
	result, err := tx.ExecContext(ctx, importCancel, now, now, id)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return ErrImportNotInState
	}
	if _, err := tx.ExecContext(ctx, linesSkipLeft, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

// NextLine takes the next line to do, 'working' until Finish; nil when no
// import has one waiting. A line left 'working' by a worker that stopped is
// taken again.
func (r *ImportsRepo) NextLine() (*Line, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	now := r.now()
	if _, err := r.db.ExecContext(ctx, linesLeftBehind, now.Add(-r.WorkFor)); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int64
	err = tx.QueryRowContext(ctx, lineNext).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, lineClaim, now, id); err != nil {
		return nil, err
	}
	line, err := scanLine(tx.QueryRowContext(ctx, lineRead, id))
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, importTouch, now, now, line.ImportID); err != nil {
		return nil, err
	}
	return line, tx.Commit()
}

// Finish says what came of a line: done, already, or failed and why.
func (r *ImportsRepo) Finish(lineID int64, status, reason string) error {
	ctx, cancel := r.ctx()
	defer cancel()

	_, err := r.db.ExecContext(ctx, lineFinish, status, reason, r.now(), lineID)
	return err
}

// FinishImports marks done every running import with nothing left to do.
func (r *ImportsRepo) FinishImports() error {
	ctx, cancel := r.ctx()
	defer cancel()

	now := r.now()
	_, err := r.db.ExecContext(ctx, importsFinished, now, now)
	return err
}

// Failed is the lines of an import that could not be done, for the person
// to see why.
func (r *ImportsRepo) Failed(id string) ([]*Line, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	rows, err := r.db.QueryContext(ctx, linesFailed, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lines := []*Line{}
	for rows.Next() {
		line, err := scanLine(rows)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

// ImportsInterface is what the handlers and the worker need of the imports.
type ImportsInterface interface {
	Add(imp *Import, lines []*Line) error
	Read(id string) (*Import, error)
	OfPerson(personID int64) ([]*Import, error)
	Counts(id string) ([]Count, error)
	Start(id string, kinds []string) error
	Cancel(id string) error
	NextLine() (*Line, error)
	Finish(lineID int64, status, reason string) error
	FinishImports() error
	Failed(id string) ([]*Line, error)
}
