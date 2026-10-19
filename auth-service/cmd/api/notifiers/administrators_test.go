package notifiers

import (
	"testing"

	"auth/data/entities"
	"auth/data/repositories"
)

// WHAT THIS GUARDS (roadmap row 96, part 5): every administrator is told,
// not the first twenty.
func TestEveryAdministratorIsRead(t *testing.T) {
	users := &fakeUsers{}
	for i := 1; i <= 250; i++ {
		admin := &entities.User{}
		admin.ID = int64(i)
		users.admins = append(users.admins, admin)
	}

	all, err := administrators(&repositories.Repositories{Users: users})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 250 {
		t.Errorf("read %d administrators, want 250", len(all))
	}
	if users.pages != 3 {
		t.Errorf("read %d pages, want 3", users.pages)
	}
}
