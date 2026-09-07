package repositories

import (
	"strings"
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	"github.com/DATA-DOG/go-sqlmock"
	sq "github.com/Masterminds/squirrel"
)

// Search filtered neither deleted nor disabled, for the node or its owner.
// A group deleted today stayed in search for its whole grace period, and so
// did every post in it. Verified live before the fix: a group with
// deleted_at set and enabled = 0 came back from /search with its post.

func lifecycleSQL(t *testing.T, viewer *entities.Viewer) string {
	t.Helper()

	query := SetLifecycleQuery(
		sq.Select("node.id").From("nodes AS node").
			LeftJoin("nodes AS owner ON owner.id = node.owner_id"),
		viewer,
	)

	sql, _, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql: %v", err)
	}
	return strings.ToLower(sql)
}

func TestLifecycleGateCoversNodeAndOwner(t *testing.T) {
	viewer := &entities.Viewer{}
	viewer.ID = 42

	sql := lifecycleSQL(t, viewer)

	// Each of the four, and the owner half is the one that was missing
	// everywhere — a post inherits its container's state.
	for _, want := range []string{
		"node.deleted_at is null",
		"owner.deleted_at is null",
		"node.enabled = ?",
		"coalesce(owner.enabled, 1) = 1",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("missing %q:\n%s", want, sql)
		}
	}
}

// The two NULLs mean opposite things, which is why they are not one predicate.
// The owner join is a LEFT JOIN: a node with no owner yields NULL for both
// columns, and must read as "not owned by a deleted actor" (passes) and "not
// owned by a disabled actor" (passes).
func TestLifecycleGateLetsThroughNodesWithNoOwner(t *testing.T) {
	viewer := &entities.Viewer{}
	viewer.ID = 42

	sql := lifecycleSQL(t, viewer)

	// A bare owner.enabled = 1 would drop every actor, since actors have no
	// owner and the LEFT JOIN gives them NULL.
	if strings.Contains(sql, "owner.enabled = ?") ||
		strings.Contains(sql, "owner.enabled = 1") && !strings.Contains(sql, "coalesce(owner.enabled") {
		t.Errorf("owner.enabled is compared without COALESCE; every actor "+
			"would vanish from search, since an actor has no owner row:\n%s", sql)
	}
}

// Administrators see everything, matching the access query. Moderation has to
// be able to find the thing it is moderating.
func TestLifecycleGateIsBypassedForAdministrators(t *testing.T) {
	admin := &entities.Viewer{}
	admin.ID = 1
	admin.UserType = "super-administrator"

	if !admin.IsAdmin() {
		t.Skip("fixture is not an admin; IsAdmin reads a field this test does not set")
	}

	sql := lifecycleSQL(t, admin)
	if strings.Contains(sql, "deleted_at") || strings.Contains(sql, "enabled") {
		t.Errorf("an administrator got the lifecycle filter:\n%s", sql)
	}
}

// A nil viewer is a logged-out visitor, not an administrator. Reading the
// bypass off a nil pointer would panic; treating nil as admin would expose
// every deleted node to the public.
func TestLifecycleGateAppliesToAnonymousViewers(t *testing.T) {
	sql := lifecycleSQL(t, nil)

	if !strings.Contains(sql, "node.deleted_at is null") {
		t.Errorf("a logged-out visitor was not filtered:\n%s", sql)
	}
}

