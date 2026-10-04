package repositories

import (
	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// ReadByID returns a token by its token ID
func (repo *tokens) ReadByID(id int64) (*entities.Token, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"t.id": id})
	return repo.getOne(query)
}

// ReadLiveCode returns the newest unused token of one type for one address.
//
// Issuing a code retires the earlier ones of its type and address, so there is
// at most one. Expiry is not checked here: it is computed in Go, and the
// caller's verify step refuses an expired one.
func (repo *tokens) ReadLiveCode(tokenType, email string) (*entities.Token, error) {
	query := repo.getBaseSelectQuery().
		Where(sq.Eq{"t.type": tokenType, "t.recipient_email": email, "t.is_used": 0}).
		OrderBy("t.id DESC").
		Limit(1)
	return repo.getOne(query)
}

// ReadByBody returns a token by its body
func (repo *tokens) ReadByBody(body string) (*entities.Token, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"t.body": body})
	return repo.getOne(query)
}
