package repositories

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"recommendation/constants"
	"recommendation/scoring"
)

// The signals against a real database.
//
//	docker run -d --name anahita-acl-test -e MYSQL_ALLOW_EMPTY_PASSWORD=1 \
//	  -e MYSQL_DATABASE=fresh -p 13309:3306 mysql:9.6
//	docker exec -i anahita-acl-test mysql -uroot fresh < migration-job/schema/init.sql
//	RECOMMENDATION_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -v
//
// THE RULE THIS GUARDS: nobody is suggested who may not be. Not somebody the
// viewer follows already, has asked to follow, has blocked or is blocked by,
// or turned down; not somebody who asked to be left out; not an account
// that is disabled, archived, deleted or unconfirmed.

const (
	viewer    = 1
	leaderB   = 2
	leaderC   = 3
	friendD   = 4 // B and C follow them
	blockerE  = 5 // blocked the viewer; B follows them
	archivedF = 6
	unverG    = 7 // never confirmed their email
	followerH = 8 // follows the viewer
	inviterI  = 9
	emptyJ    = 10 // no picture, nothing said
	hiddenL   = 12 // asked not to be suggested
	turnedM   = 13 // the viewer turned them down
	askedN    = 14 // the viewer has asked to follow them
	gardenQ   = 15 // posts in public about what the viewer posts about
	privateR  = 16 // posts about it for followers only
	likedS    = 17 // the viewer liked their post
	popularT  = 18
	featuredU = 19
	blockedV  = 21 // the viewer blocked them
	disabledW = 22
	deletedX  = 23
	inviteeY  = 24 // the viewer invited them
	groupP    = 20
	groupZ    = 25 // archived

	tagGarden = 100
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("RECOMMENDATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set RECOMMENDATION_TEST_DSN to run against MySQL")
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

func seed(t *testing.T, db *sql.DB) {
	t.Helper()

	const (
		person  = "'node.actor.person-service.person.v1','person-service'"
		group   = "'node.actor.group-service.group.v1','group-service'"
		note    = "'node.medium.text-service.note.v1','text-service'"
		aPerson = "'node.actor.person-service.person.v1'"
		follow  = "'edge.actor.follow.v1'"
	)

	// Everybody has something to say for themselves and was here yesterday,
	// unless a statement below says otherwise.
	people := ""
	for id := 1; id <= 24; id++ {
		if id == 11 || id == 20 {
			continue
		}
		people += fmt.Sprintf("(%d,%s,'P%d','p%d','public','About me',1,NOW() - INTERVAL 1 DAY,NOW(),NOW()),", id, person, id, id)
	}
	// Three more, to follow somebody lately.
	for id := 30; id <= 32; id++ {
		people += fmt.Sprintf("(%d,%s,'P%d','p%d','public','About me',1,NOW() - INTERVAL 1 DAY,NOW(),NOW()),", id, person, id, id)
	}
	people = people[:len(people)-1]

	accounts := ""
	for id := 1; id <= 32; id++ {
		if id == 11 || id == 20 || (id > 24 && id < 30) {
			continue
		}
		verified := "NOW()"
		if id == unverG {
			verified = "NULL"
		}
		accounts += fmt.Sprintf("(%d,'p%d@example.org',%s),", id, id, verified)
	}
	accounts = accounts[:len(accounts)-1]

	follows := func(pairs ...[2]int) string {
		rows := ""
		for _, pair := range pairs {
			kind := aPerson
			if pair[1] == groupP || pair[1] == groupZ {
				kind = "'node.actor.group-service.group.v1'"
			}
			rows += fmt.Sprintf("(%s,%d,%s,%d,%s,NOW()),", follow, pair[0], aPerson, pair[1], kind)
		}
		return rows[:len(rows)-1]
	}

	for _, stmt := range []string{
		`DELETE FROM recommendation_dismissals`,
		`DELETE FROM recommendation_preferences`,
		`DELETE FROM auth_accounts`,
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,body,enabled,last_visited_at,created_at,updated_at) VALUES ` + people,
		`INSERT INTO nodes (id,type,service,name,alias,access,body,enabled,created_at,updated_at) VALUES
		 (20,` + group + `,'Gardeners','gardeners','public','We garden',1,NOW(),NOW()),
		 (25,` + group + `,'Gone quiet','gone-quiet','public','We did',1,NOW(),NOW()),
		 (100,'node.tag.hashtag-service.hashtag.v1','hashtag-service','garden','garden','public','',1,NOW(),NOW())`,
		`INSERT INTO auth_accounts (node_id,email,email_verified_at) VALUES ` + accounts,

		`UPDATE nodes SET archived_at = NOW() WHERE id IN (6, 25)`,
		`UPDATE nodes SET body = '', avatar_filename = '' WHERE id = 10`,
		`UPDATE nodes SET enabled = 0 WHERE id = 22`,
		`UPDATE nodes SET deleted_at = NOW() WHERE id = 23`,
		`UPDATE nodes SET featured_at = NOW() WHERE id IN (19, 22)`,
		`UPDATE auth_accounts SET invited_by = 9 WHERE node_id = 1`,
		`UPDATE auth_accounts SET invited_by = 1 WHERE node_id = 24`,

		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES ` + follows(
			[2]int{viewer, leaderB}, [2]int{viewer, leaderC},
			[2]int{leaderB, friendD}, [2]int{leaderC, friendD},
			[2]int{leaderB, blockerE}, [2]int{leaderB, archivedF}, [2]int{leaderB, unverG},
			[2]int{leaderB, emptyJ}, [2]int{leaderC, emptyJ},
			[2]int{leaderB, hiddenL}, [2]int{leaderB, turnedM}, [2]int{leaderC, askedN},
			[2]int{leaderB, blockedV}, [2]int{leaderB, disabledW}, [2]int{leaderB, deletedX},
			[2]int{leaderB, groupP}, [2]int{leaderC, groupP}, [2]int{leaderB, groupZ},
			[2]int{followerH, viewer},
			[2]int{30, popularT}, [2]int{31, popularT}, [2]int{32, popularT},
		),
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.block.v1',5,` + aPerson + `,1,` + aPerson + `,NOW()),
		 ('edge.actor.block.v1',1,` + aPerson + `,21,` + aPerson + `,NOW()),
		 ('edge.actor.follow-request.v1',1,` + aPerson + `,14,` + aPerson + `,NOW())`,
		`INSERT INTO recommendation_preferences (person_id,suggest_people,suggest_groups,use_activity,discoverable,updated_at) VALUES
		 (12,1,1,1,0,NOW())`,
		`INSERT INTO recommendation_dismissals (person_id,actor_id,created_at) VALUES (1,13,NOW())`,

		// What the viewer wrote, what three others wrote, and what the
		// viewer liked.
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (200,` + note + `,'My beans #garden','public',1,1,` + aPerson + `,1,NOW(),NOW()),
		 (201,` + note + `,'Tomatoes #garden','public',1,15,` + aPerson + `,15,NOW(),NOW()),
		 (202,` + note + `,'For my followers #garden','followers',1,16,` + aPerson + `,16,NOW(),NOW()),
		 (203,` + note + `,'Something else','public',1,17,` + aPerson + `,17,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',200,'node.medium.text-service.note.v1',NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',201,'node.medium.text-service.note.v1',NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',202,'node.medium.text-service.note.v1',NOW()),
		 ('edge.base.voteup.v1',1,` + aPerson + `,203,'node.medium.text-service.note.v1',NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

func idsOf(hits []scoring.Hit) []int64 {
	ids := make([]int64, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func hitFor(hits []scoring.Hit, id int64) *scoring.Hit {
	for i := range hits {
		if hits[i].ID == id {
			return &hits[i]
		}
	}
	return nil
}

func TestSignalsFindPeopleForTheirReasons(t *testing.T) {
	db := testDB(t)
	seed(t, db)

	found, err := NewSignals(db).Find(viewer, constants.KindPerson, true, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprint(idsOf(found[constants.ReasonFollowsYou])); got != "[8]" {
		t.Errorf("follows you: %s", got)
	}
	if got := fmt.Sprint(idsOf(found[constants.ReasonFollowedByLeaders])); got != "[4]" {
		t.Errorf("followed by leaders: %s", got)
	}
	if hit := hitFor(found[constants.ReasonFollowedByLeaders], friendD); hit == nil || hit.Count != 2 || fmt.Sprint(hit.IDs) != "[2 3]" {
		t.Errorf("who follows D: %+v", hit)
	}
	if got := fmt.Sprint(idsOf(found[constants.ReasonInviter])); got != "[9 24]" {
		t.Errorf("inviter and invited: %s", got)
	}
	// Q posts in public about what the viewer posts about. R does so for
	// followers only, which is not the viewer's to be told.
	if got := fmt.Sprint(idsOf(found[constants.ReasonSharedInterest])); got != "[15]" {
		t.Errorf("shared interest: %s", got)
	}
	if hit := hitFor(found[constants.ReasonSharedInterest], gardenQ); hit == nil || fmt.Sprint(hit.IDs) != "[100]" {
		t.Errorf("what is shared with Q: %+v", hit)
	}
	if got := fmt.Sprint(idsOf(found[constants.ReasonYouEngaged])); got != "[17]" {
		t.Errorf("you engaged: %s", got)
	}
	if hit := hitFor(found[constants.ReasonPopular], popularT); hit == nil || hit.Count != 3 {
		t.Errorf("popular: %+v among %v", hit, idsOf(found[constants.ReasonPopular]))
	}
	if got := fmt.Sprint(idsOf(found[constants.ReasonFeatured])); got != "[19]" {
		t.Errorf("featured: %s", got)
	}

	// Whatever the reason, these are never found.
	never := map[int64]string{
		viewer: "the viewer", leaderB: "followed already", leaderC: "followed already",
		blockerE: "blocked the viewer", blockedV: "blocked by the viewer",
		archivedF: "archived", disabledW: "disabled", deletedX: "deleted",
		unverG: "never confirmed their email", emptyJ: "nothing to show",
		hiddenL: "asked to be left out", turnedM: "turned down", askedN: "asked to follow already",
		groupP: "a group, when people were asked for",
	}
	for reason, hits := range found {
		for _, hit := range hits {
			if why, bad := never[hit.ID]; bad {
				t.Errorf("%s found %d, who is %s", reason, hit.ID, why)
			}
		}
	}
}

func TestSignalsFindGroups(t *testing.T) {
	db := testDB(t)
	seed(t, db)

	found, err := NewSignals(db).Find(viewer, constants.KindGroup, true, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	hit := hitFor(found[constants.ReasonFollowedByLeaders], groupP)
	if hit == nil || hit.Count != 2 {
		t.Errorf("the group B and C follow: %+v", hit)
	}

	for reason, hits := range found {
		for _, hit := range hits {
			if hit.ID != groupP {
				t.Errorf("%s found %d among groups", reason, hit.ID)
			}
		}
	}
	// Only people follow back and are invited.
	if len(found[constants.ReasonFollowsYou]) != 0 || len(found[constants.ReasonInviter]) != 0 {
		t.Errorf("a group follows back or invited: %v", found)
	}
}

func TestSignalsLeaveOutWhatWasNotAskedFor(t *testing.T) {
	db := testDB(t)
	seed(t, db)
	signals := NewSignals(db)

	// Told not to use what the viewer does.
	found, err := signals.Find(viewer, constants.KindPerson, false, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(found[constants.ReasonSharedInterest]) != 0 || len(found[constants.ReasonYouEngaged]) != 0 {
		t.Errorf("activity was used: %v", found)
	}
	if len(found[constants.ReasonFollowedByLeaders]) == 0 {
		t.Errorf("who they follow is not activity, and was left out")
	}

	// Just joined: nothing worked out from who they follow.
	found, err = signals.Find(viewer, constants.KindPerson, true, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{constants.ReasonFollowsYou, constants.ReasonFollowedByLeaders, constants.ReasonYouEngaged} {
		if len(found[reason]) != 0 {
			t.Errorf("%s for somebody who has just joined: %v", reason, idsOf(found[reason]))
		}
	}
	if len(found[constants.ReasonInviter]) == 0 || len(found[constants.ReasonFeatured]) == 0 {
		t.Errorf("who invited them, and who is featured, were left out: %v", found)
	}

	// A kind there is not, and nobody.
	if found, _ := signals.Find(viewer, "event", true, false, time.Now()); len(found) != 0 {
		t.Errorf("events were suggested: %v", found)
	}
	if found, _ := signals.Find(0, constants.KindPerson, true, false, time.Now()); len(found) != 0 {
		t.Errorf("a visitor was suggested to: %v", found)
	}
}

func TestLastActiveIsTheLaterOfVisitingAndPosting(t *testing.T) {
	db := testDB(t)
	seed(t, db)

	if _, err := db.Exec(`UPDATE nodes SET last_visited_at = NOW() - INTERVAL 400 DAY WHERE id = 15`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE nodes SET last_visited_at = NULL WHERE id = 4`); err != nil {
		t.Fatal(err)
	}

	active, err := NewSignals(db).LastActive([]int64{4, 15, 18, 20})
	if err != nil {
		t.Fatal(err)
	}

	// Q has not been here for a year, and posted today.
	if since := time.Since(active[15]); since > time.Hour || since < -time.Hour {
		t.Errorf("Q was last active %v ago", since)
	}
	// D never visited or posted, and the group has no posts: each is as
	// alive as the day it was made, which is today.
	for _, id := range []int64{4, 18, 20} {
		if since := time.Since(active[id]); since > 48*time.Hour || since < -time.Hour {
			t.Errorf("%d was last active %v ago", id, since)
		}
	}
}