// Count and Browse must apply the gate identically, or the total describes a
// different set than the rows — pagination then reports pages the results
// cannot fill. The same mismatch already happened once here with the LIKE
// sanitiser.
func TestSearchCountAndBrowseApplyTheLifecycleGateAlike(t *testing.T) {
	const marker = "zzlifecycle"

	for _, name := range []string{"count", "browse"} {
		t.Run(name, func(t *testing.T) {
			db, mock, seen := newNodesTestDB(t)
			repo := &SearchRepo{db: db, dbTimeout: 3 * time.Second}
			viewer := newTestViewer(42)

			if name == "count" {
				mock.ExpectQuery("select").WillReturnRows(
					sqlmock.NewRows([]string{"count"}).AddRow(0))
				_, _ = repo.Count(&graphpb.SearchCountRequest{Keywords: marker}, viewer)
			} else {
				mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))
				_, _ = repo.Browse(&graphpb.SearchBrowseRequest{Keywords: marker, Limit: 10}, viewer)
			}

			if len(*seen) == 0 {
				t.Fatal("no query was issued")
			}
			query := strings.ToLower((*seen)[0])

			for _, want := range []string{
				"node.deleted_at is null",
				"owner.deleted_at is null",
				"coalesce(owner.enabled, 1) = 1",
			} {
				if !strings.Contains(query, want) {
					t.Errorf("%s is missing %q — a deleted group and its posts "+
						"stay findable:\n%s", name, want, query)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------
// Archive
//
// Not a simple exclusion, which is what makes it worth testing carefully.
// Two requirements share the word and want opposite things from search: a
// memorial must stay findable, and a finished group must stop competing with
// live spaces.

func TestArchiveFilterExcludesNodeAndOwner(t *testing.T) {
	viewer := &entities.Viewer{}
	viewer.ID = 42

	query := SetArchiveQuery(
		sq.Select("node.id").From("nodes AS node").
			LeftJoin("nodes AS owner ON owner.id = node.owner_id"),
		viewer,
	)

	sql, _, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql: %v", err)
	}
	got := strings.ToLower(sql)

	for _, want := range []string{"node.archived_at is null", "owner.archived_at is null"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q — an archived group's posts would still be "+
				"pushed at people:\n%s", want, got)
		}
	}
}

// The type-aware rule. A person stays in search; a group does not.
func TestArchiveSearchKeepsPeopleAndDropsGroups(t *testing.T) {
	viewer := &entities.Viewer{}
	viewer.ID = 42

	query := SetArchiveSearchQuery(
		sq.Select("node.id").From("nodes AS node").
			LeftJoin("nodes AS owner ON owner.id = node.owner_id"),
		viewer,
	)

	sql, args, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql: %v", err)
	}
	got := strings.ToLower(sql)

	// An archived row is allowed through only when it is a person, so both
	// halves must appear as alternatives rather than as a flat exclusion.
	if !strings.Contains(got, "node.archived_at is null") ||
		!strings.Contains(got, "node.type = ?") {
		t.Errorf("the node rule is not type-aware; a memorial would leave "+
			"search:\n%s", got)
	}
	if !strings.Contains(got, "owner.archived_at is null") ||
		!strings.Contains(got, "owner.type = ?") {
		t.Errorf("the owner rule is not type-aware:\n%s", got)
	}

	// And the type it excepts must be the person type, not something else.
	people := 0
	for _, a := range args {
		if s, ok := a.(string); ok && s == constants.NodeActorPersonEntityType {
			people++
		}
	}
	if people != 2 {
		t.Errorf("expected the person type twice (node and owner), found %d — "+
			"the exception is for the wrong type", people)
	}
}

// Administrators see everything, matching the other gates. A moderator has to
// be able to find what they are moderating.
func TestArchiveFiltersAreBypassedForAdministrators(t *testing.T) {
	admin := &entities.Viewer{}
	admin.ID = 1
	admin.UserType = constants.UserTypeSuperAdmin

	if !admin.IsAdmin() {
		t.Skip("fixture is not an admin")
	}

	for name, build := range map[string]func(sq.SelectBuilder, *entities.Viewer) sq.SelectBuilder{
		"SetArchiveQuery":       SetArchiveQuery,
		"SetArchiveSearchQuery": SetArchiveSearchQuery,
	} {
		sql, _, err := build(sq.Select("node.id").From("nodes AS node"), admin).ToSql()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Contains(strings.ToLower(sql), "archived_at") {
			t.Errorf("%s filtered an administrator:\n%s", name, sql)
		}
	}
}
