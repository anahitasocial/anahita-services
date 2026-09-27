package repositories

import (
	"strings"
	"testing"
)

// featured_at is read the way deleted_at is: selected bare, so NULL stays
// "not featured", and scanned into a pointer. A COALESCE to any date would
// make every actor featured.
func TestBaseSelectReadsFeaturedAtBare(t *testing.T) {
	sql, _, err := GetBaseSelectQuery().From("nodes AS node").ToSql()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(sql, "node.featured_at") {
		t.Errorf("featured_at is not selected, so no read carries it:\n%s", sql)
	}
	if strings.Contains(sql, "COALESCE(node.featured_at") {
		t.Errorf("featured_at is COALESCEd; NULL is how \"not featured\" is said:\n%s", sql)
	}
}
