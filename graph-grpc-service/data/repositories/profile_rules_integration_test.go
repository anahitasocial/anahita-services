package repositories

import (
	"errors"
	"testing"
)

// A group's rules against a real database (ACCESS_TEST_DSN; see
// access_integration_test.go):
//
//	go test ./data/repositories/ -run TestAGroupsRules -v
//
// THREE RULES THESE GUARD. Every change raises the version, so whoever agreed
// before is asked again. Agreeing is to one version, and kept per person.
// And a group has at most ten.
func TestAGroupsRulesAndWhoAgreedToThem(t *testing.T) {
	db := accessTestDB(t)
	repo := NewProfileRulesRepo(db)
	const promenade, quark = 9001, 9002
	t.Cleanup(func() {
		db.Exec(`DELETE FROM profile_rules WHERE profile_id = ?`, promenade)
		db.Exec(`DELETE FROM profile_rule_versions WHERE profile_id = ?`, promenade)
		db.Exec(`DELETE FROM profile_rule_agreements WHERE profile_id = ?`, promenade)
	})

	none, err := repo.Read(promenade)
	if err != nil || none.Version != 0 || len(none.Items) != 0 {
		t.Fatalf("a group with no rules: %+v %v", none, err)
	}

	first, err := repo.Set(promenade, []ProfileRule{{Title: "No gambling debts"}, {Title: "Be kind", Body: "To Ferengi too."}})
	if err != nil || first.Version != 1 || len(first.Items) != 2 || first.Items[1].Body != "To Ferengi too." {
		t.Fatalf("set: %+v %v", first, err)
	}
	if err := repo.Agree(quark, promenade, first.Version); err != nil {
		t.Fatal(err)
	}
	if agreed, _ := repo.Agreed(quark, promenade); agreed != 1 {
		t.Errorf("agreed to %d", agreed)
	}

	// Changed: the version rises, and Quark's agreement is to the old one.
	second, err := repo.Set(promenade, []ProfileRule{{Title: "Be kind"}})
	if err != nil || second.Version != 2 || len(second.Items) != 1 || second.Items[0].Title != "Be kind" {
		t.Fatalf("changed: %+v %v", second, err)
	}
	if agreed, _ := repo.Agreed(quark, promenade); agreed != 1 {
		t.Errorf("agreed to %d after a change", agreed)
	}

	// Taken away: a change too.
	if gone, err := repo.Set(promenade, nil); err != nil || gone.Version != 3 || len(gone.Items) != 0 {
		t.Errorf("taken away: %+v %v", gone, err)
	}

	eleven := make([]ProfileRule, 11)
	for i := range eleven {
		eleven[i] = ProfileRule{Title: "A rule"}
	}
	if _, err := repo.Set(promenade, eleven); !errors.Is(err, ErrTooManyRules) {
		t.Errorf("eleven rules: %v", err)
	}
}
