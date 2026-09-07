package repositories

import (
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// Access predicates decide who can see what, and they fail in the quiet
// direction: a wrong clause returns rows rather than an error. Two of them
// were wrong.
//
// These are shape tests over the generated SQL — no database — so they run in
// CI on every change. The behavioural counterpart, which needs real rows, is
// access_integration_test.go beside this file.

func testViewer(id int64) *entities.Viewer {
	v := entities.NewViewer()
	v.ID = id
	v.UserType = "registered"
	return v
}

func generatedSQL(t *testing.T, build func(sq.SelectBuilder) sq.SelectBuilder) string {
	t.Helper()

	base := sq.Select("node.id").From("nodes AS node").
		LeftJoin("nodes AS owner ON node.owner_id = owner.id")

	query, _, err := build(base).ToSql()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return query
}

// Blocking somebody must hide their posts wherever they appear — including
// posts they made INTO a group they do not own, where the row's owner is the
// group and only created_by names the author.
//
// The predicate previously compared block edges against node.id alone. For an
// actor listing node IS the actor so it worked; for a photo, node.id is the
// photo's id and no block edge references it, so the check could never match.
func TestContentAccessBlocksOnOwnerAndAuthor(t *testing.T) {
	queries := map[string]func(sq.SelectBuilder) sq.SelectBuilder{
		"medium": func(b sq.SelectBuilder) sq.SelectBuilder {
			return SetMediumAccessQuery(b, testViewer(1))
		},
		"feed": func(b sq.SelectBuilder) sq.SelectBuilder {
			return SetFeedAccessQuery(b, testViewer(1))
		},
		"polymorphic": func(b sq.SelectBuilder) sq.SelectBuilder {
			return SetPolymorphicAccessQuery(b, testViewer(1))
		},
		"generic": func(b sq.SelectBuilder) sq.SelectBuilder {
			return SetAccessQuery(b, testViewer(1))
		},
	}

	for name, build := range queries {
		t.Run(name, func(t *testing.T) {
			query := generatedSQL(t, build)

			blockClause := blockPredicate(t, query)
			for _, ref := range []string{"node.owner_id", "node.created_by"} {
				if !strings.Contains(blockClause, ref) {
					t.Errorf("the block check does not consider %s, so a blocked "+
						"person's posts stay visible:\n%s", ref, blockClause)
				}
			}
		})
	}
}

// An actor listing is the exception: the row IS the actor, and its created_by
// is whoever created it. Blocking a group's creator must not hide the group.
func TestActorAccessBlocksOnTheActorOnly(t *testing.T) {
	query := generatedSQL(t, func(b sq.SelectBuilder) sq.SelectBuilder {
		return SetActorAccessQuery(b, testViewer(1))
	})

	blockClause := blockPredicate(t, query)
	if strings.Contains(blockClause, "created_by") {
		t.Errorf("an actor listing blocks on created_by; blocking a group's "+
			"creator would hide the group:\n%s", blockClause)
	}
	if !strings.Contains(blockClause, "node.id") {
		t.Errorf("the actor itself is not checked:\n%s", blockClause)
	}
}

// The "only me" rule tested `LIKE 'node.actor.person.%'` while a person's type
// is node.actor.person-service.person.v1 — a hyphen after "person", not a dot.
// The pattern matched nothing, so the rule always fell through to its
// admin-edge branch and a person could not see their own private content.
func TestPersonTypePatternMatchesRealPeople(t *testing.T) {
	const realType = "node.actor.person-service.person.v1"

	queries := map[string]func(sq.SelectBuilder) sq.SelectBuilder{
		"generic": func(b sq.SelectBuilder) sq.SelectBuilder {
			return SetAccessQuery(b, testViewer(1))
		},
		"actor": func(b sq.SelectBuilder) sq.SelectBuilder {
			return SetActorAccessQuery(b, testViewer(1))
		},
		"search": func(b sq.SelectBuilder) sq.SelectBuilder {
			return SetSearchAccessQuery(b, testViewer(1), false)
		},
	}

	for name, build := range queries {
		t.Run(name, func(t *testing.T) {
			query := generatedSQL(t, build)

			for _, pattern := range likePatterns(query) {
				if !strings.HasPrefix(pattern, "node.actor.person") {
					continue
				}
				// The pattern must actually match a real person type.
				prefix := strings.TrimSuffix(pattern, "%")
				if !strings.HasPrefix(realType, prefix) {
					t.Errorf("LIKE %q never matches %q, so the 'only me' rule "+
						"can never identify a person", pattern, realType)
				}
			}
		})
	}
}

// A guest gets the public gate only, and still gets the block check — an
// anonymous viewer has no blocks, but the clause must not be silently dropped
// in a way that changes shape between viewer kinds.
func TestGuestSeesOnlyPublicContent(t *testing.T) {
	guest := entities.NewViewer()
	guest.ID = 0

	query := generatedSQL(t, func(b sq.SelectBuilder) sq.SelectBuilder {
		return SetMediumAccessQuery(b, guest)
	})

	for _, restricted := range []string{"followers", "leaders", "mutuals"} {
		if strings.Contains(query, restricted) {
			t.Errorf("a guest query references %q; only the public gate should "+
				"apply with no viewer:\n%s", restricted, query)
		}
	}
}

// An administrator bypasses the gates entirely, which is deliberate — but it
// must be an explicit early return, not an accident of an empty clause.
func TestAdminBypassesAccessGates(t *testing.T) {
	admin := entities.NewViewer()
	admin.ID = 7
	admin.UserType = "administrator"

	query := generatedSQL(t, func(b sq.SelectBuilder) sq.SelectBuilder {
		return SetMediumAccessQuery(b, admin)
	})

	if strings.Contains(query, "WHERE") {
		t.Errorf("an admin query carries access predicates; the bypass is not "+
			"taking effect:\n%s", query)
	}
}

// blockPredicate extracts the NOT EXISTS clause covering block edges.
func blockPredicate(t *testing.T, query string) string {
	t.Helper()

	i := strings.Index(query, "NOT EXISTS")
	if i < 0 {
		t.Fatalf("no block predicate in the query:\n%s", query)
	}
	end := i + 600
	if end > len(query) {
		end = len(query)
	}
	return query[i:end]
}

// likePatterns returns the literal patterns used in LIKE comparisons.
func likePatterns(query string) []string {
	var out []string
	for _, part := range strings.Split(query, "LIKE '")[1:] {
		if j := strings.Index(part, "'"); j >= 0 {
			out = append(out, part[:j])
		}
	}
	return out
}

// The block predicate spans the row, its owner and its author, so it needs
// applying once. It was applied twice — two byte-identical clauses in every
// search query, left from when there were genuinely two under two names.
func TestSearchAppliesTheBlockPredicateOnce(t *testing.T) {
	viewer := &entities.Viewer{}
	viewer.ID = 42

	query := SetSearchAccessQuery(
		sq.Select("node.id").From("nodes AS node").
			LeftJoin("nodes AS owner ON owner.id = node.owner_id"),
		viewer, false,
	)

	sql, args, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql: %v", err)
	}

	// The edge type is BOUND, not interpolated, so it is counted in the args
	// rather than in the statement text.
	blockArgs := 0
	for _, arg := range args {
		if s, ok := arg.(string); ok && s == constants.EdgeBlockEntityType {
			blockArgs++
		}
	}

	if blockArgs == 0 {
		t.Fatalf("no block predicate at all — a blocked person's posts would "+
			"show up in search:\n%s", sql)
	}
	if blockArgs > 1 {
		t.Errorf("the block predicate is applied %d times; it already spans "+
			"the row, its owner and its author, so once is enough", blockArgs)
	}
}
