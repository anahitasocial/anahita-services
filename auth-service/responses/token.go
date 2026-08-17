package responses

import (
	"auth/data/entities"
)

type Token struct {
	// ID             uuid.UUID               `json:"id"`
	RecipientEmail string `json:"recipient_email"`
	// Status         string                  `json:"status"`
	// StartTime      time.Time               `json:"startTime"`
	// ExpiredAfter   int                     `json:"expiredAfter"`
	// IsUsed         bool                    `json:"isUsed"`
	// CreatedAt      time.Time               `json:"createdAt"`
	// Creator        *responses.Person       `json:"creator"`
	// UpdatedAt      time.Time               `json:"updatedAt"`
	// Editor         *responses.Person       `json:"editor"`
}

type TokenResponse struct{}

func (r *TokenResponse) MapToken(token *entities.Token) *Token {
	return &Token{
		RecipientEmail: token.RecipientEmail,
	}
}
