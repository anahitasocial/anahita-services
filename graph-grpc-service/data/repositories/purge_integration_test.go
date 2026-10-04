package repositories_test

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"anahita-libs/data/services"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	"graph/cmd/server/handlers"
	"graph/data/repositories"
	"graph/purge"
	"graph/sweeper"
)

// A whole purge against real MySQL.
//
// An external test package, because purge imports repositories and a test
// inside the package could not import it back. It lives in this directory
// rather than beside purge so it runs in the same binary as the other
// real-database tests: they all empty the same tables, and two packages doing
// that at once would trip over each other.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestPurge -v
//
// Never the development database: the fixture starts by emptying nodes.

// The cast.
const (
	purgeAda   = 1 // the one who is purged
	purgeGrace = 2 // stays
	purgeRoot  = 3 // a super administrator, so Ada is never the last one
	purgeGroup = 8 // Ada founded it and administers it; Grace is a member

	purgeAdaPost        = 10 // on Ada's profile, a photo
	purgeAdaArticle     = 11 // on Ada's profile
	purgeAdaInGroup     = 12 // written by Ada into the group
	purgeGracePost      = 13 // on Grace's profile
	purgeAdaOnGrace     = 14 // written by Ada on Grace's profile
	purgeGraceInGroup   = 15 // written by Grace into the group
	purgeGraceOnAdaPost = 20 // Grace's comment under Ada's post
	purgeGraceOnAdaInGr = 21 // Grace's comment under Ada's post in the group
	purgeAdaOnGracePost = 22 // Ada's comment under Grace's post
	purgeGraceOnOwnPost = 23 // Grace's comment under her own post
	purgeAdaRevision    = 25 // a revision of Ada's article

	purgeHashtag  = 30 // Ada used it first
	purgeLocation = 31 // Ada added it
)

func purgeTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run the purge against MySQL")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

func seedPurgeFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	const person = "'node.actor.person-service.person.v1','person-service'"
	const note = "'node.medium.text-service.note.v1','text-service'"
	const comment = "'node.base.comment-service.comment.v1','comment-service'"
	const follow = "'edge.actor.follow.v1'"
	const personType = "'node.actor.person-service.person.v1'"
	const groupType = "'node.actor.group-service.group.v1'"
	const noteType = "'node.medium.text-service.note.v1'"

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`DELETE FROM notifications`,
		`DELETE FROM purge_work`,
		`DELETE FROM node_deletion_logs`,

		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,person_type,filename,follower_count,leader_count,created_on,modified_on) VALUES
		 (1,` + person + `,'Ada','ada','public',1,'registered','avatar.jpg',1,2,NOW(),NOW()),
		 (2,` + person + `,'Grace','grace','public',1,'registered',NULL,1,2,NOW(),NOW()),
		 (3,` + person + `,'Root','root','public',1,'super-administrator',NULL,0,0,NOW(),NOW())`,

		// What Ada founded. All three name her as created_by, and none of
		// them is hers to take.
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_by,follower_count,created_on,modified_on) VALUES
		 (8,'node.actor.group-service.group.v1','group-service','Circle','circle','public',1,1,2,NOW(),NOW()),
		 (30,'node.tag.hashtag-service.hashtag.v1','hashtag-service','gardens','gardens','',1,1,0,NOW(),NOW()),
		 (31,'node.tag.location-service.location.v1','location-service','Vancouver','vancouver','',1,1,0,NOW(),NOW())`,

		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,owner_id,created_by,filename,comment_count,vote_up_count,subscriber_count,created_on,modified_on) VALUES
		 (10,'node.medium.photo-service.photo.v1','photo-service','a photo','n10','public',1,1,1,'photo.jpg',1,0,0,NOW(),NOW()),
		 (11,'node.medium.text-service.article.v1','text-service','an article','n11','public',1,1,1,NULL,0,0,0,NOW(),NOW()),
		 (12,` + note + `,'in the group','n12','public',1,8,1,NULL,1,0,0,NOW(),NOW()),
		 (13,` + note + `,'by grace','n13','public',1,2,2,NULL,2,2,2,NOW(),NOW()),
		 (14,` + note + `,'on grace','n14','public',1,2,1,NULL,0,0,0,NOW(),NOW()),
		 (15,` + note + `,'grace in the group','n15','public',1,8,2,NULL,0,0,0,NOW(),NOW())`,

		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,parent_id,created_by,created_on,modified_on) VALUES
		 (20,` + comment + `,'c20','c20','',1,10,2,NOW(),NOW()),
		 (21,` + comment + `,'c21','c21','',1,12,2,NOW(),NOW()),
		 (22,` + comment + `,'c22','c22','',1,13,1,NOW(),NOW()),
		 (23,` + comment + `,'c23','c23','',1,13,2,NOW(),NOW()),
		 (25,'node.base.text-service.article-revision.v1','text-service','r25','r25','',1,11,1,NOW(),NOW())`,

		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_on) VALUES
		 (` + follow + `,1,` + personType + `,2,` + personType + `,NOW()),
		 (` + follow + `,2,` + personType + `,1,` + personType + `,NOW()),
		 (` + follow + `,1,` + personType + `,8,` + groupType + `,NOW()),
		 (` + follow + `,2,` + personType + `,8,` + groupType + `,NOW()),
		 ('edge.actor.admin.v1',1,` + personType + `,8,` + groupType + `,NOW()),
		 ('edge.base.voteup.v1',1,` + personType + `,13,` + noteType + `,NOW()),
		 ('edge.base.voteup.v1',2,` + personType + `,13,` + noteType + `,NOW()),
		 ('edge.base.subscription.v1',1,` + personType + `,13,` + noteType + `,NOW()),
		 ('edge.base.subscription.v1',2,` + personType + `,13,` + noteType + `,NOW()),
		 ('edge.base.voteup.v1',2,` + personType + `,10,'node.medium.photo-service.photo.v1',NOW()),
		 ('edge.tag.hashtag.v1',10,'node.medium.photo-service.photo.v1',30,'node.tag.hashtag-service.hashtag.v1',NOW()),
		 ('edge.tag.hashtag.v1',13,` + noteType + `,30,'node.tag.hashtag-service.hashtag.v1',NOW())`,

		// Six that must go, and the last one, between Grace and herself, that
		// must stay.
		`INSERT INTO notifications (type,service,recipient_id,subject_id,object_id,target_id,comment_id,created_on) VALUES
		 ('follow','socialgraph-service',1,2,NULL,NULL,NULL,NOW()),
		 ('follow','socialgraph-service',2,1,NULL,NULL,NULL,NOW()),
		 ('comment','comment-service',2,3,10,NULL,NULL,NOW()),
		 ('comment','comment-service',2,3,13,NULL,22,NOW()),
		 ('medium','text-service',2,3,15,12,NULL,NOW()),
		 ('follow','socialgraph-service',2,3,1,NULL,NULL,NOW()),
		 ('comment','comment-service',2,3,13,NULL,23,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

// fakeFiles records what it is asked to delete.
type fakeFiles struct {
	services.FilesGRPCInterface

	mu      sync.Mutex
	deleted []string
}

func (f *fakeFiles) Delete(req *services.FileDeleteRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, req.Path)
	return nil
}

func ids(t *testing.T, db *sql.DB, query string, args ...any) []int64 {
	t.Helper()

	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("query: %v\n%s", err, query)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func count(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v\n%s", err, query)
	}
	return n
}

