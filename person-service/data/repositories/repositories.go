package repositories

import (
	"database/sql"
)

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		MentionRelations: NewMentionRelationRepo(db),
	}
}

type Repositories struct {
	MentionRelations MentionRelationRepoInterface
}
