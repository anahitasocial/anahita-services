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

// ReadByBody returns a token by its body
func (repo *tokens) ReadByBody(body string) (*entities.Token, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"t.body": body})
	return repo.getOne(query)
}
