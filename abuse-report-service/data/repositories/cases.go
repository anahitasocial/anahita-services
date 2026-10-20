package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"abusereport/constants"
	"abusereport/data/entities"

	"github.com/go-sql-driver/mysql"
)

const dbTimeout = 5 * time.Second

// ErrOpenCaseExists is returned when reopening a case whose target already
// has another open one.
var ErrOpenCaseExists = errors.New("the target already has an open case")

// CasesInterface is the storage for cases and their reports.
type CasesInterface interface {
	// AddReport files one person's report against a target, opening a case
	// for it if none is open. A second report by the same person in the
	// same case replaces their first. `created` is false for that.
	AddReport(target entities.Case, report entities.Report) (caseID int64, created bool, err error)

	// Browse and Count take a status and, optionally, a reason: with one,
	// only cases in which somebody gave that reason. An empty reason means
	// every case of the status.
	Browse(status, reasonKey string, limit, offset uint64) ([]*entities.Case, error)
	Count(status, reasonKey string) (uint64, error)
	Read(id int64) (*entities.Case, error)
	BrowseReports(caseID int64) ([]*entities.Report, error)
	// BrowseForProfile is the cases with a report against a group's or an
	// event's rules, newest first, and CountForProfile how many; for one of
	// its administrators (roadmap row 102, part 6b). Never the profile
	// itself, nor what the viewer is answerable for: whoever is reported is
	// not told.
	BrowseForProfile(profileID, viewerID int64, status string, limit, offset uint64) ([]*entities.Case, error)
	CountForProfile(profileID, viewerID int64, status string) (uint64, error)
	// ProfileReports is those reports, of the given cases, oldest first.
	ProfileReports(profileID int64, caseIDs []int64) ([]*entities.Report, error)
	BrowseForTarget(targetID, exceptCaseID int64) ([]*entities.Case, error)
	ReasonCounts(caseIDs []int64) ([]entities.ReasonCount, error)

	// Resolve closes an open case, and reports whether this call did it.
	Resolve(id int64, status, action, note string, decidedBy int64) (bool, error)
	// Reopen opens a closed case again, and reports whether this call did.
	Reopen(id int64) (bool, error)

	// MarkTargetGone records that a target no longer exists, and closes its
	// open case as actioned. It returns how many cases were closed.
	MarkTargetGone(targetID int64) (int64, error)

	// RemoveReportsBy deletes every report one person filed, because that
	// person has been purged. Each case's count drops by one, and an open
	// case nobody else reported is removed with it: there is no complaint
	// left to decide. It returns how many reports went.
	RemoveReportsBy(reporterID int64) (int64, error)

	// ForgetAddresses blanks the network address on reports filed before
	// the cutoff, and returns how many it blanked.
	ForgetAddresses(before time.Time) (int64, error)
}

type cases struct {
	db *sql.DB
}

func NewCases(db *sql.DB) CasesInterface {
	return &cases{db: db}
}

// The queries, each written out in full so it can be read without running
// the code that uses it. One clause to a line, one column to a line.

// caseColumns is what scanCase reads, in its order. The COALESCEs read whose
// target it is, and who decided it, as 0 when there is nobody.
const caseColumns = `
	id,
	target_id,
	target_type,
	COALESCE(target_owner_id, 0),
	status,
	report_count,
	first_reported_at,
	last_reported_at,
	COALESCE(decided_by, 0),
	decided_at,
	decision_action,
	decision_note,
	target_gone_at`

