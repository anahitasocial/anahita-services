package repositories

import (
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"auth/constants"
	"auth/data/entities"

	_ "github.com/go-sql-driver/mysql"
)

// Against a real MySQL, because what these check is what the database does
// with statements arriving together, which no mock can show.
//
// Skipped unless AUTH_TEST_DSN names a THROWAWAY database that has the schema
// (migration-job/schema/init.sql). Never the development one: rows are
// written to its tokens table.
//
//	AUTH_TEST_DSN='root@tcp(127.0.0.1:13309)/fresh?parseTime=true' go test ./data/repositories/ -run Real -v
func realTokens(t *testing.T) *tokens {
	t.Helper()

	dsn := os.Getenv("AUTH_TEST_DSN")
	if dsn == "" {
		t.Skip("AUTH_TEST_DSN is not set")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	return &tokens{db: db, sb: stmtBuilder}
}

var realCodeKey = []byte("a key the database never sees")

func addRealCode(t *testing.T, repo *tokens) *entities.Token {
	t.Helper()

	now := time.Now()
	token := entities.NewToken()
	token.Type = constants.TokenTypeLoginOTP
	token.Channel = constants.CodeChannelEmail
	token.Status = constants.TokenStatusPending
	token.RecipientEmail = "ada@example.com"
	token.ExpiredAfter = constants.TokenExpiryTimeOTP
	token.CreatedAt, token.UpdatedAt = now, now
	token.CreatedBy, token.UpdatedBy = 7, 7

	if err := token.GenerateToken(); err != nil {
		t.Fatal(err)
	}
	if err := token.GenerateCode(realCodeKey); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.Add(token)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(stored.ID) })

	return stored
}

func TestRealCodeIsStoredAsItsHashOnly(t *testing.T) {
	repo := realTokens(t)
	stored := addRealCode(t, repo)

	if stored.Code == "" {
		t.Fatal("Add returned no code to send")
	}

	read, err := repo.ReadByID(stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Code != "" {
		t.Errorf("the code came back from the database: %q", read.Code)
	}
	if read.Channel != constants.CodeChannelEmail || read.CodeAttempts != 0 {
		t.Errorf("channel %q attempts %d", read.Channel, read.CodeAttempts)
	}
	if !read.IsCodeValid(realCodeKey, stored.Code) {
		t.Error("the stored hash does not match the code that was issued")
	}
	if read.IsExpired() {
		t.Error("a code just issued reads as expired")
	}

	// No column anywhere in the row holds the six digits.
	var holding int
	err = repo.db.QueryRow(
		`SELECT COUNT(*) FROM tokens WHERE id = ? AND (code_hash = ? OR body = ?)`,
		stored.ID, stored.Code, stored.Code).Scan(&holding)
	if err != nil {
		t.Fatal(err)
	}
	if holding != 0 {
		t.Error("the code is in the row")
	}
}

// A token that is not a code stores NULL in the code columns, and reads back.
func TestRealOtherTokensLeaveTheCodeColumnsEmpty(t *testing.T) {
	repo := realTokens(t)

	now := time.Now()
	token := entities.NewToken()
	token.Type = constants.TokenTypePasswordReset
	token.Status = constants.TokenStatusPending
	token.RecipientEmail = "ada@example.com"
	token.CreatedAt, token.UpdatedAt = now, now
	token.CreatedBy, token.UpdatedBy = 7, 7
	if err := token.GenerateToken(); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.Add(token)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(stored.ID) })

	var nulls int
	err = repo.db.QueryRow(
		`SELECT COUNT(*) FROM tokens WHERE id = ? AND code_hash IS NULL AND channel IS NULL`,
		stored.ID).Scan(&nulls)
	if err != nil {
		t.Fatal(err)
	}
	if nulls != 1 {
		t.Error("a password reset token stored something in the code columns")
	}
	if stored.CodeHash != "" || stored.IsCodeValid(realCodeKey, "") {
		t.Error("a token with no code can be satisfied")
	}
}

func TestRealAttemptsArrivingTogetherStopAtTheCap(t *testing.T) {
	repo := realTokens(t)
	stored := addRealCode(t, repo)

	var wg sync.WaitGroup
	var mu sync.Mutex
	spent := 0

	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.SpendCodeAttempt(stored.ID)
			if err != nil {
				t.Errorf("SpendCodeAttempt: %v", err)
				return
			}
			if ok {
				mu.Lock()
				spent++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if spent != constants.OTPMaxAttempts {
		t.Errorf("%d attempts were allowed, want exactly %d", spent, constants.OTPMaxAttempts)
	}

	read, err := repo.ReadByID(stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.CodeAttempts != constants.OTPMaxAttempts || read.HasAttemptsLeft() {
		t.Errorf("recorded %d attempts", read.CodeAttempts)
	}
}

func TestRealCodeIsConsumedOnce(t *testing.T) {
	repo := realTokens(t)
	stored := addRealCode(t, repo)

	var wg sync.WaitGroup
	var mu sync.Mutex
	consumed := 0

	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.ConsumeCode(stored.ID, 7)
			if err != nil {
				t.Errorf("ConsumeCode: %v", err)
				return
			}
			if ok {
				mu.Lock()
				consumed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if consumed != 1 {
		t.Errorf("the code was consumed %d times, want exactly once", consumed)
	}

	// A used code takes no further attempts either.
	if ok, _ := repo.SpendCodeAttempt(stored.ID); ok {
		t.Error("an attempt was counted against a used code")
	}

	// And issuing a new code retires this address's live ones.
	live := addRealCode(t, repo)
	if _, err := repo.InvalidateOutstanding(constants.TokenTypeLoginOTP, "ada@example.com"); err != nil {
		t.Fatal(err)
	}
	read, err := repo.ReadByID(live.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !read.IsUsed {
		t.Error("InvalidateOutstanding left a live code")
	}
}

// The email-change flow has no browser session to remember its code, so it
// finds the pending one by whose it is.
func TestRealLiveCodeIsTheNewestUnusedOfItsType(t *testing.T) {
	repo := realTokens(t)

	if _, err := repo.InvalidateOutstanding(constants.TokenTypeLoginOTP, "ada@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReadLiveCode(constants.TokenTypeLoginOTP, "ada@example.com"); err == nil {
		t.Fatal("a live code was found where there is none")
	}

	older := addRealCode(t, repo)
	newer := addRealCode(t, repo)

	live, err := repo.ReadLiveCode(constants.TokenTypeLoginOTP, "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if live.ID != newer.ID {
		t.Errorf("got token %d, want the newer one, %d (older is %d)", live.ID, newer.ID, older.ID)
	}

	// Not another type's, and not another address's.
	if _, err := repo.ReadLiveCode(constants.TokenTypeEmailChangeCode, "ada@example.com"); err == nil {
		t.Error("a sign-in code was returned as an email-change code")
	}
	if _, err := repo.ReadLiveCode(constants.TokenTypeLoginOTP, "grace@example.com"); err == nil {
		t.Error("another address's code was returned")
	}

	if _, err := repo.ConsumeCode(newer.ID, 7); err != nil {
		t.Fatal(err)
	}
	live, err = repo.ReadLiveCode(constants.TokenTypeLoginOTP, "ada@example.com")
	if err != nil || live.ID != older.ID {
		t.Errorf("after the newer one was used: %v, %v", live, err)
	}
}
