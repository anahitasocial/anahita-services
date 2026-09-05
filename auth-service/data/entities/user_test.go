package entities

import (
	"strings"
	"testing"

	"anahita-libs/constants"
)

// CountRecoveryCodes decides which of three messages somebody gets after
// spending a code, so an off-by-one here tells a person with none left that
// they have one — the reading that would stop them generating a new set.
func TestCountRecoveryCodes(t *testing.T) {
	sep := constants.TotpRecoveryCodeSep

	cases := []struct {
		name  string
		codes string
		want  int
	}{
		// The case the naive len(Split(...)) gets wrong: splitting "" yields
		// one empty element, so it reports 1 for an account with none.
		{"empty", "", 0},
		{"one", "hash1", 1},
		{"several", strings.Join([]string{"a", "b", "c"}, sep), 3},
		// Trailing separator, as removing the last element can leave.
		{"trailing separator", "a" + sep, 1},
		{"only a separator", sep, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := NewUser()
			u.TotpRecoveryCodes = tc.codes

			if got := u.CountRecoveryCodes(); got != tc.want {
				t.Errorf("CountRecoveryCodes() = %d, want %d (codes=%q)", got, tc.want, tc.codes)
			}
		})
	}
}

// Consuming a code must decrement the count, since the notification reports
// what is left AFTER the spend.
func TestValidateRecoveryCodeConsumesExactlyOne(t *testing.T) {
	u := NewUser()
	if err := u.SetRecoveryCodes([]string{"alpha", "bravo", "charlie"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	if got := u.CountRecoveryCodes(); got != 3 {
		t.Fatalf("setup: %d codes, want 3", got)
	}

	if !u.ValidateRecoveryCode("bravo") {
		t.Fatal("a valid code was refused")
	}
	if got := u.CountRecoveryCodes(); got != 2 {
		t.Errorf("after one use: %d codes remain, want 2", got)
	}

	// And it is single-use: the same code must not work twice.
	if u.ValidateRecoveryCode("bravo") {
		t.Error("a spent recovery code was accepted a second time")
	}

	if u.ValidateRecoveryCode("not-a-code") {
		t.Error("an unknown code was accepted")
	}
	if got := u.CountRecoveryCodes(); got != 2 {
		t.Errorf("a rejected code changed the count: %d, want 2", got)
	}
}

// Spending the last code must report zero, not one. This is the case the
// notification treats specially — somebody with no codes left is one lost
// authenticator away from being locked out, and the message says so instead of
// printing a number.
func TestCountReachesZeroAfterTheLastCode(t *testing.T) {
	u := NewUser()
	if err := u.SetRecoveryCodes([]string{"only-one"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	if !u.ValidateRecoveryCode("only-one") {
		t.Fatal("the only code was refused")
	}

	if got := u.CountRecoveryCodes(); got != 0 {
		t.Errorf("after spending the last code: %d remain, want 0", got)
	}
}
