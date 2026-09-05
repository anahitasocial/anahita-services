package entities

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"auth/constants"
)

// Token is the credential behind password reset, email change and invites.
// Its body is the only thing standing between a link in a mailbox and
// somebody else's account, and its status is computed in Go rather than
// stored — so both the randomness and the state machine are worth pinning.

func TestGenerateTokenProducesAUniqueHighEntropyBody(t *testing.T) {
	seen := map[string]bool{}

	for range 100 {
		token := NewToken()
		if err := token.GenerateToken(); err != nil {
			t.Fatalf("generate: %v", err)
		}

		// 64 random bytes, hex-encoded. The reset request validates
		// len=128, so a change here silently breaks that binding.
		if len(token.Body) != 128 {
			t.Fatalf("body is %d chars, want 128", len(token.Body))
		}
		if _, err := hex.DecodeString(token.Body); err != nil {
			t.Fatalf("body is not hex: %v", err)
		}
		if seen[token.Body] {
			t.Fatal("generated the same token body twice")
		}
		seen[token.Body] = true
	}
}

// SetStatus is what every gate in the reset and email-change handlers reads.
// Used beats expired beats pending: a spent link must never read as merely
// expired, because the handlers treat those as different failures and only
// one of them is terminal.
func TestSetStatusPrecedence(t *testing.T) {
	cases := []struct {
		name   string
		age    time.Duration
		ttl    int
		isUsed bool
		want   string
	}{
		{"fresh and unused", time.Minute, 15 * 60, false, constants.TokenStatusPending},
		{"past its window", time.Hour, 15 * 60, false, constants.TokenStatusExpired},
		{"used and still fresh", time.Minute, 15 * 60, true, constants.TokenStatusUsed},
		// Used wins. A link that was spent and then expired is spent.
		{"used and expired", time.Hour, 15 * 60, true, constants.TokenStatusUsed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := NewToken()
			token.StartTime = time.Now().Add(-tc.age)
			token.ExpiredAfter = tc.ttl
			token.IsUsed = tc.isUsed

			if got := token.SetStatus(); got != tc.want {
				t.Errorf("status = %q, want %q", got, tc.want)
			}
			if token.Status != tc.want {
				t.Errorf("Status field = %q, want %q", token.Status, tc.want)
			}
		})
	}
}

// The boundary itself. Expiry is computed from StartTime, which the tokens
// table fills with its own CURRENT_TIMESTAMP default rather than from Go —
// so a token whose window is read one second wrong is the failure mode to
// stay honest about.
func TestIsExpiredAtTheBoundary(t *testing.T) {
	token := NewToken()
	token.ExpiredAfter = 900

	token.StartTime = time.Now().Add(-899 * time.Second)
	if token.IsExpired() {
		t.Error("a token one second inside its window reads as expired")
	}

	token.StartTime = time.Now().Add(-901 * time.Second)
	if !token.IsExpired() {
		t.Error("a token one second past its window reads as live")
	}
}

func TestRenewRestartsTheWindow(t *testing.T) {
	token := NewToken()
	token.ExpiredAfter = 900
	token.StartTime = time.Now().Add(-time.Hour)

	if !token.IsExpired() {
		t.Fatal("setup: token should start expired")
	}

	token.Renew()

	if token.IsExpired() {
		t.Error("the token is still expired after Renew")
	}
}

