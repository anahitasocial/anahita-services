package repositories

import (
	"auth/data/entities"

	"anahita-libs/requests"
)

// TokensInterface is the interface for the tokens repository
type TokensInterface interface {
	// CountFrom and BrowseFrom are both scoped to one issuer, and the
	// personID is required rather than optional.
	//
	// They replace an unscoped Count and Browse that filtered on type
	// alone. Their one caller is the invitation list, where every row
	// carries a recipient's email address — so unscoped meant any
	// administrator opening the page read the address of everybody
	// every OTHER administrator had invited.
	//
	// No "0 means everyone" escape hatch, deliberately. This file
	// already carries a note about a filter parameter that was
	// accepted and ignored; a sentinel that silently widens the query
	// is the same trap wearing a different hat, and here it would
	// widen it back onto other people's correspondence.
	CountFrom(tokenType string, personID int64) (uint64, error)
	CountPendingFrom(tokenType string, personID int64) (int, error)
	BrowseFrom(tokenType string, personID int64, req *requests.Browse) ([]*entities.Token, error)
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

	// SpendCodeAttempt counts one submission against a one-time code and
	// reports whether there was an attempt left. ConsumeCode marks the code
	// used and reports whether this call did it. Both are single statements,
	// so submissions arriving together cannot share an attempt or a sign-in.
	SpendCodeAttempt(id int64) (bool, error)
	ConsumeCode(id, personID int64) (bool, error)
}
