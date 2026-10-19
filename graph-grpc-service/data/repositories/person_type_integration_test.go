package repositories

import (
	"errors"
	"testing"
)

// A person's type, against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestPersonType -v
//
// WHAT THIS GUARDS. Only a person here, not deleted, is read or given a type,
// only one of the three a person may have, and only from the type that was
// read: a decision is never applied to a type it was not made about.
func TestPersonTypeIsChangedOnlyForAPersonHere(t *testing.T) {
	db := accessTestDB(t)
	repo := NewNodesRepo(db)

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,person_type,created_at,updated_at) VALUES
		 (1,'node.actor.person-service.person.v1','person-service','Sisko','sisko','public',1,'super-administrator',NOW(),NOW()),
		 (2,'node.actor.person-service.person.v1','person-service','Dax','dax','public',1,'registered',NOW(),NOW()),
		 (3,'node.actor.activitypub-service.person.v1','activitypub-service','Garak','garak','public',1,'registered',NOW(),NOW()),
		 (4,'node.actor.group-service.group.v1','group-service','Bajor','bajor','public',1,'',NOW(),NOW()),
		 (5,'node.actor.person-service.person.v1','person-service','Jadzia','jadzia','public',1,'registered',NOW(),NOW())`,
		`UPDATE nodes SET deleted_at = NOW() WHERE id = 5`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	typeOf := func(id int64) string {
		t.Helper()
		var got string
		if err := db.QueryRow(`SELECT person_type FROM nodes WHERE id = ?`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}

	if got, err := repo.PersonTypeOf(2); err != nil || got != "registered" {
		t.Fatalf("Dax: %q, %v; want registered", got, err)
	}
	if changed, err := repo.SetPersonType(2, "registered", "administrator"); err != nil || !changed || typeOf(2) != "administrator" {
		t.Fatalf("Dax made an administrator: changed %v, now %q, err %v", changed, typeOf(2), err)
	}

	// Judged as registered, but she is an administrator now: nothing is
	// written on a decision about a type she no longer has.
	if changed, err := repo.SetPersonType(2, "registered", "super-administrator"); err != nil || changed || typeOf(2) != "administrator" {
		t.Errorf("changed from a type she does not have: changed %v, now %q, err %v", changed, typeOf(2), err)
	}

	for id, who := range map[int64]string{3: "a person on another server", 4: "a group", 5: "a person deleted", 99: "nobody"} {
		if _, err := repo.PersonTypeOf(id); !errors.Is(err, ErrNotAPerson) {
			t.Errorf("%s read as a person: err %v; want ErrNotAPerson", who, err)
		}
		if changed, _ := repo.SetPersonType(id, "registered", "administrator"); changed {
			t.Errorf("%s given a type", who)
		}
	}
	if typeOf(3) != "registered" || typeOf(5) != "registered" {
		t.Error("something not a person here had its type changed")
	}

	for _, wrong := range []string{"guest", "", "Administrator", "moderator"} {
		if _, err := repo.SetPersonType(2, "administrator", wrong); !errors.Is(err, ErrNotAPersonType) {
			t.Errorf("type %q: err %v; want ErrNotAPersonType", wrong, err)
		}
	}
	if typeOf(2) != "administrator" {
		t.Error("a refused type was written")
	}
}