// Validate is the last thing standing between a half-built token and the
// tokens table, and it runs inside both Add and Edit.
func TestValidateRejectsIncompleteTokens(t *testing.T) {
	complete := func() *Token {
		token := NewToken()
		token.RecipientEmail = "ada@example.com"
		token.Type = constants.TokenTypePasswordReset
		token.Status = constants.TokenStatusPending
		token.CreatedAt = time.Now()
		token.CreatedBy = 42
		token.UpdatedAt = time.Now()
		token.UpdatedBy = 42
		return token
	}

	if err := complete().Validate(); err != nil {
		t.Fatalf("a complete token was rejected: %v", err)
	}

	cases := []struct {
		name string
		make func(*Token)
		want string
	}{
		{"no recipient", func(tk *Token) { tk.RecipientEmail = "" }, "recipient email"},
		{"no type", func(tk *Token) { tk.Type = "" }, "type"},
		{"no status", func(tk *Token) { tk.Status = "" }, "status"},
		{"no creation time", func(tk *Token) { tk.CreatedAt = time.Time{} }, "created at"},
		{"no creator", func(tk *Token) { tk.CreatedBy = 0 }, "creator"},
		{"no update time", func(tk *Token) { tk.UpdatedAt = time.Time{} }, "updated at"},
		{"no editor", func(tk *Token) { tk.UpdatedBy = 0 }, "editor"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := complete()
			tc.make(token)

			err := token.Validate()
			if err == nil {
				t.Fatalf("accepted a token with %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// An emailchange token with no destination is a link that does nothing when
// clicked — the confirm handler would have no address to write.
func TestValidateRequiresADestinationOnEmailChangeTokens(t *testing.T) {
	token := NewToken()
	token.RecipientEmail = "ada@example.com"
	token.Type = constants.TokenTypeEmailChange
	token.Status = constants.TokenStatusPending
	token.CreatedAt = time.Now()
	token.CreatedBy = 42
	token.UpdatedAt = time.Now()
	token.UpdatedBy = 42

	if err := token.Validate(); err == nil {
		t.Fatal("accepted an email change token with no new address")
	}

	token.NewEmail = "new@example.com"
	if err := token.Validate(); err != nil {
		t.Errorf("rejected a complete email change token: %v", err)
	}

	// The same omission on any other type is fine — NewEmail is empty on
	// every token that is not an email change.
	token.Type = constants.TokenTypePasswordReset
	token.NewEmail = ""
	if err := token.Validate(); err != nil {
		t.Errorf("rejected a password reset token for having no new address: %v", err)
	}
}

// ---------------------------------------------------------------------
// One-time codes
// ---------------------------------------------------------------------
//
// Scaffolding for the login-OTP flow, committed but not yet wired to a
// route. Tested now because the properties are cheap to hold and expensive
// to discover missing: six digits is a small keyspace, and the two things
// keeping it out of reach are uniformity and the attempt cap.

func TestGenerateCodeIsSixDigitsAndZeroPadded(t *testing.T) {
	for range 200 {
		token := NewToken()
		if err := token.GenerateCode(); err != nil {
			t.Fatalf("generate code: %v", err)
		}

		if len(token.Code) != constants.OTPCodeLength {
			t.Fatalf("code %q is %d chars, want %d",
				token.Code, len(token.Code), constants.OTPCodeLength)
		}
		for _, r := range token.Code {
			if r < '0' || r > '9' {
				t.Fatalf("code %q contains a non-digit", token.Code)
			}
		}
	}
}

// A modulo-based generator biases the low end. Not a proof of uniformity —
// that is what crypto/rand.Int is for — but it does catch a generator that
// has collapsed onto a constant or lost its high digits.
func TestGenerateCodeVariesAcrossTheRange(t *testing.T) {
	seen := map[string]bool{}
	leading := map[byte]bool{}

	for range 500 {
		token := NewToken()
		if err := token.GenerateCode(); err != nil {
			t.Fatalf("generate code: %v", err)
		}
		seen[token.Code] = true
		leading[token.Code[0]] = true
	}

	if len(seen) < 450 {
		t.Errorf("only %d distinct codes in 500 draws", len(seen))
	}
	// Ten possible leading digits; seeing fewer than seven in 500 draws
	// means the top of the range is not being reached.
	if len(leading) < 7 {
		t.Errorf("only %d distinct leading digits, want the full range", len(leading))
	}
}

func TestIsCodeValid(t *testing.T) {
	token := NewToken()
	token.Code = "042195"

	if !token.IsCodeValid("042195") {
		t.Error("the correct code was refused")
	}
	if token.IsCodeValid("042196") {
		t.Error("a wrong code was accepted")
	}
	// Not a prefix match: a comparison that stops early would accept this.
	if token.IsCodeValid("0421") {
		t.Error("a truncated code was accepted")
	}

	// A token type that carries no code must never be satisfied by
	// submitting the empty string.
	empty := NewToken()
	if empty.IsCodeValid("") {
		t.Error("an empty submission satisfied a token with no code")
	}
}

func TestHasAttemptsLeftStopsAtTheCap(t *testing.T) {
	token := NewToken()

	for i := range constants.OTPMaxAttempts {
		token.CodeAttempts = i
		if !token.HasAttemptsLeft() {
			t.Fatalf("refused attempt %d of %d", i+1, constants.OTPMaxAttempts)
		}
	}

	token.CodeAttempts = constants.OTPMaxAttempts
	if token.HasAttemptsLeft() {
		t.Error("allowed an attempt past the cap")
	}
}
