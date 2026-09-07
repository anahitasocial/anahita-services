package repositories

import (
	"database/sql"
	"os"
	"regexp"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// Every column the destructive statements name must exist.
//
// TombstoneActor named three that did not — actor_gender, permissions and
// enabled_optional_features — so the UPDATE failed with "Unknown column" on
// every run. Nothing caught it because nothing had ever purged an account, and
// the shape of the failure is the worst available: PurgeActorContent runs
// first and succeeds, so the content was destroyed, no tombstone was written,
// and the row stayed flagged deleted — back in the sweeper's queue to fail the
// same way forever.
//
// A unit test cannot see this. sqlmock does not know the schema, so a
// statement naming an imaginary column passes against it exactly as a correct
// one does. This needs the real table.
//
//	ACCESS_TEST_DSN="root:root@tcp(127.0.0.1:3306)/anahita?parseTime=true" \
//	  go test ./data/repositories/ -run TestDestructiveStatements -v
func TestDestructiveStatementsNameRealColumns(t *testing.T) {
	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	columns := map[string]bool{}
	rows, err := db.Query(`
		SELECT COLUMN_NAME FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'`)
	if err != nil {
		t.Fatalf("reading schema: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		columns[name] = true
	}
	if len(columns) == 0 {
		t.Fatal("no columns found for nodes; is the schema loaded?")
	}

	// Read the SET clause out of the source rather than duplicating it here.
	// A copy would drift, and a drifted copy of this particular list is what
	// the test exists to catch.
	source, err := os.ReadFile("nodes_purge.go")
	if err != nil {
		t.Fatalf("reading source: %v", err)
	}

	setClause := regexp.MustCompile(`(?s)SET\n(.*?)\nWHERE id = \? AND deleted_at IS NOT NULL`).
		FindSubmatch(source)
	if setClause == nil {
		t.Fatal("could not find the tombstone SET clause; has it been rewritten?")
	}

	assignments := regexp.MustCompile(`(?m)^\s*([a-z_]+)\s*=`).
		FindAllSubmatch(setClause[1], -1)
	if len(assignments) == 0 {
		t.Fatal("no assignments parsed out of the SET clause")
	}

	for _, m := range assignments {
		name := string(m[1])
		if !columns[name] {
			t.Errorf("TombstoneActor sets %q, which is not a column on nodes. "+
				"The UPDATE fails, and because the content purge runs first "+
				"that leaves an account destroyed and un-tombstoned.", name)
		}
	}

	t.Logf("checked %d assignments against %d real columns",
		len(assignments), len(columns))
}
