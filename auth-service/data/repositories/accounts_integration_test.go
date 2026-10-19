package repositories

import (
	"fmt"
	"testing"
	"time"
)

// The accounts list against a real MySQL: what it is checking is arithmetic
// the database does — the latest of four dates, counts through joins — which
// a mock would only repeat back. See realTokens for AUTH_TEST_DSN.

// The cast, with ids far from anything else's.
const (
	accEmpty     = 9201 // signed up three years ago and did nothing
	accDormant   = 9202 // wrote once, four years ago
	accActive    = 9203 // old account, wrote last week
	accSignedIn  = 9204 // wrote nothing, but signed in yesterday and follows somebody
	accDeleted   = 9205 // waiting out the thirty days
	accPurged    = 9206 // a tombstone
	accGroup     = 9210 // has an administrator and a post
	accLostGroup = 9211 // nobody administers it, nothing in it
	accEvent     = 9212 // an event, with its administrator
	accGoneEvent = 9213 // an event deleted, waiting out the thirty days
	accGoneSite  = 9214 // a website deleted, waiting out the thirty days
)

func seedAccounts(t *testing.T) *accounts {
	t.Helper()

	db := realTokens(t).db

	clean := func() {
		for _, stmt := range []string{
			`DELETE FROM nodes WHERE id BETWEEN 9200 AND 9299`,
			`DELETE FROM edges WHERE node_a_id BETWEEN 9200 AND 9299 OR node_b_id BETWEEN 9200 AND 9299`,
			`DELETE FROM auth_accounts WHERE node_id BETWEEN 9200 AND 9299`,
			`DELETE FROM auth_logs WHERE created_by BETWEEN 9200 AND 9299`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				t.Fatalf("clean: %v", err)
			}
		}
	}
	clean()
	t.Cleanup(clean)

	const person = "'node.actor.person-service.person.v1','person-service'"
	const group = "'node.actor.group-service.group.v1','group-service'"
	const note = "'node.medium.text-service.note.v1','text-service'"
	const event = "'node.actor.actor-service.event.v1','actor-service'"
	const website = "'node.actor.actor-service.website.v1','actor-service'"

	for _, stmt := range []string{
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,person_type,follower_count,leader_count,created_at,updated_at,last_visited_at) VALUES
		 (9201,` + person + `,'Empty One','acc-empty','public',1,'registered',0,0,NOW() - INTERVAL 3 YEAR,NOW(),NOW() - INTERVAL 3 YEAR),
		 (9202,` + person + `,'Dormant One','acc-dormant','public',1,'registered',0,0,NOW() - INTERVAL 5 YEAR,NOW(),NULL),
		 (9203,` + person + `,'Active One','acc-active','public',1,'registered',3,2,NOW() - INTERVAL 5 YEAR,NOW(),NULL),
		 (9204,` + person + `,'Signed In','acc-signedin','public',1,'registered',0,1,NOW() - INTERVAL 5 YEAR,NOW(),NULL),
		 (9205,` + person + `,'Deleted One','acc-deleted','public',0,'registered',0,0,NOW() - INTERVAL 1 YEAR,NOW(),NULL),
		 (9206,` + person + `,'','acc-purged','public',0,'registered',0,0,NOW() - INTERVAL 1 YEAR,NOW(),NULL),
		 (9210,` + group + `,'Kept Group','acc-group','public',1,'',2,0,NOW() - INTERVAL 5 YEAR,NOW(),NULL),
		 (9211,` + group + `,'Lost Group','acc-lost','public',1,'',1,0,NOW() - INTERVAL 5 YEAR,NOW(),NULL),
		 (9212,` + event + `,'Kept Event','acc-event','public',1,'',1,0,NOW() - INTERVAL 1 MONTH,NOW(),NULL),
		 (9213,` + event + `,'Gone Event','acc-gone-event','public',0,'',1,0,NOW() - INTERVAL 1 MONTH,NOW(),NULL),
		 (9214,` + website + `,'Gone Site','acc-gone-site','public',0,'',1,0,NOW() - INTERVAL 1 MONTH,NOW(),NULL)`,
		`UPDATE nodes SET deleted_at = NOW() WHERE id IN (9205, 9206, 9213, 9214)`,
		`UPDATE nodes SET purged_at = NOW() WHERE id = 9206`,

		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,owner_id,created_by,created_at,updated_at) VALUES
		 (9220,` + note + `,'old','acc-n1','public',1,9202,9202,NOW() - INTERVAL 4 YEAR,NOW()),
		 (9221,` + note + `,'recent','acc-n2','public',1,9203,9203,NOW() - INTERVAL 7 DAY,NOW()),
		 (9222,` + note + `,'older','acc-n3','public',1,9203,9203,NOW() - INTERVAL 2 YEAR,NOW()),
		 (9223,` + note + `,'in the group','acc-n4','public',1,9210,9203,NOW() - INTERVAL 30 DAY,NOW())`,

		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.admin.v1',9203,'node.actor.person-service.person.v1',9210,'node.actor.group-service.group.v1',NOW()),
		 ('edge.actor.admin.v1',9203,'node.actor.person-service.person.v1',9212,'node.actor.actor-service.event.v1',NOW())`,

		`INSERT INTO auth_accounts (node_id,email,email_verified_at,onboarded_at) VALUES
		 (9201,'acc-empty@example.com',NULL,NULL),
		 (9202,'acc-dormant@example.com',NOW(),NOW()),
		 (9203,'acc-active@example.com',NOW(),NOW()),
		 (9204,'acc-signedin@example.com',NOW(),NULL),
		 (9205,'acc-deleted@example.com',NOW(),NOW())`,

		`INSERT INTO auth_logs (created_at,created_by,oauth_refresh_token_id,client_id) VALUES
		 (NOW() - INTERVAL 1 DAY,9204,0,'test'),
		 (NOW() - INTERVAL 2 YEAR,9204,0,'test')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	return &accounts{db: db}
}

