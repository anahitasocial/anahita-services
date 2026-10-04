package repositories

import (
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"abusereport/constants"
	"abusereport/data/entities"

	_ "github.com/go-sql-driver/mysql"
)

// Against a real MySQL: what these check is the unique index, the upsert and
// what happens when reports arrive together, none of which a mock can show.
//
// Skipped unless ABUSE_TEST_DSN names a THROWAWAY database that has the schema
// (migration-job/schema/init.sql). The two tables are emptied.
//
//	ABUSE_TEST_DSN='root@tcp(127.0.0.1:13309)/fresh?parseTime=true' go test ./data/repositories/ -v
func realCases(t *testing.T) *cases {
	t.Helper()

	dsn := os.Getenv("ABUSE_TEST_DSN")
	if dsn == "" {
		t.Skip("ABUSE_TEST_DSN is not set")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, table := range []string{"abuse_reports", "abuse_cases"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("empty %s: %v", table, err)
		}
	}

	return &cases{db: db}
}

const (
	post      = int64(100)
	author    = int64(7)
	ada       = int64(1)
	grace     = int64(2)
	moderator = int64(9)
)

func target() entities.Case {
	return entities.Case{TargetID: post, TargetType: "node.medium.text-service.note.v1", TargetOwnerID: author}
}

func by(reporter int64, reason string) entities.Report {
	return entities.Report{ReporterID: reporter, ReasonKey: reason, CreatedIP: "203.0.113.9"}
}

func TestRealReportsOfOneNodeShareACase(t *testing.T) {
	repo := realCases(t)

	first, created, err := repo.AddReport(target(), by(ada, "spam"))
	if err != nil || !created {
		t.Fatalf("first report: created=%v err=%v", created, err)
	}

	second, created, err := repo.AddReport(target(), by(grace, "violence"))
	if err != nil || !created {
		t.Fatalf("second report: created=%v err=%v", created, err)
	}
	if second != first {
		t.Fatalf("two reports of one node opened cases %d and %d", first, second)
	}

	item, err := repo.Read(first)
	if err != nil {
		t.Fatal(err)
	}
	if item.ReportCount != 2 || item.Status != constants.CaseStatusOpen || item.TargetOwnerID != author {
		t.Errorf("case: count %d status %q owner %d", item.ReportCount, item.Status, item.TargetOwnerID)
	}

	counts, err := repo.ReasonCounts([]int64{first})
	if err != nil || len(counts) != 2 {
		t.Errorf("reason counts: %v %v", counts, err)
	}
}

// Reporting again changes the report somebody already made. It is not a
// second report, and the case's count does not move.
func TestRealReportingAgainUpdatesTheSameReport(t *testing.T) {
	repo := realCases(t)

	caseID, _, _ := repo.AddReport(target(), by(ada, "spam"))

	again := by(ada, "harassment")
	again.Details = "It got worse."
	_, created, err := repo.AddReport(target(), again)
	if err != nil || created {
		t.Fatalf("second submission: created=%v err=%v", created, err)
	}

	reports, err := repo.BrowseReports(caseID)
	if err != nil || len(reports) != 1 {
		t.Fatalf("%d reports, %v", len(reports), err)
	}
	if reports[0].ReasonKey != "harassment" || reports[0].Details != "It got worse." {
		t.Errorf("report was not updated: %+v", reports[0])
	}

	item, _ := repo.Read(caseID)
	if item.ReportCount != 1 {
		t.Errorf("report count = %d, want 1", item.ReportCount)
	}
}

// Twenty people reporting the same node at once open exactly one case, and
// every one of them is counted.
func TestRealReportsArrivingTogetherOpenOneCase(t *testing.T) {
	repo := realCases(t)

	var wg sync.WaitGroup
	for reporter := int64(1); reporter <= 20; reporter++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := repo.AddReport(target(), by(reporter, "spam")); err != nil {
				t.Errorf("reporter %d: %v", reporter, err)
			}
		}()
	}
	wg.Wait()

	total, err := repo.Count(constants.CaseStatusOpen, "")
	if err != nil || total != 1 {
		t.Fatalf("%d open cases, %v", total, err)
	}

	list, _ := repo.Browse(constants.CaseStatusOpen, "", 10, 0)
	if list[0].ReportCount != 20 {
		t.Errorf("report count = %d, want 20", list[0].ReportCount)
	}
}