const (
	// The open case for a target, locked until the transaction ends so that
	// the report about to be counted is counted on the right one.
	selectOpenCaseForUpdate = `
		SELECT id
		  FROM abuse_cases
		 WHERE open_target_id = ?
		   FOR UPDATE`

	// target_owner_id is NULL, not zero, for a target nobody answers for.
	insertCase = `
		INSERT INTO abuse_cases (
			target_id,
			target_type,
			target_owner_id,
			status,
			report_count,
			first_reported_at,
			last_reported_at
		) VALUES (?, ?, NULLIF(?, 0), ?, 0, NOW(), NOW())`

	// One report per person per case: a second one replaces the reason and
	// the details of the first.
	upsertReport = `
		INSERT INTO abuse_reports (
			case_id,
			reporter_id,
			reason_key,
			details,
			rule_title,
			rule_profile_id,
			created_at,
			updated_at,
			created_ip
		) VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW(), ?)
		ON DUPLICATE KEY UPDATE
			reason_key      = VALUES(reason_key),
			details         = VALUES(details),
			rule_title      = VALUES(rule_title),
			rule_profile_id = VALUES(rule_profile_id),
			updated_at      = NOW()`

	countNewReport = `
		UPDATE abuse_cases
		   SET report_count     = report_count + 1,
		       last_reported_at = NOW()
		 WHERE id = ?`

	// Cases of one status and, optionally, with at least one report that
	// gave one reason. The reason is bound twice: an empty one switches the
	// second condition off.
	selectCasesByStatus = `
		SELECT ` + caseColumns + `
		  FROM abuse_cases
		 WHERE status = ?
		   AND (? = '' OR EXISTS (
		           SELECT 1
		             FROM abuse_reports
		            WHERE abuse_reports.case_id = abuse_cases.id
		              AND abuse_reports.reason_key = ?
		       ))
		 ORDER BY last_reported_at DESC, id DESC
		 LIMIT ? OFFSET ?`

	countCasesByStatus = `
		SELECT COUNT(*)
		  FROM abuse_cases
		 WHERE status = ?
		   AND (? = '' OR EXISTS (
		           SELECT 1
		             FROM abuse_reports
		            WHERE abuse_reports.case_id = abuse_cases.id
		              AND abuse_reports.reason_key = ?
		       ))`

	// Cases of one status with a report against one profile's rules.
	selectCasesForProfile = `
		SELECT ` + caseColumns + `
		  FROM abuse_cases
		 WHERE status = ?
		   AND target_id <> ?
		   AND COALESCE(target_owner_id, 0) <> ?
		   AND EXISTS (
		           SELECT 1
		             FROM abuse_reports
		            WHERE abuse_reports.case_id = abuse_cases.id
		              AND abuse_reports.rule_profile_id = ?
		       )
		 ORDER BY last_reported_at DESC, id DESC
		 LIMIT ? OFFSET ?`

	countCasesForProfile = `
		SELECT COUNT(*)
		  FROM abuse_cases
		 WHERE status = ?
		   AND target_id <> ?
		   AND COALESCE(target_owner_id, 0) <> ?
		   AND EXISTS (
		           SELECT 1
		             FROM abuse_reports
		            WHERE abuse_reports.case_id = abuse_cases.id
		              AND abuse_reports.rule_profile_id = ?
		       )`

	// %s is one placeholder per case id.
	selectProfileReports = `
		SELECT id,
		       case_id,
		       reporter_id,
		       reason_key,
		       details,
		       rule_title,
		       rule_profile_id,
		       created_at,
		       updated_at,
		       created_ip
		  FROM abuse_reports
		 WHERE rule_profile_id = ?
		   AND case_id IN (%s)
		 ORDER BY created_at, id`

	selectCase = `
		SELECT ` + caseColumns + `
		  FROM abuse_cases
		 WHERE id = ?`

	selectReportsOfCase = `
		SELECT id,
		       case_id,
		       reporter_id,
		       reason_key,
		       details,
		       rule_title,
		       rule_profile_id,
		       created_at,
		       updated_at,
		       created_ip
		  FROM abuse_reports
		 WHERE case_id = ?
		 ORDER BY created_at, id`

	selectOtherCasesOfTarget = `
		SELECT ` + caseColumns + `
		  FROM abuse_cases
		 WHERE target_id = ?
		   AND id <> ?
		 ORDER BY first_reported_at DESC, id DESC`

	// %s is one placeholder per case id.
	selectReasonCounts = `
		SELECT case_id,
		       reason_key,
		       COUNT(*) AS reports
		  FROM abuse_reports
		 WHERE case_id IN (%s)
		 GROUP BY case_id, reason_key
		 ORDER BY case_id, reports DESC, reason_key`

	// Only an open case matches, so of two decisions made at once, one is
	// recorded.
	resolveOpenCase = `
		UPDATE abuse_cases
		   SET status          = ?,
		       decision_action = ?,
		       decision_note   = ?,
		       decided_by      = ?,
		       decided_at      = NOW()
		 WHERE id = ?
		   AND status = ?`

	reopenClosedCase = `
		UPDATE abuse_cases
		   SET status          = ?,
		       decision_action = '',
		       decision_note   = '',
		       decided_by      = NULL,
		       decided_at      = NULL
		 WHERE id = ?
		   AND status <> ?`

	stampTargetGone = `
		UPDATE abuse_cases
		   SET target_gone_at = NOW()
		 WHERE target_id = ?
		   AND target_gone_at IS NULL`

	// decided_by stays NULL: nobody here decided it.
	closeOpenCaseOfGoneTarget = `
		UPDATE abuse_cases
		   SET status          = ?,
		       decision_action = ?,
		       decided_at      = NOW()
		 WHERE target_id = ?
		   AND status = ?`

	// A person has one report in a case at most, so one fewer is exact.
	// Run before the reports are deleted: it finds the cases through them.
	uncountReportsOfReporter = `
		UPDATE abuse_cases
		  JOIN abuse_reports
		    ON abuse_reports.case_id = abuse_cases.id
		   SET abuse_cases.report_count = abuse_cases.report_count - 1
		 WHERE abuse_reports.reporter_id = ?
		   AND abuse_cases.report_count > 0`

	deleteReportsOfReporter = `
		DELETE FROM abuse_reports
		 WHERE reporter_id = ?`

	// Only open cases. A decided one is the record of a decision, and
	// stays even when the reports behind it are gone.
	deleteOpenCasesWithoutReports = `
		DELETE FROM abuse_cases
		 WHERE status = ?
		   AND report_count = 0`

	blankOldAddresses = `
		UPDATE abuse_reports
		   SET created_ip = ''
		 WHERE created_ip <> ''
		   AND created_at < ?`
)