// people is a filter for the test's own people, whatever else the database
// holds: every alias in the fixture starts with "acc-".
func people() *AccountFilter {
	return &AccountFilter{
		Kind:          AccountKindPerson,
		Search:        "acc-",
		ContentMax:    -1,
		DormantBefore: time.Now().AddDate(-2, 0, 0),
		Limit:         50,
	}
}

func accountIDs(found []*Account) string {
	var ids []int64
	for _, account := range found {
		ids = append(ids, account.ID)
	}
	return fmt.Sprint(ids)
}

func TestRealAccountsAreSortedIntoTiers(t *testing.T) {
	repo := seedAccounts(t)

	found, err := repo.Browse(people())
	if err != nil {
		t.Fatalf("browse: %v", err)
	}

	// Least recently active first. The tombstone is not listed.
	if got, want := accountIDs(found), fmt.Sprint([]int64{accDormant, accEmpty, accDeleted, accActive, accSignedIn}); got != want {
		t.Fatalf("listed %s, want %s", got, want)
	}

	byID := map[int64]*Account{}
	for _, account := range found {
		byID[account.ID] = account
	}

	for id, want := range map[int64]string{
		accEmpty:    AccountTierEmpty,
		accDormant:  AccountTierDormant,
		accActive:   AccountTierActive,
		accSignedIn: AccountTierActive,
	} {
		if got := byID[id].Tier; got != want {
			t.Errorf("account %d is %q, want %q (%+v)", id, got, want, byID[id])
		}
	}

	// What was written anywhere counts, including in a group.
	if got := byID[accActive].ContentCount; got != 3 {
		t.Errorf("the active account's content = %d, want 3", got)
	}

	// The latest of the four dates: a sign-in yesterday outweighs an account
	// that is five years old and has written nothing.
	if since := time.Since(byID[accSignedIn].LastActiveAt); since > 48*time.Hour {
		t.Errorf("the signed-in account was last active %s ago, want about a day", since)
	}

	if byID[accEmpty].EmailVerified || byID[accEmpty].Onboarded {
		t.Errorf("the empty account: %+v", byID[accEmpty])
	}
	if byID[accDeleted].State != AccountStateDeleted {
		t.Errorf("the deleted account's state = %q", byID[accDeleted].State)
	}

	total, err := repo.Count(people())
	if err != nil || total != 5 {
		t.Errorf("count = %d, %v; want 5", total, err)
	}
}