func TestRealADecidedCaseStaysDecidedAndANewReportOpensANewOne(t *testing.T) {
	repo := realCases(t)
	first, _, _ := repo.AddReport(target(), by(ada, "spam"))

	closed, err := repo.Resolve(first, constants.CaseStatusDismissed, constants.ActionNone, "Not spam.", moderator)
	if err != nil || !closed {
		t.Fatalf("resolve: %v %v", closed, err)
	}

	// A second decision on the same case does nothing.
	closed, err = repo.Resolve(first, constants.CaseStatusActioned, constants.ActionDeleted, "", moderator)
	if err != nil || closed {
		t.Fatalf("a decided case was decided again: %v %v", closed, err)
	}

	item, _ := repo.Read(first)
	if item.Status != constants.CaseStatusDismissed || item.DecidedBy != moderator ||
		item.DecidedAt == nil || item.DecisionNote != "Not spam." {
		t.Errorf("decision not recorded: %+v", item)
	}

	// Reported again: a new case, with the old one as its history.
	second, created, err := repo.AddReport(target(), by(grace, "spam"))
	if err != nil || !created || second == first {
		t.Fatalf("report after a decision: case %d created=%v err=%v", second, created, err)
	}

	earlier, err := repo.BrowseForTarget(post, second)
	if err != nil || len(earlier) != 1 || earlier[0].ID != first {
		t.Errorf("earlier cases: %v %v", earlier, err)
	}

	// The old one cannot be reopened while the new one is open.
	if _, err := repo.Reopen(first); !errors.Is(err, ErrOpenCaseExists) {
		t.Errorf("reopen with a newer case open: %v, want ErrOpenCaseExists", err)
	}

	// Once the new one is closed, it can.
	if _, err := repo.Resolve(second, constants.CaseStatusDismissed, constants.ActionNone, "", moderator); err != nil {
		t.Fatal(err)
	}
	reopened, err := repo.Reopen(first)
	if err != nil || !reopened {
		t.Fatalf("reopen: %v %v", reopened, err)
	}

	item, _ = repo.Read(first)
	if item.Status != constants.CaseStatusOpen || item.DecidedBy != 0 || item.DecidedAt != nil || item.DecisionNote != "" {
		t.Errorf("a reopened case kept its decision: %+v", item)
	}

	if reopened, _ := repo.Reopen(first); reopened {
		t.Error("an open case was reopened")
	}
}

func TestRealAGoneTargetClosesItsOpenCase(t *testing.T) {
	repo := realCases(t)

	old, _, _ := repo.AddReport(target(), by(ada, "spam"))
	if _, err := repo.Resolve(old, constants.CaseStatusDismissed, constants.ActionNone, "", moderator); err != nil {
		t.Fatal(err)
	}
	open, _, _ := repo.AddReport(target(), by(grace, "spam"))

	closed, err := repo.MarkTargetGone(post)
	if err != nil || closed != 1 {
		t.Fatalf("closed %d cases, %v", closed, err)
	}

	current, _ := repo.Read(open)
	if current.Status != constants.CaseStatusActioned || current.DecisionAction != constants.ActionDeleted ||
		current.DecidedBy != 0 || current.TargetGoneAt == nil {
		t.Errorf("the open case: %+v", current)
	}

	// The earlier, dismissed case keeps its decision and is stamped too.
	earlier, _ := repo.Read(old)
	if earlier.Status != constants.CaseStatusDismissed || earlier.TargetGoneAt == nil {
		t.Errorf("the earlier case: %+v", earlier)
	}

	// Hearing it twice changes nothing.
	if closed, err := repo.MarkTargetGone(post); err != nil || closed != 0 {
		t.Errorf("second time: closed %d, %v", closed, err)
	}
}

// A hashtag has nobody who answers for it. That is NULL in the table and
// zero in Go.
func TestRealATargetWithNoOwner(t *testing.T) {
	repo := realCases(t)

	caseID, _, err := repo.AddReport(
		entities.Case{TargetID: 30, TargetType: "node.tag.hashtag-service.hashtag.v1"},
		by(ada, "spam"))
	if err != nil {
		t.Fatal(err)
	}

	var isNull bool
	if err := repo.db.QueryRow(
		`SELECT target_owner_id IS NULL FROM abuse_cases WHERE id = ?`, caseID).Scan(&isNull); err != nil {
		t.Fatal(err)
	}
	if !isNull {
		t.Error("an ownerless target stored an owner")
	}

	item, _ := repo.Read(caseID)
	if item.TargetOwnerID != 0 {
		t.Errorf("owner = %d, want 0", item.TargetOwnerID)
	}
}

