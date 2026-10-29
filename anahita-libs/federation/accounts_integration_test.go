package federation

import (
	"database/sql"
	"errors"
	"os"
	"sort"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// The refused accounts against MySQL, with the schema of schema/init.sql:
//
//	FEDERATION_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./federation/ -run RefusedAccounts -v
func TestRefusedAccountsAreKeptByWhatDoesNotChange(t *testing.T) {
	dsn := os.Getenv("FEDERATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set FEDERATION_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`DELETE FROM federation_account_blocks`); err != nil {
		t.Fatal(err)
	}

	accounts := NewRefusedAccounts(db)

	bob, err := accounts.Add(&RefusedAccount{
		Network: NetworkActivityPub, Identifier: "https://example.net/users/bob",
		Handle: "@bob@example.net", Name: "Bob", Reason: "Spam", CreatedBy: 1,
	})
	if err != nil || bob.ID == 0 {
		t.Fatalf("refusing Bob: %+v %v", bob, err)
	}
	// A DID is kept in lower case, and found however it is written.
	if _, err := accounts.Add(&RefusedAccount{
		Network: NetworkATProto, Identifier: " did:plc:ABC123 ",
		Handle: "eve.bsky.social", Name: "Eve", Reason: "Harassment", CreatedBy: 1,
	}); err != nil {
		t.Fatal(err)
	}

	if !accounts.IsRefused(NetworkActivityPub, "https://example.net/users/bob") ||
		!accounts.IsRefused(NetworkATProto, "did:plc:abc123") ||
		!accounts.IsRefused(NetworkATProto, "DID:PLC:ABC123") {
		t.Error("a refused account is not found refused")
	}
	// Only on its own network, and only itself.
	if accounts.IsRefused(NetworkATProto, "https://example.net/users/bob") ||
		accounts.IsRefused(NetworkActivityPub, "https://example.net/users/bobby") ||
		accounts.IsRefused(NetworkActivityPub, "") {
		t.Error("an account not on the list is found refused")
	}
	if got := accounts.Refused(NetworkATProto); len(got) != 1 || got[0] != "did:plc:abc123" {
		t.Errorf("refused on Bluesky: %v", got)
	}

	// Twice is once.
	if _, err := accounts.Add(&RefusedAccount{Network: NetworkActivityPub, Identifier: "https://example.net/users/bob", CreatedBy: 2}); !errors.Is(err, ErrAlreadyRefused) {
		t.Errorf("refused twice: %v", err)
	}

	// Searched by name, handle and reason.
	for search, want := range map[string]int{"": 2, "bob": 1, "bsky": 1, "harass": 1, "nobody": 0, "%": 0} {
		page, total, err := accounts.Browse(search, 50, 0)
		if err != nil || total != want || len(page) != want {
			t.Errorf("searching %q: %d of %d (%v)", search, len(page), total, err)
		}
	}
	page, _, _ := accounts.Browse("", 50, 0)
	names := []string{page[0].Name, page[1].Name}
	sort.Strings(names)
	if names[0] != "Bob" || names[1] != "Eve" || page[0].Reason == "" {
		t.Errorf("listed %+v", page)
	}

	// Read, and let back.
	read, err := accounts.Read(bob.ID)
	if err != nil || read == nil || read.Handle != "@bob@example.net" {
		t.Errorf("read %+v (%v)", read, err)
	}
	if err := accounts.Delete(bob.ID); err != nil {
		t.Fatal(err)
	}
	if accounts.IsRefused(NetworkActivityPub, "https://example.net/users/bob") {
		t.Error("still refused after being let back")
	}
	if gone, _ := accounts.Read(bob.ID); gone != nil {
		t.Errorf("still on the list: %+v", gone)
	}
}
