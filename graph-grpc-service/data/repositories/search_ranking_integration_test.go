package repositories

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"

	"graph/ranking"
)

// "Most relevant", against a real database:
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestMostRelevant -v
//
// It empties nodes and edges: a throwaway database only.
func seedRankingFixture(t *testing.T) func(scope, keywords string, viewer int64) []int64 {
	t.Helper()
	db := accessTestDB(t)

	const (
		person  = "'node.actor.person-service.person.v1','person-service'"
		bluesky = "'node.actor.atproto-service.person.v1','atproto-service'"
		group   = "'node.actor.group-service.group.v1','group-service'"
		note    = "'node.medium.text-service.note.v1','text-service'"
		hashtag = "'node.tag.hashtag-service.hashtag.v1','hashtag-service'"
		aPerson = "'node.actor.person-service.person.v1'"
		aNote   = "'node.medium.text-service.note.v1'"
	)
	daysAgo := func(days int) string {
		return fmt.Sprintf("NOW() - INTERVAL %d DAY", days)
	}
	like := func(who, what, days int) string {
		return fmt.Sprintf("('edge.base.voteup.v1',%d,%s,%d,%s,%s)", who, aPerson, what, aNote, daysAgo(days))
	}

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		// Ana searches. Bob is somebody she follows; 2 to 6 like things.
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'Ana','ana','public',1,NOW(),NOW()),
		 (2,` + person + `,'Bob','bob','public',1,NOW(),NOW()),
		 (3,` + person + `,'Cy','cy','public',1,NOW(),NOW()),
		 (4,` + person + `,'Di','di','public',1,NOW(),NOW()),
		 (5,` + person + `,'Ed','ed','public',1,NOW(),NOW()),
		 (6,` + person + `,'Flo','flo','public',1,NOW(),NOW()),
		 (10,` + person + `,'Roaster','coffee','public',1,NOW(),NOW()),
		 (11,` + group + `,'Coffee Lovers','lovers','public',1,NOW(),NOW()),
		 (12,` + group + `,'Vancouver Coffee','vancouver','public',1,NOW(),NOW()),
		 (13,` + group + `,'Decaffeinated','decafcoffeeclub','public',1,NOW(),NOW()),
		 (14,` + bluesky + `,'Boris','bmann.ca','public',1,NOW(),NOW()),
		 (15,` + person + `,'Coffee','beans','public',1,NOW(),NOW()),
		 (100,` + hashtag + `,'coffee','coffee','public',1,NOW(),NOW())`,
		// Posts that mention coffee, written at different times.
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (20,` + note + `,'A coffeehouse visit','public',1,3,` + aPerson + `,3,` + daysAgo(10) + `,NOW()),
		 (21,` + note + `,'Coffee again','public',1,3,` + aPerson + `,3,` + daysAgo(10) + `,NOW()),
		 (23,` + note + `,'Morning coffee','public',1,3,` + aPerson + `,3,` + daysAgo(1) + `,NOW()),
		 (24,` + note + `,'A guide to coffee','public',1,3,` + aPerson + `,3,` + daysAgo(800) + `,NOW()),
		 (25,` + note + `,'Coffee news','public',1,3,` + aPerson + `,3,` + daysAgo(800) + `,NOW()),
		 (26,` + note + `,'Coffee chat','public',1,2,` + aPerson + `,2,` + daysAgo(5) + `,NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,` + aPerson + `,2,` + aPerson + `,NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',21,` + aNote + `,NOW()),
		 ` + like(2, 20, 1) + `,` + like(3, 20, 1) + `,` + like(4, 20, 1) + `,` + like(5, 20, 1) + `,` + like(6, 20, 1) + `,
		 ` + like(2, 24, 3) + `,` + like(3, 24, 4) + `,` + like(4, 24, 5) + `,
		 ` + like(2, 25, 790) + `,` + like(3, 25, 790) + `,` + like(4, 25, 790),
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	repo := NewSearchRepo(db).(*SearchRepo)
	repo.ranking = ranking.NewSource(filepath.Join(t.TempDir(), "none.json"))

	return func(scope, keywords string, viewer int64) []int64 {
		t.Helper()
		nodes, err := repo.Browse(&graphpb.SearchBrowseRequest{
			Scope: scope, Keywords: keywords, Ordering: "relevant", Limit: 50,
		}, feedViewer(viewer))
		if err != nil {
			t.Fatalf("browse %q: %v", keywords, err)
		}
		ids := make([]int64, 0, len(nodes))
		for _, node := range nodes {
			ids = append(ids, node.ID)
		}
		return ids
	}
}

