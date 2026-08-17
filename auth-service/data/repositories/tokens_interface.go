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
}
