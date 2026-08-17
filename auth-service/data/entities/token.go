package entities

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"anahita-libs/data/entities"

	"auth/constants"
)

type Token struct {
	ID             int64            `json:"id"`
	RecipientEmail string           `json:"email"`
	Type           string           `json:"-"`
	Status         string           `json:"status" db:"-"`
	Body           string           `json:"-"`
	StartTime      time.Time        `json:"startTime"`
	ExpiredAfter   int              `json:"expiredAfter"`
	IsUsed         bool             `json:"isUsed"`
	CreatedAt      time.Time        `json:"createdAt"`
	CreatedBy      int64            `json:"-"`
	Creator        *entities.Person `json:"creator" db:"-"`
	UpdatedAt      time.Time        `json:"updatedAt"`
	UpdatedBy      int64            `json:"-"`
	Editor         *entities.Person `json:"editor" db:"-"`
}

func NewToken() *Token {
	return &Token{
		StartTime:    time.Now(),
		ExpiredAfter: constants.TokenExpiryTime,
		Creator:      entities.NewPerson(),
		Editor:       entities.NewPerson(),
	}
}

// Validate validates the token
func (token *Token) Validate() error {
	if token.RecipientEmail == "" {
		return fmt.Errorf("recipient email is required")
	}

	if token.Type == "" {
		return fmt.Errorf("type is required")
	}

	if token.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}

	if token.CreatedBy == 0 {
		return fmt.Errorf("creator is required")
	}

	if token.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}

	if token.UpdatedBy == 0 {
		return fmt.Errorf("editor is required")
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