func TestPreferencesAndDismissalsAreKept(t *testing.T) {
	db := testDB(t)
	seed(t, db)
	repos := NewRepositories(db)

	// Nothing chosen is every choice at yes.
	prefs, err := repos.Preferences.Read(viewer)
	if err != nil || !reflect.DeepEqual(prefs, DefaultPreferences()) {
		t.Fatalf("nothing chosen read as %+v (%v)", prefs, err)
	}

	prefs.SuggestGroups = false
	prefs.Discoverable = false
	if err := repos.Preferences.Write(viewer, prefs); err != nil {
		t.Fatal(err)
	}
	prefs.UseActivity = false
	if err := repos.Preferences.Write(viewer, prefs); err != nil {
		t.Fatal(err)
	}
	read, err := repos.Preferences.Read(viewer)
	if err != nil || read.SuggestGroups || read.Discoverable || read.UseActivity || !read.SuggestPeople {
		t.Errorf("read back %+v (%v)", read, err)
	}

	// Their own weights, and whether to be suggested events.
	read.Weights = map[string]float64{constants.ReasonPopular: 7.5, constants.ReasonFollowsYou: 0}
	read.SuggestEvents = false
	if err := repos.Preferences.Write(viewer, read); err != nil {
		t.Fatal(err)
	}
	again, err := repos.Preferences.Read(viewer)
	if err != nil || again.SuggestEvents || again.Weights[constants.ReasonPopular] != 7.5 || len(again.Weights) != 2 {
		t.Errorf("weights read back %+v (%v)", again, err)
	}

	// Turned down twice is once, and can be taken back.
	for i := 0; i < 2; i++ {
		if err := repos.Dismissals.Add(viewer, friendD); err != nil {
			t.Fatal(err)
		}
	}
	if list, _ := repos.Dismissals.List(viewer); len(list) != 2 {
		t.Errorf("turned down: %v", list)
	}
	found, _ := repos.Signals.Find(viewer, constants.KindPerson, true, false, time.Now())
	if hitFor(found[constants.ReasonFollowedByLeaders], friendD) != nil {
		t.Errorf("D is still suggested after being turned down")
	}
	if err := repos.Dismissals.Remove(viewer, friendD); err != nil {
		t.Fatal(err)
	}
	found, _ = repos.Signals.Find(viewer, constants.KindPerson, true, false, time.Now())
	if hitFor(found[constants.ReasonFollowedByLeaders], friendD) == nil {
		t.Errorf("D is not suggested again after that was taken back")
	}

	// Somebody gone leaves nothing behind, either way round.
	if err := repos.Dismissals.Forget(turnedM); err != nil {
		t.Fatal(err)
	}
	if err := repos.Preferences.Forget(viewer); err != nil {
		t.Fatal(err)
	}
	if list, _ := repos.Dismissals.List(viewer); len(list) != 0 {
		t.Errorf("still turned down after they were removed: %v", list)
	}
	if read, _ := repos.Preferences.Read(viewer); !reflect.DeepEqual(read, DefaultPreferences()) {
		t.Errorf("choices kept after being forgotten: %+v", read)
	}
}