type scanner interface {
	Scan(dest ...any) error
}

func scanCase(row scanner) (*entities.Case, error) {
	c := &entities.Case{}
	var decidedAt, goneAt sql.NullTime

	if err := row.Scan(
		&c.ID, &c.TargetID, &c.TargetType, &c.TargetOwnerID, &c.Status,
		&c.ReportCount, &c.FirstReportedAt, &c.LastReportedAt,
		&c.DecidedBy, &decidedAt, &c.DecisionAction,
		&c.DecisionNote, &goneAt,
	); err != nil {
		return nil, err
	}

	if decidedAt.Valid {
		c.DecidedAt = &decidedAt.Time
	}
	if goneAt.Valid {
		c.TargetGoneAt = &goneAt.Time
	}

	return c, nil
}

func isDeadlock(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1213
}

func isDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// AddReport runs in one transaction: find or open the case, write the report,
// and count it.
//
// Two people reporting the same node at the same moment both try to open its
// case. The unique index on the open target lets one succeed; the other gets a
// duplicate-key error, reads the case the first one opened, and joins it.
func (r *cases) AddReport(target entities.Case, report entities.Report) (int64, bool, error) {
	// When two people open the same node's first case at the same moment,
	// MySQL may settle it by refusing one of the transactions as a deadlock.
	// That one has done nothing wrong: running it again finds the case the
	// other opened.
	var (
		caseID  int64
		created bool
		err     error
	)
	for attempt := 0; attempt < 3; attempt++ {
		caseID, created, err = r.addReport(target, report)
		if !isDeadlock(err) {
			break
		}
	}
	return caseID, created, err
}

func (r *cases) addReport(target entities.Case, report entities.Report) (int64, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()

	caseID, err := openCaseID(ctx, tx, target.TargetID)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(ctx, insertCase,
			target.TargetID, target.TargetType, target.TargetOwnerID, constants.CaseStatusOpen)

		switch {
		case insertErr == nil:
			caseID, err = result.LastInsertId()
		case isDuplicate(insertErr):
			caseID, err = openCaseID(ctx, tx, target.TargetID)
		default:
			err = insertErr
		}
	}
	if err != nil {
		return 0, false, err
	}

	// One report per person per case. MySQL reports 1 row for an insert, 2
	// for a row it changed, and 0 for one it found identical.
	result, err := tx.ExecContext(ctx, upsertReport,
		caseID, report.ReporterID, report.ReasonKey, report.Details,
		report.RuleTitle, report.RuleProfileID, report.CreatedIP)
	if err != nil {
		return 0, false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	created := affected == 1

	if created {
		if _, err := tx.ExecContext(ctx, countNewReport, caseID); err != nil {
			return 0, false, err
		}
	}

	return caseID, created, tx.Commit()
}

func openCaseID(ctx context.Context, tx *sql.Tx, targetID int64) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, selectOpenCaseForUpdate, targetID).Scan(&id)
	return id, err
}

// Browse lists cases of one status, the most recently reported first.
func (r *cases) Browse(status, reasonKey string, limit, offset uint64) ([]*entities.Case, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, selectCasesByStatus,
		status, reasonKey, reasonKey, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanCases(rows)
}

func scanCases(rows *sql.Rows) ([]*entities.Case, error) {
	list := []*entities.Case{}
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *cases) Count(status, reasonKey string) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var total uint64
	err := r.db.QueryRowContext(ctx, countCasesByStatus, status, reasonKey, reasonKey).Scan(&total)
	return total, err
}

func (r *cases) BrowseForProfile(profileID, viewerID int64, status string, limit, offset uint64) ([]*entities.Case, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, selectCasesForProfile,
		status, profileID, viewerID, profileID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanCases(rows)
}

func (r *cases) CountForProfile(profileID, viewerID int64, status string) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var total uint64
	err := r.db.QueryRowContext(ctx, countCasesForProfile,
		status, profileID, viewerID, profileID).Scan(&total)
	return total, err
}

