package repositories

import (
	"sync"
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"
)

// Events against a real database, on the fixture the thread tests use.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestEvent -v
//
// WHAT THESE GUARD. Never more people going than there are places, however
// many ask at once. No answering an event that is over or called off. And
// an answer can always be taken back.

const (
	testEvent     = 900
	testEventOver = 901
)

func seedEvents(t *testing.T, repo EventsRepoInterface, capacity uint32) {
	t.Helper()

	db := repo.(*eventsRepo).db
	for _, stmt := range []string{
		`DELETE FROM events`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,starts_at,ends_at,created_at,updated_at) VALUES
		 (900,'` + constants.NodeActorEventEntityType + `','actor-service','Picnic','picnic','public',1,NOW() + INTERVAL 1 DAY,NOW() + INTERVAL 2 DAY,NOW(),NOW()),
		 (901,'` + constants.NodeActorEventEntityType + `','actor-service','Last year','last-year','public',1,NOW() - INTERVAL 3 DAY,NOW() - INTERVAL 2 DAY,NOW(),NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	for _, id := range []int64{testEvent, testEventOver} {
		if err := repo.Save(&EventDetails{NodeID: id, TimezoneName: "America/Toronto", Capacity: capacity, OnlineURL: "https://meet.example/x"}); err != nil {
			t.Fatalf("save %d: %v", id, err)
		}
	}
}

func TestEventAnswers(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewEventsRepo(db)
	seedEvents(t, repo, 2)

	answer := func(person int64, rsvp string, want string) {
		t.Helper()
		got, err := repo.Rsvp(testEvent, person, rsvp)
		if err != nil || got != want {
			t.Errorf("person %d answering %q: %q (err %v), want %q", person, rsvp, got, err, want)
		}
	}
	read := func(person int64) *EventDetails {
		t.Helper()
		events, err := repo.Read([]int64{testEvent, 12345}, person)
		if err != nil {
			t.Fatal(err)
		}
		if _, isEvent := events[12345]; isEvent {
			t.Error("something that is not an event has details")
		}
		return events[testEvent]
	}

	// Two places. Alice and Bob take them; Carol is told it is full, and
	// can still say maybe.
	answer(replyAlice, constants.EventRsvpGoing, constants.EventRsvpSaved)
	answer(replyBob, constants.EventRsvpGoing, constants.EventRsvpSaved)
	answer(replyCarol, constants.EventRsvpGoing, constants.EventRsvpFull)
	answer(replyCarol, constants.EventRsvpMaybe, constants.EventRsvpSaved)

	if got := read(replyCarol); got.GoingCount != 2 || got.MaybeCount != 1 || got.ViewerRsvp != constants.EventRsvpMaybe {
		t.Errorf("as Carol: %+v, want 2 going, 1 maybe, and maybe for her", got)
	}
	if got := read(replyAlice); got.ViewerRsvp != constants.EventRsvpGoing || got.TimezoneName != "America/Toronto" || got.Capacity != 2 {
		t.Errorf("as Alice: %+v", got)
	}
	if got := read(0); got.ViewerRsvp != "" {
		t.Errorf("a visitor has answered %q", got.ViewerRsvp)
	}

	// Somebody already going can say so again: their own place is not
	// counted against them.
	answer(replyAlice, constants.EventRsvpGoing, constants.EventRsvpSaved)

	// Bob moves to maybe, which frees a place for Carol.
	answer(replyBob, constants.EventRsvpMaybe, constants.EventRsvpSaved)
	answer(replyCarol, constants.EventRsvpGoing, constants.EventRsvpSaved)
	if got := read(replyBob); got.GoingCount != 2 || got.MaybeCount != 1 {
		t.Errorf("after the swap: %d going, %d maybe, want 2 and 1", got.GoingCount, got.MaybeCount)
	}

	// One edge a person, however often they answer.
	var edges int
	if err := db.QueryRow(`SELECT COUNT(*) FROM edges WHERE type = 'edge.actor.follow.v1' AND node_b_id = ?`, testEvent).Scan(&edges); err != nil || edges != 3 {
		t.Errorf("%d follow edges on the event (err %v), want 3", edges, err)
	}

	// Following it the ordinary way, with nothing said, counts as maybe.
	if _, err := db.Exec(`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES ('edge.actor.follow.v1',5,'node.actor.person-service.person.v1',?,?,NOW())`, testEvent, constants.NodeActorEventEntityType); err != nil {
		t.Fatal(err)
	}
	if got := read(5); got.MaybeCount != 2 || got.ViewerRsvp != constants.EventRsvpMaybe {
		t.Errorf("after a plain follow: %+v, want 2 maybe and maybe for the follower", got)
	}

	// Taking an answer back.
	answer(replyAlice, "", constants.EventRsvpSaved)
	if got := read(replyAlice); got.GoingCount != 1 || got.ViewerRsvp != "" {
		t.Errorf("after Alice left: %+v", got)
	}

	// An event that is over takes no answers, and one that is not there
	// says so.
	if got, err := repo.Rsvp(testEventOver, replyAlice, constants.EventRsvpGoing); err != nil || got != constants.EventRsvpClosed {
		t.Errorf("answering an event that is over: %q (err %v), want closed", got, err)
	}
	if got, err := repo.Rsvp(4242, replyAlice, constants.EventRsvpGoing); err != nil || got != constants.EventRsvpMissing {
		t.Errorf("answering nothing: %q (err %v), want missing", got, err)
	}
	if _, err := repo.Rsvp(testEvent, replyAlice, "definitely"); err == nil {
		t.Error("an answer there is no such thing as was accepted")
	}

	// Called off: once, and then no more answers. Leaving still works.
	if changed, err := repo.Cancel(testEvent); err != nil || !changed {
		t.Fatalf("cancel: %v (err %v)", changed, err)
	}
	if changed, _ := repo.Cancel(testEvent); changed {
		t.Error("called off twice")
	}
	answer(replyAlice, constants.EventRsvpMaybe, constants.EventRsvpClosed)
	answer(replyCarol, "", constants.EventRsvpSaved)
	if got := read(replyCarol); got.CancelledAt == nil || got.ViewerRsvp != "" {
		t.Errorf("after it was called off: %+v", got)
	}

	// Saving its details again does not call it back on.
	if err := repo.Save(&EventDetails{NodeID: testEvent, TimezoneName: "Europe/Paris", Capacity: 5}); err != nil {
		t.Fatal(err)
	}
	if got := read(0); got.CancelledAt == nil || got.TimezoneName != "Europe/Paris" || got.Capacity != 5 || got.OnlineURL != "" {
		t.Errorf("after saving again: %+v", got)
	}
}

// The last place is given once, however many ask for it in the same moment.
func TestEventLastPlaceIsGivenOnce(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewEventsRepo(db)
	seedEvents(t, repo, 1)

	people := []int64{replyAlice, replyBob, replyCarol, 5}
	results := make([]string, len(people))

	var wait sync.WaitGroup
	for i, person := range people {
		wait.Add(1)
		go func(i int, person int64) {
			defer wait.Done()
			got, err := repo.Rsvp(testEvent, person, constants.EventRsvpGoing)
			if err != nil {
				got = "error: " + err.Error()
			}
			results[i] = got
		}(i, person)
	}
	wait.Wait()

	saved, full := 0, 0
	for _, result := range results {
		switch result {
		case constants.EventRsvpSaved:
			saved++
		case constants.EventRsvpFull:
			full++
		}
	}
	if saved != 1 || full != len(people)-1 {
		t.Errorf("one place, four askers: %v, want one saved and three full", results)
	}

	events, err := repo.Read([]int64{testEvent}, 0)
	if err != nil || events[testEvent].GoingCount != 1 {
		t.Errorf("%d going (err %v), want 1", events[testEvent].GoingCount, err)
	}
}

// The events that are one person's, and who is going to one.
func TestEventListsAndWhoIsGoing(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewEventsRepo(db)
	seedEvents(t, repo, 0)

	const (
		later   = 902 // further off than the picnic
		deleted = 903 // answered, then removed
	)
	personType := constants.NodeActorPersonEntityType
	eventType := constants.NodeActorEventEntityType

	for _, stmt := range []string{
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,starts_at,ends_at,created_at,updated_at) VALUES
		 (902,'` + eventType + `','actor-service','Later','later','followers',1,NOW() + INTERVAL 9 DAY,NOW() + INTERVAL 10 DAY,NOW(),NOW()),
		 (903,'` + eventType + `','actor-service','Gone','gone','public',1,NOW() + INTERVAL 3 DAY,NOW() + INTERVAL 4 DAY,NOW(),NOW())`,
		`UPDATE nodes SET deleted_at = NOW() WHERE id = 903`,
		// Alice looks after the picnic and is going. She went to last
		// year's. She said maybe to the one that is gone.
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,meta,created_at) VALUES
		 ('edge.actor.admin.v1',1,'` + personType + `',900,'` + eventType + `',NULL,NOW()),
		 ('edge.actor.follow.v1',1,'` + personType + `',900,'` + eventType + `','` + constants.EventRsvpGoingMeta + `',NOW() - INTERVAL 2 HOUR),
		 ('edge.actor.follow.v1',1,'` + personType + `',901,'` + eventType + `','` + constants.EventRsvpGoingMeta + `',NOW()),
		 ('edge.actor.follow.v1',1,'` + personType + `',903,'` + eventType + `','` + constants.EventRsvpMaybeMeta + `',NOW()),
		 ('edge.actor.follow.v1',2,'` + personType + `',900,'` + eventType + `','` + constants.EventRsvpMaybeMeta + `',NOW() - INTERVAL 1 HOUR),
		 ('edge.actor.follow.v1',4,'` + personType + `',900,'` + eventType + `',NULL,NOW())`,
		// Alice is invited to the later one. Bob's invitation to it lapsed.
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,meta,created_at,ends_at) VALUES
		 ('edge.actor.invite.v1',1,'` + personType + `',902,'` + eventType + `','` + constants.InviteMetaPending + `',NOW(),NOW() + INTERVAL 30 DAY),
		 ('edge.actor.invite.v1',2,'` + personType + `',902,'` + eventType + `','` + constants.InviteMetaPending + `',NOW(),NOW() - INTERVAL 1 DAY)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
	for _, id := range []int64{later, deleted} {
		if err := repo.Save(&EventDetails{NodeID: id, TimezoneName: "UTC"}); err != nil {
			t.Fatal(err)
		}
	}

	list := func(person int64, filter string) []int64 {
		t.Helper()
		ids, total, err := repo.ListFor(person, filter, 20, 0)
		if err != nil {
			t.Fatalf("%s: %v", filter, err)
		}
		if int(total) != len(ids) {
			t.Errorf("%s: counted %d, listed %d", filter, total, len(ids))
		}
		return ids
	}
	same := func(got []int64, want ...int64) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range want {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// What is gone is in no list. What is over is only in the past.
	if got := list(replyAlice, constants.EventListUpcoming); !same(got, testEvent) {
		t.Errorf("Alice, upcoming: %v, want the picnic", got)
	}
	if got := list(replyAlice, constants.EventListPast); !same(got, testEventOver) {
		t.Errorf("Alice, past: %v, want last year's", got)
	}
	if got := list(replyAlice, constants.EventListHosting); !same(got, testEvent) {
		t.Errorf("Alice, hosting: %v, want the picnic", got)
	}
	if got := list(replyAlice, constants.EventListInvited); !same(got, later) {
		t.Errorf("Alice, invited: %v, want the later one", got)
	}
	// An invitation that lapsed is not one to answer.
	if got := list(replyBob, constants.EventListInvited); len(got) != 0 {
		t.Errorf("Bob, invited: %v, want nothing", got)
	}
	if got := list(replyBob, constants.EventListHosting); len(got) != 0 {
		t.Errorf("Bob, hosting: %v, want nothing", got)
	}
	if _, _, err := repo.ListFor(replyAlice, "everything", 20, 0); err == nil {
		t.Error("a list there is no such thing as was given")
	}

	// Who is going, and who said maybe: Carol followed it saying nothing,
	// which counts as maybe.
	graph := NewGraphRepo(db)
	who := func(is string, isNot string) (map[int64]bool, uint64) {
		t.Helper()
		req := &graphpb.BrowseConnectionNodesRequest{
			NodeId:      testEvent,
			EdgeType:    constants.EdgeFollowEntityType,
			Direction:   constants.GraphQueryDirectionInbound,
			Limit:       50,
			EdgeMeta:    is,
			EdgeMetaNot: isNot,
		}
		nodes, err := graph.BrowseConnectionNodes(req, replyViewer(replyAlice))
		got := nodeIDs(t, nodes, err)
		total, err := graph.CountConnectionNodes(req, replyViewer(replyAlice))
		if err != nil {
			t.Fatal(err)
		}
		return got, total
	}

	if got, total := who(constants.EventRsvpGoingMeta, ""); !sameSet(got, replyAlice) || total != 1 {
		t.Errorf("going: %v, counted %d, want Alice alone", got, total)
	}
	if got, total := who("", constants.EventRsvpGoingMeta); !sameSet(got, replyBob, replyCarol) || total != 2 {
		t.Errorf("maybe: %v, counted %d, want Bob and Carol", got, total)
	}
}

// A group hosting events: said once, and listed for each viewer as that
// viewer may see the events.
func TestEventsHostedByAGroup(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewEventsRepo(db)
	seedEvents(t, repo, 0)

	const private = 902
	eventType := constants.NodeActorEventEntityType

	if _, err := db.Exec(`INSERT INTO nodes (id,type,service,name,alias,access,enabled,starts_at,ends_at,created_at,updated_at) VALUES
		 (902,'` + eventType + `','actor-service','Members evening','members','followers',1,NOW() + INTERVAL 5 DAY,NOW() + INTERVAL 6 DAY,NOW(),NOW())`); err != nil {
		t.Fatal(err)
	}

	// The group hosts all three: the picnic, last year's, and one only its
	// own followers may see, which Carol follows.
	for _, id := range []int64{testEvent, testEventOver, private} {
		if err := repo.Save(&EventDetails{NodeID: id, TimezoneName: "UTC", HostID: replyGroup}); err != nil {
			t.Fatalf("save %d: %v", id, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES ('edge.actor.follow.v1',4,'node.actor.person-service.person.v1',902,?,NOW())`, eventType); err != nil {
		t.Fatal(err)
	}

	// Its host is read back, and saying another does not change it.
	if err := repo.Save(&EventDetails{NodeID: testEvent, TimezoneName: "UTC", HostID: replyAlice}); err != nil {
		t.Fatal(err)
	}
	events, err := repo.Read([]int64{testEvent}, 0)
	if err != nil || events[testEvent].HostID != replyGroup {
		t.Errorf("the picnic's host: %d (err %v), want the group", events[testEvent].HostID, err)
	}
	var hostEdges int
	if err := db.QueryRow(`SELECT COUNT(*) FROM edges WHERE type = ? AND node_b_id = ?`, constants.EdgeActorHostEntityType, testEvent).Scan(&hostEdges); err != nil || hostEdges != 1 {
		t.Errorf("%d host edges (err %v), want 1", hostEdges, err)
	}

	hosted := func(viewer int64, filter string) map[int64]bool {
		t.Helper()
		ids, total, err := repo.ListHostedBy(replyGroup, replyViewer(viewer), filter, 20, 0)
		if err != nil {
			t.Fatalf("%s: %v", filter, err)
		}
		if int(total) != len(ids) {
			t.Errorf("%s: counted %d, listed %d", filter, total, len(ids))
		}
		got := map[int64]bool{}
		for _, id := range ids {
			got[id] = true
		}
		return got
	}

	// Bob follows neither the group nor the private event.
	if got := hosted(replyBob, constants.EventListUpcoming); !sameSet(got, testEvent) {
		t.Errorf("Bob, upcoming: %v, want the picnic alone", got)
	}
	// Alice follows the group and not the private event: following the
	// host is not following the event.
	if got := hosted(replyAlice, constants.EventListUpcoming); !sameSet(got, testEvent) {
		t.Errorf("Alice, upcoming: %v, want the picnic alone", got)
	}
	if got := hosted(replyCarol, constants.EventListUpcoming); !sameSet(got, testEvent, private) {
		t.Errorf("Carol, upcoming: %v, want the picnic and the members evening", got)
	}
	if got := hosted(replyBob, constants.EventListPast); !sameSet(got, testEventOver) {
		t.Errorf("Bob, past: %v, want last year's", got)
	}
	if got := hosted(0, constants.EventListUpcoming); !sameSet(got, testEvent) {
		t.Errorf("a visitor, upcoming: %v, want the picnic alone", got)
	}
	if _, _, err := repo.ListHostedBy(replyGroup, replyViewer(replyBob), constants.EventListInvited, 20, 0); err == nil {
		t.Error("a list a group has no such thing as was given")
	}
}

// What the hourly sweep asks of events: who to remind, which invitations
// have nothing left to answer, and which events are long over.
func TestEventSweepQueries(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewEventsRepo(db)
	seedEvents(t, repo, 0)

	const (
		soon      = 902 // starts in six hours
		long      = 903 // ended two months ago
		cancelled = 904 // starts in six hours, called off
	)
	personType := constants.NodeActorPersonEntityType
	eventType := constants.NodeActorEventEntityType

	for _, stmt := range []string{
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,starts_at,ends_at,created_at,updated_at) VALUES
		 (902,'` + eventType + `','actor-service','Soon','soon','public',1,NOW() + INTERVAL 6 HOUR,NOW() + INTERVAL 8 HOUR,NOW(),NOW()),
		 (903,'` + eventType + `','actor-service','Long ago','long-ago','public',1,NOW() - INTERVAL 61 DAY,NOW() - INTERVAL 60 DAY,NOW(),NOW()),
		 (904,'` + eventType + `','actor-service','Off','off','public',1,NOW() + INTERVAL 6 HOUR,NOW() + INTERVAL 8 HOUR,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,meta,created_at) VALUES
		 ('edge.actor.follow.v1',1,'` + personType + `',902,'` + eventType + `','` + constants.EventRsvpGoingMeta + `',NOW()),
		 ('edge.actor.follow.v1',2,'` + personType + `',902,'` + eventType + `','` + constants.EventRsvpMaybeMeta + `',NOW())`,
		// Invited: Carol to the one that is soon, Bob to last year's, Dave
		// to the one called off.
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,meta,created_at,created_by,ends_at) VALUES
		 ('edge.actor.invite.v1',4,'` + personType + `',902,'` + eventType + `','` + constants.InviteMetaPending + `',NOW(),1,NOW() + INTERVAL 30 DAY),
		 ('edge.actor.invite.v1',2,'` + personType + `',901,'` + eventType + `','` + constants.InviteMetaPending + `',NOW(),1,NOW() + INTERVAL 30 DAY),
		 ('edge.actor.invite.v1',5,'` + personType + `',904,'` + eventType + `','` + constants.InviteMetaPending + `',NOW(),1,NOW() + INTERVAL 30 DAY)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
	for _, id := range []int64{soon, long, cancelled} {
		if err := repo.Save(&EventDetails{NodeID: id, TimezoneName: "UTC"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.Cancel(cancelled); err != nil {
		t.Fatal(err)
	}

	// Who answered, and who is going.
	if everybody, err := repo.AttendeeIDs(soon, false); err != nil || len(everybody) != 2 {
		t.Errorf("answered the one that is soon: %v (err %v), want two", everybody, err)
	}
	if going, err := repo.AttendeeIDs(soon, true); err != nil || len(going) != 1 || going[0] != replyAlice {
		t.Errorf("going to the one that is soon: %v (err %v), want Alice", going, err)
	}

	// Reminders: the one that starts within a day and is still on. Not the
	// picnic (tomorrow, and a little more), not the one called off. And
	// only once.
	day := constants.EventReminderBefore
	due, err := repo.ClaimReminders(day-time.Hour, 50)
	if err != nil || len(due) != 1 || due[0] != soon {
		t.Errorf("to remind: %v (err %v), want the one that is soon", due, err)
	}
	if again, err := repo.ClaimReminders(day-time.Hour, 50); err != nil || len(again) != 0 {
		t.Errorf("reminded twice: %v (err %v)", again, err)
	}

	// Invitations to events nobody can go to: last year's and the one
	// called off. Carol's, to an event still to come, stays.
	gone, err := NewInvitesRepo(db).SweepClosedEvents()
	if err != nil || len(gone) != 2 {
		t.Fatalf("invitations removed: %+v (err %v), want two", gone, err)
	}
	var left, carols int
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(node_a_id = 4), 0) FROM edges WHERE type = ?`, constants.EdgeActorInviteEntityType).Scan(&left, &carols); err != nil || left != 1 || carols != 1 {
		t.Errorf("%d invitations left, %d of them Carol's (err %v), want hers alone", left, carols, err)
	}
	if again, err := NewInvitesRepo(db).SweepClosedEvents(); err != nil || len(again) != 0 {
		t.Errorf("a second sweep removed %d (err %v)", len(again), err)
	}

	// Long over: the one that ended two months ago. Not last year's, which
	// in this fixture ended two days ago.
	over, err := repo.OverSince(time.Now().Add(-constants.EventArchiveAfter), 50)
	if err != nil || len(over) != 1 || over[0] != long {
		t.Errorf("to archive: %v (err %v), want the one long ago", over, err)
	}
	if changed, err := NewNodesRepo(db).ArchiveActor(long); err != nil || !changed {
		t.Fatalf("archive: %v (err %v)", changed, err)
	}
	if over, _ := repo.OverSince(time.Now().Add(-constants.EventArchiveAfter), 50); len(over) != 0 {
		t.Errorf("archived, and still to archive: %v", over)
	}
}

// An event takes no handle: two can be called the same thing, and neither
// takes the name from a group or a person, who still cannot share one.
func TestEventsTakeNoHandle(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)

	insert := func(id int64, nodeType string, alias string) error {
		_, err := db.Exec(`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES (?,?,?,?,?,'public',1,NOW(),NOW())`,
			id, nodeType, "actor-service", alias, alias)
		return err
	}

	if err := insert(910, constants.NodeActorEventEntityType, "birthday"); err != nil {
		t.Fatalf("the first event: %v", err)
	}
	if err := insert(911, constants.NodeActorEventEntityType, "birthday"); err != nil {
		t.Errorf("a second event called the same: %v", err)
	}
	// An event called what a person is called takes nothing from them.
	if err := insert(912, constants.NodeActorEventEntityType, "alice"); err != nil {
		t.Errorf("an event named as a person is: %v", err)
	}
	// And a group may still take a name an event has.
	if err := insert(913, "node.actor.group-service.group.v1", "birthday"); err != nil {
		t.Errorf("a group named as an event is: %v", err)
	}
	// Handles are still one namespace for everything else.
	if err := insert(914, "node.actor.group-service.group.v1", "alice"); err == nil {
		t.Error("a group took a person's handle")
	}
}