// before says whether `a` comes before `b` in `ids`, both being there.
func before(ids []int64, a, b int64) bool {
	at := map[int64]int{}
	for i, id := range ids {
		at[id] = i
	}
	ia, okA := at[a]
	ib, okB := at[b]
	return okA && okB && ia < ib
}

// THE RULES THIS GUARDS: where the words are comes first; then whether the
// searcher follows it, and how useful people have found it lately; and
// what is old and still used stands above what is new and unused.
func TestMostRelevant(t *testing.T) {
	search := seedRankingFixture(t)
	const ana, visitor = 1, 0

	all := search(constants.SearchScopeAll, "coffee", visitor)

	// Where the words are: the handle, the name, its start, a word in it,
	// inside it, a hashtag, a word in the body, inside a word in the body.
	for _, pair := range [][2]int64{
		{10, 15}, // the handle "coffee" before a name "Coffee"
		{15, 11}, // the name exactly before a name that begins with it
		{11, 12}, // the name's start before a later word in it
		{12, 13}, // a word in the name before the letters inside one ("decafcoffeeclub")
		{13, 21}, // the name before a hashtag
		{21, 23}, // a hashtag before the body
		{23, 20}, // a word in the body before letters inside one ("coffeehouse"), however liked
	} {
		if !before(all, pair[0], pair[1]) {
			t.Errorf("%d should come before %d: %v", pair[0], pair[1], all)
		}
	}

	// Old and useful: the guide from two years ago liked this week comes
	// before the news from two years ago liked then, and before this
	// morning's post that nobody has liked.
	if !before(all, 24, 25) || !before(all, 24, 23) {
		t.Errorf("an old post still liked should come first: %v", all)
	}

	// What the searcher follows: Bob's chat comes first for Ana, who
	// follows him, among the posts that mention coffee in the same way.
	mine := search(constants.SearchScopeMedium, "coffee", ana)
	if !before(mine, 26, 24) {
		t.Errorf("what Ana follows should come first for her: %v", mine)
	}
	theirs := search(constants.SearchScopeMedium, "coffee", visitor)
	if !before(theirs, 24, 26) {
		t.Errorf("for a visitor, the post liked lately comes first: %v", theirs)
	}

	// A handle typed with its @ is found by it, and first.
	if got := search(constants.SearchScopeActor, "@bmann.ca", visitor); len(got) == 0 || got[0] != 14 {
		t.Errorf("@bmann.ca: %v", got)
	}
}

// With interest that never fades, two posts liked as often are as useful,
// however long ago: the later id comes first, and the same on every page.
func TestMostRelevantWithInterestThatNeverFades(t *testing.T) {
	search := seedRankingFixture(t)

	path := filepath.Join(t.TempDir(), "search_ranking.json")
	if err := os.WriteFile(path, []byte(`{"half_life_days": 0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = path
	// The fixture's repository reads a file that is not there; this one
	// is built to read the one just written.
	db := accessTestDB(t)
	repo := NewSearchRepo(db).(*SearchRepo)
	repo.ranking = ranking.NewSource(path)
	nodes, err := repo.Browse(&graphpb.SearchBrowseRequest{
		Scope: constants.SearchScopeMedium, Keywords: "coffee", Ordering: "relevant", Limit: 50,
	}, feedViewer(0))
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	if !before(ids, 25, 24) {
		t.Errorf("liked as often, the later id first: %v", ids)
	}
	_ = search
}