func (r *cases) ProfileReports(profileID int64, caseIDs []int64) ([]*entities.Report, error) {
	list := []*entities.Report{}
	if len(caseIDs) == 0 {
		return list, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	args := make([]any, 0, len(caseIDs)+1)
	args = append(args, profileID)
	for _, id := range caseIDs {
		args = append(args, id)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(caseIDs)), ",")

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(selectProfileReports, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		report := &entities.Report{}
		if err := rows.Scan(&report.ID, &report.CaseID, &report.ReporterID, &report.ReasonKey,
			&report.Details, &report.RuleTitle, &report.RuleProfileID,
			&report.CreatedAt, &report.UpdatedAt, &report.CreatedIP); err != nil {
			return nil, err
		}
		list = append(list, report)
	}
	return list, rows.Err()
}

func (r *cases) Read(id int64) (*entities.Case, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	return scanCase(r.db.QueryRowContext(ctx, selectCase, id))
}

// BrowseReports lists a case's reports, oldest first.
func (r *cases) BrowseReports(caseID int64) ([]*entities.Report, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, selectReportsOfCase, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []*entities.Report{}
	for rows.Next() {
		report := &entities.Report{}
		if err := rows.Scan(&report.ID, &report.CaseID, &report.ReporterID, &report.ReasonKey,
			&report.Details, &report.RuleTitle, &report.RuleProfileID,
			&report.CreatedAt, &report.UpdatedAt, &report.CreatedIP); err != nil {
			return nil, err
		}
		list = append(list, report)
	}
	return list, rows.Err()
}

// BrowseForTarget lists the other cases the same target has had, newest first.
func (r *cases) BrowseForTarget(targetID, exceptCaseID int64) ([]*entities.Case, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, selectOtherCasesOfTarget, targetID, exceptCaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanCases(rows)
}

// ReasonCounts says, for each of the given cases, how many reports gave each
// reason, the most given first.
func (r *cases) ReasonCounts(caseIDs []int64) ([]entities.ReasonCount, error) {
	if len(caseIDs) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	args := make([]any, len(caseIDs))
	for i, id := range caseIDs {
		args[i] = id
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(caseIDs)), ",")

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(selectReasonCounts, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []entities.ReasonCount
	for rows.Next() {
		var count entities.ReasonCount
		if err := rows.Scan(&count.CaseID, &count.ReasonKey, &count.Count); err != nil {
			return nil, err
		}
		counts = append(counts, count)
	}
	return counts, rows.Err()
}

// Resolve is a single statement that only matches an open case, so of two
// administrators deciding the same case at once, one decision is recorded.
func (r *cases) Resolve(id int64, status, action, note string, decidedBy int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := r.db.ExecContext(ctx, resolveOpenCase,
		status, action, note, decidedBy, id, constants.CaseStatusOpen)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	return affected == 1, err
}

// Reopen clears the decision. If the target has been reported again since,
// and so has a newer open case, the unique index refuses and the caller is
// told to use that one.
func (r *cases) Reopen(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := r.db.ExecContext(ctx, reopenClosedCase,
		constants.CaseStatusOpen, id, constants.CaseStatusOpen)
	if isDuplicate(err) {
		return false, ErrOpenCaseExists
	}
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	return affected == 1, err
}

// MarkTargetGone stamps every case the target has had, and closes the open
// one: there is nothing left to decide about a node that no longer exists.
// Nobody is recorded as having decided it, because nobody here did.
func (r *cases) MarkTargetGone(targetID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if _, err := r.db.ExecContext(ctx, stampTargetGone, targetID); err != nil {
		return 0, err
	}

	result, err := r.db.ExecContext(ctx, closeOpenCaseOfGoneTarget,
		constants.CaseStatusActioned, constants.ActionDeleted, targetID, constants.CaseStatusOpen)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (r *cases) RemoveReportsBy(reporterID int64) (removed int64, err error) {
	if reporterID <= 0 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// One transaction: a count lowered without its report deleted would be
	// lowered again by the next attempt.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, uncountReportsOfReporter, reporterID); err != nil {
		return 0, err
	}

	result, err := tx.ExecContext(ctx, deleteReportsOfReporter, reporterID)
	if err != nil {
		return 0, err
	}
	if removed, err = result.RowsAffected(); err != nil {
		return 0, err
	}

	if removed > 0 {
		if _, err = tx.ExecContext(ctx, deleteOpenCasesWithoutReports, constants.CaseStatusOpen); err != nil {
			return 0, err
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}

	return removed, nil
}

func (r *cases) ForgetAddresses(before time.Time) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := r.db.ExecContext(ctx, blankOldAddresses, before.UTC())
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