func TestRealAccountFilters(t *testing.T) {
	repo := seedAccounts(t)

	threeYearsAgo := time.Now().AddDate(-3, 0, -1)

	for _, tc := range []struct {
		name   string
		change func(*AccountFilter)
		want   []int64
	}{
		{"the empty tier", func(f *AccountFilter) { f.Tier = AccountTierEmpty }, []int64{accEmpty, accDeleted}},
		{"the dormant tier", func(f *AccountFilter) { f.Tier = AccountTierDormant }, []int64{accDormant}},
		{"never verified", func(f *AccountFilter) { f.OnlyUnverified = true }, []int64{accEmpty}},
		{"never onboarded", func(f *AccountFilter) { f.OnlyNotOnboarded = true }, []int64{accEmpty, accSignedIn}},
		{"nothing written", func(f *AccountFilter) { f.ContentMax = 0 }, []int64{accEmpty, accDeleted, accSignedIn}},
		{"last active before", func(f *AccountFilter) { f.LastActiveBefore = &threeYearsAgo }, []int64{accDormant}},
		{"created before", func(f *AccountFilter) { f.CreatedBefore = &threeYearsAgo }, []int64{accDormant, accActive, accSignedIn}},
		{"waiting to be purged", func(f *AccountFilter) { f.State = AccountStateDeleted }, []int64{accDeleted}},
		{"by name", func(f *AccountFilter) { f.Search = "Dormant" }, []int64{accDormant}},
		// A wildcard typed into the search box is a character, not a wildcard.
		{"a percent sign", func(f *AccountFilter) { f.Search = "acc-%" }, nil},
		{"newest first", func(f *AccountFilter) { f.Sort = AccountSortCreated; f.Descending = true; f.Limit = 2 },
			[]int64{accDeleted, accEmpty}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter := people()
			tc.change(filter)

			found, err := repo.Browse(filter)
			if err != nil {
				t.Fatalf("browse: %v", err)
			}
			if got, want := accountIDs(found), fmt.Sprint(tc.want); got != want {
				t.Errorf("listed %s, want %s", got, want)
			}
		})
	}
}

func TestRealGroupAccounts(t *testing.T) {
	repo := seedAccounts(t)

	groups := func() *AccountFilter {
		filter := people()
		filter.Kind = AccountKindGroup
		return filter
	}

	found, err := repo.Browse(groups())
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("listed %s, want both groups", accountIDs(found))
	}

	for _, account := range found {
		switch account.ID {
		case accGroup:
			// What was posted in it counts, whoever wrote it, and it was
			// last active when that was posted.
			if account.ContentCount != 1 || account.AdminCount != 1 || account.Tier != AccountTierActive {
				t.Errorf("the kept group: %+v", account)
			}
		case accLostGroup:
			// One follower is whoever started it; that alone is still empty.
			if account.AdminCount != 0 || account.Tier != AccountTierEmpty {
				t.Errorf("the lost group: %+v", account)
			}
		}
	}

	filter := groups()
	filter.OnlyWithoutAdmin = true
	found, err = repo.Browse(filter)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if got, want := accountIDs(found), fmt.Sprint([]int64{accLostGroup}); got != want {
		t.Errorf("groups without an administrator: %s, want %s", got, want)
	}
}

// Events are listed too, the deleted among them, so a super administrator can
// find one that no other list shows to restore it or remove it for good (the
// user, 2026-10-18).
func TestRealEventAccounts(t *testing.T) {
	repo := seedAccounts(t)

	events := func() *AccountFilter {
		filter := people()
		filter.Kind = AccountKindEvent
		return filter
	}

	found, err := repo.Browse(events())
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if got, want := accountIDs(found), fmt.Sprint([]int64{accEvent, accGoneEvent}); got != want {
		// Order is least recently active first; both were made a month ago.
		if got != fmt.Sprint([]int64{accGoneEvent, accEvent}) {
			t.Fatalf("listed %s, want both events", got)
		}
	}

	filter := events()
	filter.State = AccountStateDeleted
	found, err = repo.Browse(filter)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if got, want := accountIDs(found), fmt.Sprint([]int64{accGoneEvent}); got != want {
		t.Errorf("deleted events: %s, want %s", got, want)
	}

	filter = events()
	filter.OnlyWithoutAdmin = true
	found, err = repo.Browse(filter)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if got, want := accountIDs(found), fmt.Sprint([]int64{accGoneEvent}); got != want {
		t.Errorf("events without an administrator: %s, want %s", got, want)
	}
}

// Websites are listed too: every kind of profile here can be scheduled for
// deletion, and a super administrator finds it in this list (the user,
// 2026-10-18).
func TestRealWebsiteAccounts(t *testing.T) {
	repo := seedAccounts(t)

	filter := people()
	filter.Kind = AccountKindWebsite
	filter.State = AccountStateDeleted
	found, err := repo.Browse(filter)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if got, want := accountIDs(found), fmt.Sprint([]int64{accGoneSite}); got != want {
		t.Errorf("deleted websites: %s, want %s", got, want)
	}
}