func sameIDs(a, b []int64) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// Everything Ada owned or wrote goes, wherever it was; what she founded stays;
// the people she leaves behind have their counters put right.
func TestPurgeRemovesEverythingTheActorOwnedOrWrote(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)
	files := &fakeFiles{}

	// The confirmation dialog first, while there is something to count. Four
	// posts — the photo, the article, and the two notes Ada wrote elsewhere —
	// and the three comments that go: two of Grace's under Ada's posts, one of
	// Ada's under Grace's.
	counts, err := repos.Nodes.ReadDeletionCounts(purgeAda)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts.Posts != 4 || counts.Comments != 3 {
		t.Errorf("the dialog promises %d posts and %d comments; the purge "+
			"removes 4 and 3", counts.Posts, counts.Comments)
	}

	if err := purge.Account(repos, nil, files, purgeAda); err != nil {
		t.Fatalf("purge: %v", err)
	}

	wantNodes := []int64{
		purgeAda, purgeGrace, purgeRoot, purgeGroup,
		purgeGracePost, purgeGraceInGroup, purgeGraceOnOwnPost,
		purgeHashtag, purgeLocation,
	}
	if got := ids(t, db, `SELECT id FROM nodes`); !sameIDs(got, wantNodes) {
		t.Errorf("nodes left: %v\nwant:       %v", got, wantNodes)
	}

	// No edge touches Ada or anything that went. What is left is Grace's own:
	// her membership, her vote and subscription on her post, and its hashtag.
	if n := count(t, db, `
		SELECT COUNT(*) FROM edges e
		LEFT JOIN nodes a ON a.id = e.node_a_id
		LEFT JOIN nodes b ON b.id = e.node_b_id
		WHERE a.id IS NULL OR b.id IS NULL OR e.node_a_id = ? OR e.node_b_id = ?`,
		purgeAda, purgeAda); n != 0 {
		t.Errorf("%d edge(s) still point at Ada or at something deleted", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM edges`); n != 4 {
		t.Errorf("%d edges left, want Grace's 4", n)
	}

	if n := count(t, db, `SELECT COUNT(*) FROM notifications`); n != 1 {
		t.Errorf("%d notification(s) left, want only the one between Grace "+
			"and her own post", n)
	}

	// Counters on what stays.
	for _, tc := range []struct {
		what   string
		id     int64
		column string
		want   int
	}{
		{"Grace's followers, without Ada", purgeGrace, "follower_count", 0},
		{"Grace's leaders, now only the group", purgeGrace, "leader_count", 1},
		{"the group's members", purgeGroup, "follower_count", 1},
		{"comments under Grace's post", purgeGracePost, "comment_count", 1},
		{"votes on Grace's post", purgeGracePost, "vote_up_count", 1},
		{"subscribers to Grace's post", purgeGracePost, "subscriber_count", 1},
	} {
		got := count(t, db, `SELECT `+tc.column+` FROM nodes WHERE id = ?`, tc.id)
		if got != tc.want {
			t.Errorf("%s: %s = %d, want %d", tc.what, tc.column, got, tc.want)
		}
	}

	// The tombstone: the handle is kept, the profile is not.
	var name, alias string
	var enabled bool
	var purgedAt sql.NullTime
	if err := db.QueryRow(
		`SELECT name, alias, enabled, purged_at FROM nodes WHERE id = ?`, purgeAda,
	).Scan(&name, &alias, &enabled, &purgedAt); err != nil {
		t.Fatalf("reading the tombstone: %v", err)
	}
	if name != "" || alias != "ada" || enabled || !purgedAt.Valid {
		t.Errorf("tombstone is name=%q alias=%q enabled=%v purged=%v",
			name, alias, enabled, purgedAt.Valid)
	}

	if n := count(t, db, `SELECT COUNT(*) FROM purge_work`); n != 0 {
		t.Errorf("%d row(s) of work left behind", n)
	}

	// The photo's files and Ada's avatar.
	var photo, avatar bool
	for _, path := range files.deleted {
		photo = photo || strings.Contains(path, "photo")
		avatar = avatar || strings.Contains(path, "avatar")
	}
	if !photo || !avatar {
		t.Errorf("files deleted: %v; want the photo's and the avatar's", files.deleted)
	}

	// A tombstone is not in the sweeper's queue, however old the deletion.
	queue, err := repos.Nodes.BrowseDeletedBefore(time.Now().Add(24*time.Hour), 100)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if len(queue) != 0 {
		t.Errorf("the purged account is back in the queue: %v", queue)
	}

	// And asking again does nothing: one audit row, not two.
	if err := purge.Account(repos, nil, files, purgeAda); err != nil {
		t.Fatalf("second purge: %v", err)
	}
	if n := count(t, db,
		`SELECT COUNT(*) FROM node_deletion_logs WHERE node_id = ? AND action = 'purged'`,
		purgeAda); n != 1 {
		t.Errorf("%d purge audit rows, want 1", n)
	}
}

// More than one batch, stopped part-way, then finished by a later call — and
// an account far past what one three-second transaction could remove.
func TestPurgeResumesAfterAnInterruption(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)

	const total = 5200
	const firstID = 1000

	for start := 0; start < total; start += 1000 {
		var values []string
		for i := start; i < start+1000 && i < total; i++ {
			id := firstID + i
			values = append(values, fmt.Sprintf(
				"(%d,'node.medium.text-service.note.v1','text-service','n','n%d','public',1,1,1,NOW(),NOW())",
				id, id))
		}
		if _, err := db.Exec(
			`INSERT INTO nodes (id,type,service,name,alias,access,enabled,owner_id,created_by,created_on,modified_on) VALUES ` +
				strings.Join(values, ",")); err != nil {
			t.Fatalf("seeding %d notes: %v", total, err)
		}
	}

	// What purge.Account does, as far as the first batch — and then nothing,
	// as if the process had been stopped there.
	if _, err := repos.Nodes.SoftDeleteActor(purgeAda); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if err := repos.Nodes.PreparePurge(purgeAda); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	batch, err := repos.Nodes.NextPurgeBatch(purgeAda, purge.BatchSize)
	if err != nil || batch == nil {
		t.Fatalf("first batch: %v, %v", batch, err)
	}
	if err := repos.Nodes.DeletePurgeBatch(purgeAda, batch); err != nil {
		t.Fatalf("deleting the first batch: %v", err)
	}

	left := count(t, db, `SELECT COUNT(*) FROM nodes WHERE created_by = ? AND id <> ?`, purgeAda, purgeAda)
	if left == 0 || left >= total {
		t.Fatalf("after one batch %d of Ada's nodes are left; the interruption "+
			"is not in the middle of anything", left)
	}

	// Half done, the account is still in the queue.
	queue, err := repos.Nodes.BrowseDeletedBefore(time.Now().Add(24*time.Hour), 100)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if !sameIDs(queue, []int64{purgeAda}) {
		t.Fatalf("queue is %v, want the half-purged account", queue)
	}

	started := time.Now()
	if err := purge.Account(repos, nil, nil, purgeAda); err != nil {
		t.Fatalf("resuming: %v", err)
	}
	t.Logf("resumed and finished %d nodes in %s", left, time.Since(started).Round(time.Millisecond))

	if n := count(t, db, `SELECT COUNT(*) FROM nodes WHERE (created_by = ? OR owner_id = ?) AND type LIKE 'node.medium.%'`, purgeAda, purgeAda); n != 0 {
		t.Errorf("%d of Ada's posts survived the resumed purge", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM purge_work`); n != 0 {
		t.Errorf("%d row(s) of work left behind", n)
	}

	// The counters were listed before the interruption and counted after it.
	if got := count(t, db, `SELECT follower_count FROM nodes WHERE id = ?`, purgeGrace); got != 0 {
		t.Errorf("Grace's follower_count = %d after the resumed purge, want 0", got)
	}
}