// The list can be narrowed to the cases in which somebody gave one reason.
func TestRealCasesCanBeFilteredByReason(t *testing.T) {
	repo := realCases(t)

	spamOnly, _, _ := repo.AddReport(target(), by(ada, "spam"))

	other := entities.Case{TargetID: 200, TargetType: "node.medium.text-service.note.v1", TargetOwnerID: author}
	mixed, _, _ := repo.AddReport(other, by(ada, "spam"))
	if _, _, err := repo.AddReport(other, by(grace, "violence")); err != nil {
		t.Fatal(err)
	}

	ids := func(reason string) []int64 {
		list, err := repo.Browse(constants.CaseStatusOpen, reason, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		total, err := repo.Count(constants.CaseStatusOpen, reason)
		if err != nil || int(total) != len(list) {
			t.Fatalf("%q: count %d, list %d, %v", reason, total, len(list), err)
		}
		var out []int64
		for _, item := range list {
			out = append(out, item.ID)
		}
		return out
	}

	if got := ids(""); len(got) != 2 {
		t.Errorf("no reason: %v, want both cases", got)
	}
	// A case with two spam reports would still be listed once.
	if got := ids("spam"); len(got) != 2 {
		t.Errorf("spam: %v, want both cases", got)
	}
	if got := ids("violence"); len(got) != 1 || got[0] != mixed {
		t.Errorf("violence: %v, want only case %d (not %d)", got, mixed, spamOnly)
	}
	if got := ids("trademark"); len(got) != 0 {
		t.Errorf("a reason nobody gave: %v", got)
	}
}

// A purged person's reports go. A case others also reported loses one from
// its count; an open case only they reported goes with it; a decided case
// stays as the record of the decision.
func TestRealAPurgedReportersReportsAreRemoved(t *testing.T) {
	repo := realCases(t)

	shared, _, _ := repo.AddReport(target(), by(ada, "spam"))
	if _, _, err := repo.AddReport(target(), by(grace, "spam")); err != nil {
		t.Fatal(err)
	}

	other := entities.Case{TargetID: post + 1, TargetType: "node.medium.text-service.note.v1", TargetOwnerID: author}
	alone, _, _ := repo.AddReport(other, by(ada, "spam"))

	third := entities.Case{TargetID: post + 2, TargetType: "node.medium.text-service.note.v1", TargetOwnerID: author}
	decided, _, _ := repo.AddReport(third, by(ada, "spam"))
	if _, err := repo.Resolve(decided, constants.CaseStatusDismissed, constants.ActionNone, "", moderator); err != nil {
		t.Fatal(err)
	}

	removed, err := repo.RemoveReportsBy(ada)
	if err != nil || removed != 3 {
		t.Fatalf("removed %d reports, %v; want Ada's 3", removed, err)
	}

	if c, err := repo.Read(shared); err != nil || c.ReportCount != 1 {
		t.Errorf("the case Grace also reported: %+v, %v", c, err)
	}
	if reports, _ := repo.BrowseReports(shared); len(reports) != 1 || reports[0].ReporterID != grace {
		t.Errorf("reports left in the shared case: %+v", reports)
	}

	if c, err := repo.Read(alone); err == nil && c != nil {
		t.Errorf("an open case with no reports is still there: %+v", c)
	}

	if c, err := repo.Read(decided); err != nil || c == nil || c.Status != constants.CaseStatusDismissed || c.ReportCount != 0 {
		t.Errorf("the decided case: %+v, %v", c, err)
	}

	// Hearing it twice changes nothing.
	if removed, err := repo.RemoveReportsBy(ada); err != nil || removed != 0 {
		t.Errorf("second time: removed %d, %v", removed, err)
	}
	if c, _ := repo.Read(shared); c.ReportCount != 1 {
		t.Errorf("the count was lowered twice: %d", c.ReportCount)
	}
}

// The address a report came from is kept for a while and then blanked. The
// report itself stays.
func TestRealOldAddressesAreForgotten(t *testing.T) {
	repo := realCases(t)

	id, _, _ := repo.AddReport(target(), by(ada, "spam"))
	if _, _, err := repo.AddReport(target(), by(grace, "spam")); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.db.Exec(
		`UPDATE abuse_reports SET created_at = NOW() - INTERVAL 100 DAY WHERE reporter_id = ?`, ada); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Now().AddDate(0, 0, -constants.AddressRetentionDays)

	blanked, err := repo.ForgetAddresses(cutoff)
	if err != nil || blanked != 1 {
		t.Fatalf("blanked %d addresses, %v; want the one old report", blanked, err)
	}

	reports, _ := repo.BrowseReports(id)
	if len(reports) != 2 {
		t.Fatalf("%d reports left, want both", len(reports))
	}
	for _, report := range reports {
		old := report.ReporterID == ada
		if old && report.CreatedIP != "" {
			t.Errorf("the old report still has its address: %q", report.CreatedIP)
		}
		if !old && report.CreatedIP == "" {
			t.Error("the recent report lost its address")
		}
	}

	if blanked, err := repo.ForgetAddresses(cutoff); err != nil || blanked != 0 {
		t.Errorf("second time: blanked %d, %v", blanked, err)
	}
}
