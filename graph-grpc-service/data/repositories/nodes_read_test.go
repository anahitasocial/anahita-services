package repositories

import (
	"strings"
	"testing"
	"time"

	"anahita-libs/constants"

	"github.com/DATA-DOG/go-sqlmock"
)

// Node ids are unique across every type in one table, so a lookup by id alone
// returns whatever happens to hold that id. GET /groups/<a hashtag id> answered
// 200 with a node.tag.hashtag-service.hashtag.v1 body, and /photos/ and
// /topics/ did the same — each service handing its client an object of a shape
// it never asked for.
//
// ReadByAlias in this package has always filtered on type. The id path never
// grew the same predicate, which is the whole bug.

func TestReadOfTypeFiltersByFamily(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
	}{
		{"actors", constants.NodeActorPrefixEntityType},
		{"media", constants.NodeMediumPrefixEntityType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, seen := newNodesTestDB(t)
			repo := &nodesRepo{db: db, dbTimeout: 3 * time.Second}

			mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

			_, _ = repo.ReadOfType(138943, tc.prefix)

			if len(*seen) == 0 {
				t.Fatal("no query was issued")
			}
			query := strings.ToLower((*seen)[0])

			if !strings.Contains(query, "node.type like") {
				t.Errorf("no family predicate; any node with this id would be "+
					"returned:\n%s", query)
			}
			if !strings.Contains(query, "node.id") {
				t.Errorf("no id predicate:\n%s", query)
			}
		})
	}
}

// The plain Read stays unscoped on purpose — comments and tags are read by id
// too, and they belong to neither family. The point is that callers who know
// the family now have somewhere to say so.
func TestReadRemainsUnscoped(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := &nodesRepo{db: db, dbTimeout: 3 * time.Second}

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, _ = repo.Read(138943)

	// node.type appears in the SELECT list either way, so look for the
	// predicate forms specifically.
	query := strings.ToLower((*seen)[0])
	for _, predicate := range []string{"node.type like", "node.type =", "node.type in"} {
		if strings.Contains(query, predicate) {
			t.Errorf("Read acquired a type predicate (%s); comment and tag reads go through it:\n%s",
				predicate, query)
		}
	}
}

// A hashtag is node.tag.*, so neither family prefix can match it. This is the
// bug stated as an assertion about the constants themselves, which is where it
// actually lived.
func TestHashtagBelongsToNeitherReadFamily(t *testing.T) {
	hashtag := constants.NodeBaseHashtagEntityType

	for _, prefix := range []string{
		constants.NodeActorPrefixEntityType,
		constants.NodeMediumPrefixEntityType,
	} {
		if strings.HasPrefix(hashtag, prefix) {
			t.Errorf("%q starts with %q, so scoping the read would not exclude it",
				hashtag, prefix)
		}
	}

	// And the families do match what they are meant to.
	if !strings.HasPrefix(constants.NodeActorPersonEntityType, constants.NodeActorPrefixEntityType) {
		t.Error("a person is not matched by the actor prefix")
	}
}

// A complete type is matched with equality, not LIKE. Both work today, but
// "…group.v1%" would also match a future "…group.v10", and equality states
// the intent.
func TestReadOfTypeUsesEqualityForACompleteType(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := &nodesRepo{db: db, dbTimeout: 3 * time.Second}

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, _ = repo.ReadOfType(138943, "node.actor.group-service.group.v1")

	query := strings.ToLower((*seen)[0])
	if strings.Contains(query, "node.type like") {
		t.Errorf("a complete type was matched with LIKE:\n%s", query)
	}
	if !strings.Contains(query, "node.type = ?") {
		t.Errorf("no equality predicate on the type:\n%s", query)
	}
}