// The last super administrator is refused before anything is touched.
func TestPurgeRefusesTheLastSuperAdministrator(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)

	before := count(t, db, `SELECT COUNT(*) FROM nodes`)

	err := purge.Account(repos, nil, nil, purgeRoot)
	if !errors.Is(err, repositories.ErrLastSuperAdmin) {
		t.Fatalf("purging the only super administrator returned %v", err)
	}

	if after := count(t, db, `SELECT COUNT(*) FROM nodes`); after != before {
		t.Errorf("%d nodes before, %d after a refused purge", before, after)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM nodes WHERE id = ? AND deleted_at IS NOT NULL`, purgeRoot); n != 0 {
		t.Error("the refused account was marked deleted")
	}
}

// ---------------------------------------------------------------------
// The request queue
// ---------------------------------------------------------------------

func purgeHandlers(db *sql.DB) (*handlers.Handlers, *repositories.Repositories) {
	repos := repositories.NewRepositories(db)
	return handlers.NewHandlers(&handlers.Env{DB: db, Repos: repos}), repos
}

func superAdmin() *graphpb.Viewer {
	return &graphpb.Viewer{Id: purgeRoot, PersonType: constants.PersonTypeSuperAdmin}
}

// waitForBatch asks how a request is doing until nothing is queued.
func waitForBatch(t *testing.T, h *handlers.Handlers, batchID string) *graphpb.ActorPurgeBatchReadResponse {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for {
		batch, err := h.Actor.ReadPurgeBatch(t.Context(), &graphpb.ActorPurgeBatchReadRequest{BatchId: batchID})
		if err != nil {
			t.Fatalf("reading the batch: %v", err)
		}
		if batch.Total > 0 && batch.Queued == 0 {
			return batch
		}
		if time.Now().After(deadline) {
			t.Fatalf("the batch never finished: %+v", batch)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// A request answers at once with a batch, hides the accounts straight away,
// and the purge follows without anybody running the sweeper.
func TestPurgeRequestHidesAtOnceAndPurgesInTheBackground(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	if _, err := db.Exec(`DELETE FROM purge_requests`); err != nil {
		t.Fatal(err)
	}
	h, _ := purgeHandlers(db)

	result, err := h.Actor.RequestPurge(t.Context(), &graphpb.ActorPurgeBatchRequest{
		Ids:    []int64{purgeAda, purgeGrace, purgeAda},
		Type:   constants.NodeActorPersonEntityType,
		Viewer: superAdmin(),
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if result.BatchId == "" || result.Queued != 2 || len(result.Refused) != 0 {
		t.Fatalf("the request: %+v", result)
	}

	// Hidden before the answer came back, whatever the queue is doing.
	if n := count(t, db, `SELECT COUNT(*) FROM nodes WHERE id IN (?, ?) AND deleted_at IS NOT NULL AND enabled = 0`,
		purgeAda, purgeGrace); n != 2 {
		t.Errorf("%d of the 2 accounts were hidden when the request returned", n)
	}

	batch := waitForBatch(t, h, result.BatchId)
	if batch.Total != 2 || batch.Done != 2 || batch.Failed != 0 {
		t.Fatalf("the batch: %+v", batch)
	}

	if n := count(t, db, `SELECT COUNT(*) FROM nodes WHERE id IN (?, ?) AND purged_at IS NOT NULL`,
		purgeAda, purgeGrace); n != 2 {
		t.Errorf("%d of the 2 accounts are tombstones", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM nodes WHERE type LIKE 'node.medium.%'`); n != 0 {
		t.Errorf("%d posts are left; both authors were purged", n)
	}

	// The audit names the super administrator who asked, once per account.
	if n := count(t, db, `
		SELECT COUNT(*) FROM node_deletion_logs
		WHERE action = 'hard-deleted' AND deleted_by = ? AND node_id IN (?, ?)`,
		purgeRoot, purgeAda, purgeGrace); n != 2 {
		t.Errorf("%d hard-deleted audit rows naming the requester, want 2", n)
	}

	// The progress still names the accounts, though the purge took their names.
	for _, item := range batch.Items {
		if item.Alias != "ada" && item.Alias != "grace" {
			t.Errorf("an item of the batch lost its alias: %+v", item)
		}
	}

	// Asking again finds nothing to do.
	again, err := h.Actor.RequestPurge(t.Context(), &graphpb.ActorPurgeBatchRequest{
		Ids:    []int64{purgeAda},
		Type:   constants.NodeActorPersonEntityType,
		Viewer: superAdmin(),
	})
	if err != nil || again.Queued != 0 || again.Skipped != 1 || again.BatchId != "" {
		t.Errorf("asking again: %+v, %v", again, err)
	}
}

