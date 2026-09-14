package repositories

import (
	"auth/data/entities"

	"anahita-libs/requests"
)

// TokensInterface is the interface for the tokens repository
type TokensInterface interface {
	Count(tokenType string) (uint64, error)
	CountPendingFrom(tokenType string, personID int64) (int, error)
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

	// InvalidateOutstanding marks every unused token of one type for one
	// address as used, returning how many were retired.
	//
	// Called when a new one is issued. Without it, asking for a second reset
	// email leaves the first link working: two people who each received one
	// can each use theirs, and a link forwarded or left in an inbox stays live
	// for its full window even after the owner has asked for a fresh one.
	//
	// Retired rather than deleted, so a link that is now dead reports itself
	// as used — which is the true answer — instead of as never having existed.
	InvalidateOutstanding(tokenType, email string) (int64, error)
}
