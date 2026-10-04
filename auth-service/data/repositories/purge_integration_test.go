package repositories

import (
	"testing"
)

// What a purge removes from auth-service's own tables, against a real MySQL.
// See realTokens for AUTH_TEST_DSN.

// An unused invitation goes with the person who sent it. A used one stays, and
// so does everything anybody else sent.
func TestRealPurgeRemovesUnusedInvitationsFromOnePerson(t *testing.T) {
	repo := realTokens(t)

	const ada, grace = 9101, 9102

	t.Cleanup(func() {
		_, _ = repo.db.Exec(`DELETE FROM tokens WHERE created_by IN (?, ?)`, ada, grace)
	})

	if _, err := repo.db.Exec(`
		INSERT INTO tokens (recipient_email, type, body, is_used, created_at, created_by, updated_at, updated_by) VALUES
		('one@example.com',   'invite',        'purge-test-1', 0, NOW(), ?, NOW(), ?),
		('two@example.com',   'invite',        'purge-test-2', 1, NOW(), ?, NOW(), ?),
		('three@example.com', 'passwordreset', 'purge-test-3', 0, NOW(), ?, NOW(), ?),
		('four@example.com',  'invite',        'purge-test-4', 0, NOW(), ?, NOW(), ?)`,
		ada, ada, ada, ada, ada, ada, grace, grace,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	removed, err := repo.DeleteUnusedFrom("invite", ada)
	if err != nil {
		t.Fatalf("DeleteUnusedFrom: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d, want only Ada's one unused invitation", removed)
	}

	var left int
	if err := repo.db.QueryRow(
		`SELECT COUNT(*) FROM tokens WHERE body LIKE 'purge-test-%'`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 3 {
		t.Errorf("%d of the other three tokens are left", left)
	}
}

// A request is found by its address or by its username, and an empty value
// matches nothing.
func TestRealPurgeRemovesSignupRequests(t *testing.T) {
	db := realTokens(t).db
	repo := &signupRequests{db: db}

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM signup_requests WHERE username LIKE 'purgetest%'`)
	})

	if _, err := db.Exec(`
		INSERT INTO signup_requests (username, email, password, status, created_at, updated_at) VALUES
		('purgetest1', 'purgetest1@example.com', 'x', 'approved', NOW(), NOW()),
		('purgetest2', 'renamed@example.com',    'x', 'approved', NOW(), NOW()),
		('purgetest3', 'purgetest3@example.com', 'x', 'pending',  NOW(), NOW())`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if removed, err := repo.DeleteFor("", ""); err != nil || removed != 0 {
		t.Fatalf("empty values removed %d (%v); they must match nothing", removed, err)
	}

	if removed, err := repo.DeleteFor("purgetest1@example.com", ""); err != nil || removed != 1 {
		t.Errorf("by address: removed %d (%v), want 1", removed, err)
	}
	if removed, err := repo.DeleteFor("no-longer@example.com", "purgetest2"); err != nil || removed != 1 {
		t.Errorf("by username: removed %d (%v), want 1", removed, err)
	}

	var left int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM signup_requests WHERE username LIKE 'purgetest%'`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Errorf("%d request(s) left, want the one that belongs to somebody else", left)
	}
}