// One actor that cannot be purged refuses the whole request, with a reason
// for each, and nothing is touched.
func TestPurgeRequestIsRefusedWhole(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	if _, err := db.Exec(`DELETE FROM purge_requests`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE nodes SET person_type = 'administrator' WHERE id = ?`, purgeGrace); err != nil {
		t.Fatal(err)
	}
	h, _ := purgeHandlers(db)

	result, err := h.Actor.RequestPurge(t.Context(), &graphpb.ActorPurgeBatchRequest{
		Ids:    []int64{purgeAda, purgeGrace, purgeRoot, purgeGroup, purgeAdaPost, 999},
		Type:   constants.NodeActorPersonEntityType,
		Viewer: superAdmin(),
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	want := map[int64]string{
		purgeGrace:   "administrator",
		purgeRoot:    "yourself",
		purgeGroup:   "wrong_type",
		purgeAdaPost: "wrong_type",
		999:          "not_found",
	}
	got := map[int64]string{}
	for _, refusal := range result.Refused {
		got[refusal.Id] = refusal.Reason
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("refused: %v\nwant:    %v", got, want)
	}
	if result.BatchId != "" || result.Queued != 0 {
		t.Errorf("a refused request still queued something: %+v", result)
	}

	// Ada could have been purged, and was not: the request is all or nothing.
	if n := count(t, db, `SELECT COUNT(*) FROM nodes WHERE deleted_at IS NOT NULL`); n != 0 {
		t.Errorf("%d account(s) were hidden by a refused request", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM purge_requests`); n != 0 {
		t.Errorf("%d request(s) were queued by a refused request", n)
	}

	// Named on purpose, an administrator can go.
	single, err := h.Actor.RequestPurge(t.Context(), &graphpb.ActorPurgeBatchRequest{
		Ids:                 []int64{purgeGrace},
		Type:                constants.NodeActorPersonEntityType,
		AllowAdministrators: true,
		Viewer:              superAdmin(),
	})
	if err != nil || single.Queued != 1 {
		t.Fatalf("removing one administrator by name: %+v, %v", single, err)
	}
	waitForBatch(t, h, single.BatchId)
}

