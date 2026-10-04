package entities

import (
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"anahita-libs/data/entities"

	"auth/constants"
)

type Token struct {
	ID int64 `json:"id"`
	// RecipientEmail is where the token is sent, always.
	//
	// For an emailchange token that is the person's CURRENT address,
	// not the one they are moving to — the current inbox is what has to
	// approve the move, so a stolen session alone cannot repoint the
	// account. NewEmail carries the destination.
	RecipientEmail string `json:"email"`

	// NewEmail is the requested address. Set only on emailchange
	// tokens, empty on every other type.
	NewEmail string `json:"-"`

	// Code is the six digits the person is sent, on one-time-code tokens.
	// Empty on every other type.
	//
	// It exists only in memory, between GenerateCode and the message being
	// sent. It is never stored and never read back: the row holds CodeHash.
	//
	// json:"-" is load-bearing: this is the credential. Body is the row
	// identifier and is never transmitted for these types; Code is the
	// only value that leaves the building, and only through the mail or
	// SMS body.
	Code string `json:"-" db:"-"`

	// CodeHash is what is stored: an HMAC of the code, keyed with a secret
	// the database does not hold and salted with this token's Body.
	//
	// Not a plain hash. Six digits is a million possibilities, and anybody
	// who can read the table could try them all against a SHA-256 in a
	// second. With the key held elsewhere, the stored value says nothing.
	CodeHash string `json:"-"`

	// Channel is how the code was sent: constants.CodeChannelEmail, and
	// later CodeChannelSMS. Empty on every other token type.
	Channel string `json:"-"`

	// CodeAttempts counts failed submissions against this token. Capped
	// at constants.OTPMaxAttempts — six digits is a small keyspace, and
	// the cap is most of what keeps it out of reach.
	CodeAttempts int `json:"-"`

	Type         string           `json:"-"`
	Status       string           `json:"status" db:"-"`
	Body         string           `json:"-"`
	StartTime    time.Time        `json:"startTime"`
	ExpiredAfter int              `json:"expiredAfter"`
	IsUsed       bool             `json:"isUsed"`
	CreatedAt    time.Time        `json:"createdAt"`
	CreatedBy    int64            `json:"-"`
	Creator      *entities.Person `json:"creator" db:"-"`
	UpdatedAt    time.Time        `json:"updatedAt"`
	UpdatedBy    int64            `json:"-"`
	Editor       *entities.Person `json:"editor" db:"-"`
}

func NewToken() *Token {
	return &Token{
		ID:           0,
		StartTime:    time.Now(),
		ExpiredAfter: constants.TokenExpiryTime,
		Creator:      entities.NewPerson(),
		Editor:       entities.NewPerson(),
	}
}

// GenerateCode fills Code with a cryptographically random 6-digit value, and
// CodeHash with what is stored in its place. Body must be set first: it salts
// the hash.
//
// crypto/rand over a range of 10^OTPCodeLength, NOT rand.Int63() % 1e6.
// The modulo form biases the low end whenever the source range is not a
// multiple of the modulus, which shrinks the effective keyspace of a
// credential that only has 10^6 to begin with. rand.Int does rejection
// sampling internally and is uniform.
//
// Zero-padded, so "000042" stays six characters and compares against
// what the person typed.
func (token *Token) GenerateCode(key []byte) error {
	if len(key) == 0 {
		return fmt.Errorf("generate code: no key")
	}
	if token.Body == "" {
		return fmt.Errorf("generate code: the token has no body yet")
	}

	max := big.NewInt(1)
	for range constants.OTPCodeLength {
		max.Mul(max, big.NewInt(10))
	}

	n, err := cryptorand.Int(cryptorand.Reader, max)
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	token.Code = fmt.Sprintf("%0*d", constants.OTPCodeLength, n)
	token.CodeHash = token.hashCode(key, token.Code)

	return nil
}

// hashCode is HMAC-SHA256 over the token's body and the code. The body makes
// two tokens holding the same six digits store different values.
func (token *Token) hashCode(key []byte, code string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(token.Body))
	mac.Write([]byte{0})
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// IsCodeValid reports whether the submitted code matches, in constant
// time.
//
// subtle.ConstantTimeCompare rather than ==. A naive string comparison
// returns as soon as it finds a differing byte, so the time it takes
// leaks how much of the value was right.
//
// Returns false for a token with no stored hash, so a token type that
// carries no code can never be satisfied by submitting "".
func (token *Token) IsCodeValid(key []byte, submitted string) bool {
	if token.CodeHash == "" || len(key) == 0 {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(token.CodeHash),
		[]byte(token.hashCode(key, submitted)),
	) == 1
}

// HasAttemptsLeft reports whether this token may be tried again.
func (token *Token) HasAttemptsLeft() bool {
	return token.CodeAttempts < constants.OTPMaxAttempts
}

// Validate validates the token
func (token *Token) Validate() error {
	if token.RecipientEmail == "" {
		return fmt.Errorf("recipient email is required")
	}

	if token.Type == "" {
		return fmt.Errorf("type is required")
	}

	// An emailchange token with no destination is not a valid request,
	// and the failure mode of letting one through is confusing: the
	// confirm handler would have nothing to write and the person would
	// click a working link that did nothing.
	if token.Type == constants.TokenTypeEmailChange && token.NewEmail == "" {
		return fmt.Errorf("new email is required for an email change token")
	}

	if token.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}

	if token.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}

	// A signup confirmation issued for an APPLICATION has no creator and no
	// editor, and cannot have either: the applicant is not a person in this
	// system yet, which is the entire thing the approval queue is waiting to
	// decide. Every other token is issued by or for somebody who exists.
	//
	// Narrow on purpose, and by type rather than by "is it zero". Dropping the
	// checks outright would let a password reset or an invitation be stored
	// with no issuer, and those are the two where "who did this" is the
	// question asked afterwards.
	if token.Type != constants.TokenTypeSignupConfirm {
		if token.CreatedBy == 0 {
			return fmt.Errorf("creator is required")
		}

		if token.UpdatedBy == 0 {
			return fmt.Errorf("editor is required")
		}
	}

	if token.Status == "" {
		return fmt.Errorf("status is required")
	}

	return nil
}

// GenerateToken generates a cryptographically secure random token body
// using the system's CSPRNG. Returns an error if the system entropy
// source fails, which on a healthy OS is effectively impossible — but
// we surface it rather than silently producing a weak token.
func (token *Token) GenerateToken() error {
	b := make([]byte, 64)
	if _, err := cryptorand.Read(b); err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	token.Body = hex.EncodeToString(b)
	return nil
}

func (token *Token) Renew() {
	token.StartTime = time.Now()
}

func (token *Token) IsExpired() bool {
	duration := time.Since(token.StartTime)
	return duration.Seconds() > float64(token.ExpiredAfter)
}

func (token *Token) SetStatus() string {
	token.Status = constants.TokenStatusPending

	if token.IsExpired() {
		token.Status = constants.TokenStatusExpired
	}

	if token.IsUsed {
		token.Status = constants.TokenStatusUsed
	}

	return token.Status
}
