package repositories

import (
	"errors"
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Run as the other integration tests here are, with ACCESS_TEST_DSN.
//
// THE RULE THIS GUARDS: what is kept from another server is a person there
// or a note of theirs, once each, and nothing sent from outside can write
// over, or pass as, what was made here.
func TestWhatIsKeptFromAnotherServer(t *testing.T) {
	t.Setenv("ENV", "prod")
	t.Setenv("DOMAIN_NAME", "example.org")

	db := accessTestDB(t)
	for _, statement := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
			(1,'node.actor.person-service.person.v1','person-service','Ana','ana','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
			(10,'node.medium.text-service.note.v1','text-service','Ana''s own note','public',1,1,'node.actor.person-service.person.v1',1,NOW(),NOW())`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed: %v\n%s", err, statement)
		}
	}

	repo := NewNodesRepo(db)

	alice := func() *entities.AnahitaNode {
		return &entities.AnahitaNode{
			Type:         constants.NodeActorActivityPubPersonEntityType,
			URI:          "https://mastodon.example/users/alice",
			OriginDomain: "Mastodon.Example",
			Name:         "Alice",
			Alias:        "Alice@mastodon.example",
			Body:         "Grows things.",
		}
	}

	// Somebody new is added, tidied, and open to read.
	saved, created, err := repo.UpsertRemote(alice())
	if err != nil || !created {
		t.Fatalf("adding alice: %v (created %v)", err, created)
	}
	if saved.Alias != "alice@mastodon.example" || saved.OriginDomain != "mastodon.example" ||
		saved.Access != "public" || !saved.Enabled || saved.UUID == "" || !saved.IsRemote() || !saved.IsPerson() {
		t.Errorf("alice was kept as %+v", saved)
	}
	if saved.Service != "activitypub-service" {
		t.Errorf("service %q", saved.Service)
	}

	// Heard of again, she is the same node, brought up to date.
	renamed := alice()
	renamed.Name, renamed.Body, renamed.Alias = "Alice B.", "Grows more things.", "alice_b@mastodon.example"
	again, created, err := repo.UpsertRemote(renamed)
	if err != nil || created || again.ID != saved.ID {
		t.Fatalf("alice again: %+v (created %v, %v)", again, created, err)
	}
	if again.Name != "Alice B." || again.Body != "Grows more things." || again.Alias != "alice_b@mastodon.example" || again.UUID != saved.UUID {
		t.Errorf("alice was brought up to date as %+v", again)
	}

	// Found by what she is called there, and nothing made here is.
	if found, err := repo.ReadByURI("https://mastodon.example/users/alice"); err != nil || found == nil || found.ID != saved.ID {
		t.Errorf("alice by her address: %+v (%v)", found, err)
	}
	if found, _ := repo.ReadByURI(""); found != nil {
		t.Errorf("no address found node %d", found.ID)
	}

	// A note of hers, dated when she wrote it.
	written := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	note := func() *entities.AnahitaNode {
		return &entities.AnahitaNode{
			Type:         constants.NodeMediumNoteEntityType,
			URI:          "https://mastodon.example/users/alice/statuses/1",
			OriginDomain: "mastodon.example",
			OwnerID:      saved.ID,
			Body:         "First beans.",
			Language:     "en",
			CreatedAt:    written,
		}
	}
	kept, created, err := repo.UpsertRemote(note())
	if err != nil || !created {
		t.Fatalf("adding her note: %v", err)
	}
	if kept.OwnerID != saved.ID || kept.CreatedBy != saved.ID || kept.OwnerType != constants.NodeActorActivityPubPersonEntityType ||
		!kept.CreatedAt.Equal(written) || kept.Access != "public" {
		t.Errorf("her note was kept as %+v", kept)
	}

	edited := note()
	edited.Body = "First beans, at last."
	if again, created, err := repo.UpsertRemote(edited); err != nil || created || again.ID != kept.ID || again.Body != "First beans, at last." || !again.CreatedAt.Equal(written) {
		t.Errorf("her note edited: %+v (created %v, %v)", again, created, err)
	}

	// A note that says it is from the future is from now.
	early := note()
	early.URI, early.CreatedAt = "https://mastodon.example/users/alice/statuses/2", time.Now().Add(48*time.Hour)
	if kept, _, err := repo.UpsertRemote(early); err != nil || kept.CreatedAt.After(time.Now().Add(time.Minute)) {
		t.Errorf("a note from the future was dated %v (%v)", kept.CreatedAt, err)
	}

	// What is refused, and why.
	refused := map[string]struct {
		change func(*entities.AnahitaNode)
		of     func() *entities.AnahitaNode
		want   error
	}{
		"no address":                 {func(n *entities.AnahitaNode) { n.URI = "" }, alice, ErrRemoteURI},
		"an address of ours":         {func(n *entities.AnahitaNode) { n.URI = "https://api.example.org/ap/users/1" }, alice, ErrRemoteURI},
		"an address on our web host": {func(n *entities.AnahitaNode) { n.URI = "https://example.org/people/ana" }, alice, ErrRemoteURI},
		"an address in the clear":    {func(n *entities.AnahitaNode) { n.URI = "http://mastodon.example/users/eve" }, alice, ErrRemoteURI},
		"an address with a space":    {func(n *entities.AnahitaNode) { n.URI = "https://mastodon.example/users/e ve" }, alice, ErrRemoteURI},
		"no server":                  {func(n *entities.AnahitaNode) { n.URI, n.OriginDomain = "https://mastodon.example/users/eve", "" }, alice, ErrRemoteDomain},
		"a person here": {func(n *entities.AnahitaNode) {
			n.URI, n.Type = "https://mastodon.example/users/eve", constants.NodeActorPersonEntityType
		}, alice, ErrRemoteType},
		"a group": {func(n *entities.AnahitaNode) {
			n.URI, n.Type = "https://mastodon.example/users/eve", "node.actor.group-service.group.v1"
		}, alice, ErrRemoteType},
		"a name like one of ours": {func(n *entities.AnahitaNode) { n.URI, n.Alias = "https://mastodon.example/users/eve", "ana" }, alice, ErrRemoteAlias},
		"a name somebody has": {func(n *entities.AnahitaNode) {
			n.URI, n.Alias = "https://mastodon.example/users/eve", "alice_b@mastodon.example"
		}, alice, ErrRemoteAlias},
		"a person become a note":     {func(n *entities.AnahitaNode) { n.URI = "https://mastodon.example/users/alice" }, note, ErrRemoteMismatch},
		"a note on a profile here":   {func(n *entities.AnahitaNode) { n.URI, n.OwnerID = "https://mastodon.example/x/3", 1 }, note, ErrRemoteOwner},
		"a note on nobody's profile": {func(n *entities.AnahitaNode) { n.URI, n.OwnerID = "https://mastodon.example/x/4", 999 }, note, ErrRemoteOwner},
		"a note on a note":           {func(n *entities.AnahitaNode) { n.URI, n.OwnerID = "https://mastodon.example/x/5", kept.ID }, note, ErrRemoteOwner},
	}
	for name, c := range refused {
		node := c.of()
		c.change(node)
		if _, _, err := repo.UpsertRemote(node); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}

	// A note cannot be moved to somebody else by sending it again as theirs.
	bob := alice()
	bob.URI, bob.Alias, bob.Name = "https://mastodon.example/users/bob", "bob@mastodon.example", "Bob"
	bobSaved, _, err := repo.UpsertRemote(bob)
	if err != nil {
		t.Fatal(err)
	}
	stolen := note()
	stolen.OwnerID = bobSaved.ID
	if _, _, err := repo.UpsertRemote(stolen); !errors.Is(err, ErrRemoteMismatch) {
		t.Errorf("a note was taken by somebody else: %v", err)
	}

	// Through all of it, what was made here is as it was.
	var (
		name, body string
		remote     int
	)
	if err := db.QueryRow(`SELECT (SELECT name FROM nodes WHERE id = 1), (SELECT body FROM nodes WHERE id = 10), (SELECT COUNT(*) FROM nodes WHERE id IN (1, 10) AND uri IS NOT NULL)`).Scan(&name, &body, &remote); err != nil {
		t.Fatal(err)
	}
	if name != "Ana" || body != "Ana's own note" || remote != 0 {
		t.Errorf("what was made here changed: %q, %q, %d", name, body, remote)
	}

	// A note of hers that answers Ana's: a reply in Ana's thread, seen by
	// whoever may see Ana's note and saying nothing of its own about who.
	answer := func(uri string, parent int64) *entities.AnahitaNode {
		reply := note()
		reply.URI = uri
		reply.Body = "Mine are up too."
		reply.ParentID = parent
		// Whatever is sent for the thread it is in is not believed.
		reply.RootID = 999
		return reply
	}
	first, created, err := repo.UpsertRemote(answer("https://mastodon.example/users/alice/statuses/21", 10))
	if err != nil || !created || first.ParentID != 10 || first.RootID != 10 || first.Access != "" || first.OwnerID != saved.ID {
		t.Fatalf("her reply was kept as %+v (%v)", first, err)
	}
	// And one that answers her own reply: in the same thread, under hers.
	second, _, err := repo.UpsertRemote(answer("https://mastodon.example/users/alice/statuses/22", first.ID))
	if err != nil || second.ParentID != first.ID || second.RootID != 10 {
		t.Fatalf("her reply to her reply was kept as %+v (%v)", second, err)
	}
	said := func(id int64) (count int) {
		t.Helper()
		if err := db.QueryRow(`SELECT COALESCE(comment_count, 0) FROM nodes WHERE id = ?`, id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	if said(10) != 2 || said(first.ID) != 1 {
		t.Errorf("counted %d under Ana's note and %d under the reply, want 2 and 1", said(10), said(first.ID))
	}

	// Sent again as an answer to something else, it stays where it is.
	moved := answer(second.URI, 10)
	moved.Body = "Mine are up too, at last."
	if again, created, err := repo.UpsertRemote(moved); err != nil || created || again.ParentID != first.ID || again.Body != "Mine are up too, at last." {
		t.Errorf("a reply sent again: %+v (created %v, %v)", again, created, err)
	}

	// What cannot be answered: nothing, a person, a post that is disabled.
	if _, err := db.Exec(`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		(11,'node.medium.text-service.note.v1','text-service','Put away','public',0,1,'node.actor.person-service.person.v1',1,NOW(),NOW())`); err != nil {
		t.Fatal(err)
	}
	for name, parent := range map[string]int64{"nothing there": 424242, "a person": 1, "a post that is disabled": 11} {
		if _, _, err := repo.UpsertRemote(answer("https://mastodon.example/users/alice/statuses/29", parent)); !errors.Is(err, ErrRemoteParent) {
			t.Errorf("answering %s: %v", name, err)
		}
	}

	// Taken away there, a reply is one fewer said here.
	if gone, err := repo.DeleteRemote(second.URI, saved.ID); err != nil || !gone || said(10) != 1 || said(first.ID) != 0 {
		t.Errorf("her reply taken away: %v (%v), counted %d and %d", gone, err, said(10), said(first.ID))
	}
	if gone, err := repo.DeleteRemote(first.URI, saved.ID); err != nil || !gone || said(10) != 0 {
		t.Errorf("her other reply taken away: %v (%v), counted %d", gone, err, said(10))
	}

	// Taken away where it was written, it is taken away here: by its
	// author, and nobody else, and never something that was made here.
	if _, err := db.Exec(`INSERT INTO edges (type, node_a_id, node_a_type, node_b_id, node_b_type, created_by, modified_by, created_at, updated_at) VALUES ('edge.base.voteup.v1', 1, 'node.actor.person-service.person.v1', ?, 'node.medium.text-service.note.v1', 1, 1, NOW(), NOW())`, kept.ID); err != nil {
		t.Fatal(err)
	}
	for name, try := range map[string]struct {
		uri   string
		owner int64
	}{
		"by somebody else":         {kept.URI, bobSaved.ID},
		"a person":                 {saved.URI, saved.ID},
		"something made here":      {"", 1},
		"an address nothing is at": {"https://mastodon.example/nothing", saved.ID},
	} {
		if gone, err := repo.DeleteRemote(try.uri, try.owner); err != nil || gone {
			t.Errorf("%s: deleted %v (%v)", name, gone, err)
		}
	}
	if gone, err := repo.DeleteRemote(kept.URI, saved.ID); err != nil || !gone {
		t.Errorf("by its author: %v (%v)", gone, err)
	}
	var left, likes int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM nodes WHERE id = ?), (SELECT COUNT(*) FROM edges WHERE node_b_id = ?)`, kept.ID, kept.ID).Scan(&left, &likes); err != nil || left != 0 || likes != 0 {
		t.Errorf("after: %d node, %d likes (%v)", left, likes, err)
	}
	if again, _ := repo.DeleteRemote(kept.URI, saved.ID); again {
		t.Error("deleted twice")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE id IN (1, 10)`).Scan(&left); err != nil || left != 2 {
		t.Errorf("what was made here: %d left", left)
	}

	// Her account closed there: she goes, with what she posted and every
	// follow that touched her. Nobody made here goes with her.
	if _, _, err := repo.UpsertRemote(note()); err != nil {
		t.Fatal(err)
	}
	for _, edge := range []string{
		`INSERT INTO edges (type, node_a_id, node_a_type, node_b_id, node_b_type, created_by, modified_by, created_at, updated_at) VALUES ('edge.actor.follow.v1', 1, 'p', ?, 'p', 1, 1, NOW(), NOW())`,
		`INSERT INTO edges (type, node_a_id, node_a_type, node_b_id, node_b_type, created_by, modified_by, created_at, updated_at) VALUES ('edge.actor.follow.v1', ?, 'p', 1, 'p', 1, 1, NOW(), NOW())`,
	} {
		if _, err := db.Exec(edge, saved.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, not := range []string{"", "https://mastodon.example/nobody", note().URI} {
		if gone, _, err := repo.DeleteRemoteActor(not); err != nil || gone {
			t.Errorf("%q: deleted %v (%v)", not, gone, err)
		}
	}
	gone, neighbours, err := repo.DeleteRemoteActor(saved.URI)
	if err != nil || !gone || len(neighbours) != 1 || neighbours[0] != 1 {
		t.Errorf("alice: deleted %v, neighbours %v (%v)", gone, neighbours, err)
	}
	var hers, edges, ours int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM nodes WHERE id = ? OR owner_id = ?), (SELECT COUNT(*) FROM edges WHERE node_a_id = ? OR node_b_id = ?), (SELECT COUNT(*) FROM nodes WHERE id IN (1, 10))`, saved.ID, saved.ID, saved.ID, saved.ID).Scan(&hers, &edges, &ours); err != nil || hers != 0 || edges != 0 || ours != 2 {
		t.Errorf("after: %d of hers, %d edges, %d of ours (%v)", hers, edges, ours, err)
	}
	// Bob, on the same server, is untouched.
	if still, _ := repo.ReadByURI(bobSaved.URI); still == nil {
		t.Error("somebody else from her server went with her")
	}

	// Somebody on Bluesky, under an at:// URI.
	carol, created, err := repo.UpsertRemote(&entities.AnahitaNode{
		Type:         constants.NodeActorATProtoPersonEntityType,
		URI:          "at://did:plc:ewvi7nxzyoun6zhxrhs64oiz",
		OriginDomain: "bsky.social",
		Name:         "Carol",
		Alias:        "carol.bsky.social",
	})
	if err != nil || !created || carol.Service != "atproto-service" {
		t.Errorf("carol: %+v (%v)", carol, err)
	}
}
