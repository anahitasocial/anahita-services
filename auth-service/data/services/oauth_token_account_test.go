package services

import (
	"errors"
	"testing"
	"time"

	"auth/data/entities"
)

// The refresh grant has to make the checks the sign-in page makes. Whatever it
// does not re-check is frozen as it was when the person signed in, and a
// refresh token is renewed every time it is used.
func TestRefreshAdmitsAccount(t *testing.T) {
	account := func(change func(*entities.User)) *entities.User {
		user := entities.NewUser()
		user.ID = 7
		user.Enabled = true
		if change != nil {
			change(user)
		}
		return user
	}
	now := time.Now()

	cases := []struct {
		what  string
		user  *entities.User
		admit bool
	}{
		{"an enabled account", account(nil), true},
		{"a disabled account", account(func(u *entities.User) { u.Enabled = false }), false},
		{"an account scheduled for deletion", account(func(u *entities.User) { u.DeletedAt = &now; u.Enabled = false }), false},
		// Deletion clears Enabled as well, but each state is refused in its
		// own right, so neither depends on the other being written.
		{"a deleted account still marked enabled", account(func(u *entities.User) { u.DeletedAt = &now }), false},
		{"an archived account", account(func(u *entities.User) { u.ArchivedAt = &now }), false},
		{"no account at all", nil, false},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			err := refreshAdmitsAccount(c.user)
			if c.admit && err != nil {
				t.Errorf("refused: %v", err)
			}
			if !c.admit && !errors.Is(err, ErrRefreshAccountUnavailable) {
				t.Errorf("err = %v, want ErrRefreshAccountUnavailable", err)
			}
		})
	}
}
