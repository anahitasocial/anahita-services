package repositories

import (
	"auth/data/entities"
	"context"

	sq "github.com/Masterminds/squirrel"
)

// getBaseSelectQuery returns the base select query for the tokens repository
func (repo *tokens) getBaseSelectQuery() sq.SelectBuilder {
	var fields = []string{
		"t.id",
		"t.recipient_email",
		"t.type",
		"t.body",
		"t.start_time",
		"t.expired_after",
		"t.is_used",
		"t.created_at",
		"t.created_by",
		"t.updated_at",
		"t.updated_by",
	}

	// There is no "people" table in this schema — a person is a nodes
	// row joined to its people_people row, which is how users.go
	// assembles one. id and type come from nodes; the name and email
	// columns live on people_people and are all nullable there, so each
	// is coalesced: a LEFT JOIN that misses would otherwise fail the
	// scan into Person's non-pointer string fields rather than yielding
	// an empty creator.
	var creatorFields = []string{
		"COALESCE(cn.id, 0)",
		"COALESCE(cn.type, '')",
		"COALESCE(cp.given_name, '')",
		"COALESCE(cp.family_name, '')",
		"COALESCE(cp.email, '')",
	}

	var editorFields = []string{
		"COALESCE(en.id, 0)",
		"COALESCE(en.type, '')",
		"COALESCE(ep.given_name, '')",
		"COALESCE(ep.family_name, '')",
		"COALESCE(ep.email, '')",
	}

	fields = append(fields, creatorFields...)
	fields = append(fields, editorFields...)

	return repo.sb.Select(fields...).From("tokens t").
		LeftJoin("nodes cn ON t.created_by = cn.id").
		LeftJoin("people_people cp ON cp.node_id = cn.id").
		LeftJoin("nodes en ON t.updated_by = en.id").
		LeftJoin("people_people ep ON ep.node_id = en.id")
}

// getOne returns one token based on the provided query
func (repo *tokens) getOne(query sq.SelectBuilder) (*entities.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	row := repo.db.QueryRowContext(ctx, q, args...)
	token := entities.NewToken()
	if err := row.Scan(
		&token.ID,
		&token.RecipientEmail,
		&token.Type,
		&token.Body,
		&token.StartTime,
		&token.ExpiredAfter,
		&token.IsUsed,
		&token.CreatedAt,
		&token.CreatedBy,
		&token.UpdatedAt,
		&token.UpdatedBy,
		&token.Creator.ID,
		&token.Creator.Type,
		&token.Creator.GivenName,
		&token.Creator.FamilyName,
		&token.Creator.Email,
		&token.Editor.ID,
		&token.Editor.Type,
		&token.Editor.GivenName,
		&token.Editor.FamilyName,
		&token.Editor.Email,
	); err != nil {
		return nil, err
	}

	token.SetStatus()

	return token, nil
}