// Events: the ones still to come, that were not called off and that the
// viewer has not answered.
func TestSignalsFindEvents(t *testing.T) {
	db := testDB(t)
	seed(t, db)

	const (
		event     = "'node.actor.actor-service.event.v1','actor-service'"
		anEvent   = "'node.actor.actor-service.event.v1'"
		aPerson   = "'node.actor.person-service.person.v1'"
		aGroup    = "'node.actor.group-service.group.v1'"
		upcoming  = 40 // B and C are going; hosted by the group the viewer follows
		over      = 41 // ended yesterday
		calledOff = 42
		answered  = 43 // the viewer is going already
		plain     = 44 // nobody the viewer knows is going
	)

	for _, stmt := range []string{
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,starts_at,ends_at,created_at,updated_at) VALUES
		 (40,` + event + `,'Seed swap','','public',1,NOW() + INTERVAL 2 DAY,NOW() + INTERVAL 3 DAY,NOW() - INTERVAL 200 DAY,NOW()),
		 (41,` + event + `,'Last week','','public',1,NOW() - INTERVAL 3 DAY,NOW() - INTERVAL 1 DAY,NOW(),NOW()),
		 (42,` + event + `,'Called off','','public',1,NOW() + INTERVAL 2 DAY,NOW() + INTERVAL 3 DAY,NOW(),NOW()),
		 (43,` + event + `,'Going already','','public',1,NOW() + INTERVAL 2 DAY,NOW() + INTERVAL 3 DAY,NOW(),NOW()),
		 (44,` + event + `,'Somebody else\'s','','public',1,NOW() + INTERVAL 2 DAY,NOW() + INTERVAL 3 DAY,NOW(),NOW())`,
		`INSERT INTO events (node_id,timezone_name,cancelled_at) VALUES
		 (40,'UTC',NULL),(41,'UTC',NULL),(42,'UTC',NOW()),(43,'UTC',NULL),(44,'UTC',NULL)`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,` + aPerson + `,20,` + aGroup + `,NOW()),
		 ('edge.actor.host.v1',20,` + aGroup + `,40,` + anEvent + `,NOW()),
		 ('edge.actor.host.v1',20,` + aGroup + `,41,` + anEvent + `,NOW()),
		 ('edge.actor.host.v1',20,` + aGroup + `,42,` + anEvent + `,NOW()),
		 ('edge.actor.host.v1',20,` + aGroup + `,43,` + anEvent + `,NOW()),
		 ('edge.actor.follow.v1',2,` + aPerson + `,40,` + anEvent + `,NOW()),
		 ('edge.actor.follow.v1',3,` + aPerson + `,40,` + anEvent + `,NOW()),
		 ('edge.actor.follow.v1',2,` + aPerson + `,41,` + anEvent + `,NOW()),
		 ('edge.actor.follow.v1',2,` + aPerson + `,42,` + anEvent + `,NOW()),
		 ('edge.actor.follow.v1',1,` + aPerson + `,43,` + anEvent + `,NOW()),
		 ('edge.actor.follow.v1',30,` + aPerson + `,44,` + anEvent + `,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	found, err := NewSignals(db).Find(viewer, constants.KindEvent, true, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if hit := hitFor(found[constants.ReasonHostedByLeader], upcoming); hit == nil || fmt.Sprint(hit.IDs) != "[20]" {
		t.Errorf("hosted by a group the viewer follows: %+v among %v", hit, idsOf(found[constants.ReasonHostedByLeader]))
	}
	if hit := hitFor(found[constants.ReasonFollowedByLeaders], upcoming); hit == nil || hit.Count != 2 {
		t.Errorf("people the viewer follows are going: %+v", hit)
	}
	if hitFor(found[constants.ReasonPopular], plain) == nil {
		t.Errorf("an event people have answered lately is not found: %v", idsOf(found[constants.ReasonPopular]))
	}

	for reason, hits := range found {
		for _, hit := range hits {
			switch hit.ID {
			case over, calledOff, answered:
				t.Errorf("%s found event %d, which is over, called off or answered", reason, hit.ID)
			case upcoming, plain:
			default:
				t.Errorf("%s found %d among events", reason, hit.ID)
			}
		}
	}

	// A group is not hosted by anything, nor a person.
	people, _ := NewSignals(db).Find(viewer, constants.KindPerson, true, false, time.Now())
	if len(people[constants.ReasonHostedByLeader]) != 0 {
		t.Errorf("people hosted by a group: %v", people[constants.ReasonHostedByLeader])
	}
}