// A request that cannot be carried out is tried a few times and then marked
// failed with the reason, so the queue does not turn on it forever and the
// accounts behind it are still reached.
func TestPurgeRequestThatCannotWorkIsMarkedFailed(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	if _, err := db.Exec(`DELETE FROM purge_requests`); err != nil {
		t.Fatal(err)
	}
	repos := repositories.NewRepositories(db)

	// Queued, and then the node vanished from under the request.
	ghost := entities.NewAnahitaNode()
	ghost.ID, ghost.Alias, ghost.Type = 4242, "ghost", constants.NodeActorPersonEntityType
	if queued, err := repos.PurgeRequests.Add("batch-a", ghost, purgeRoot); err != nil || !queued {
		t.Fatalf("queueing: %v, %v", queued, err)
	}

	ada, err := repos.Nodes.Read(purgeAda)
	if err != nil {
		t.Fatal(err)
	}
	if queued, err := repos.PurgeRequests.Add("batch-a", ada, purgeRoot); err != nil || !queued {
		t.Fatalf("queueing: %v, %v", queued, err)
	}
	if queued, _ := repos.PurgeRequests.Add("batch-b", ada, purgeRoot); queued {
		t.Error("the same account was queued twice")
	}

	purged, err := sweeper.RunRequests(db, repos, nil, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if purged != 1 {
		t.Errorf("purged %d, want Ada only", purged)
	}

	requests, err := repos.PurgeRequests.BrowseBatch("batch-a")
	if err != nil || len(requests) != 2 {
		t.Fatalf("the batch: %v, %v", requests, err)
	}
	for _, request := range requests {
		switch request.ActorID {
		case 4242:
			if request.Status != repositories.PurgeRequestFailed ||
				request.Attempts != repositories.PurgeRequestMaxAttempts || request.Error == "" {
				t.Errorf("the request that cannot work: %+v", request)
			}
		case purgeAda:
			if request.Status != repositories.PurgeRequestDone {
				t.Errorf("the request behind it: %+v", request)
			}
		}
	}

	// Asking again replaces a failed request, which is how it is retried.
	if queued, err := repos.PurgeRequests.Add("batch-c", ghost, purgeRoot); err != nil || !queued {
		t.Errorf("asking again for a failed request: %v, %v", queued, err)
	}
}
