package repositories

import "database/sql"

type Repositories struct {
	Cases CasesInterface
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Cases: NewCases(db),
	}
}
