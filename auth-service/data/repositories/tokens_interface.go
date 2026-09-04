package repositories

import (
	"auth/data/entities"

	"anahita-libs/requests"
)

// TokensInterface is the interface for the tokens repository
type TokensInterface interface {
	Count(tokenType string, reg *requests.Browse) (uint64, error)
	Browse(tokenType string, reg *requests.Browse) ([]*entities.Token, error)
	ReadByID(id int64) (*entities.Token, error)
	ReadByBody(body string) (*entities.Token, error)
	Add(*entities.Token) (*entities.Token, error)
	Edit(*entities.Token) (*entities.Token, error)
	Delete(id int64) error

	// DeleteByRecipientEmail removes every pending token addressed to one
	// address, returning how many rows went.
	//
	// For account purge only. A tokens row holds the person's email address in
	// plain text alongside a live password-reset or email-change link, and both
	// outlived the account until the sweeper started calling this.
	DeleteByRecipientEmail(email string) (int64, error)
}
