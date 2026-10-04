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
		// Destination address, on emailchange tokens only. COALESCE
		// because the column is NULL on every other type and Token.NewEmail
		// is a plain string — a NULL would fail the scan for every password
		// reset and invite that passes through here.
		"COALESCE(t.new_email, '') AS new_email",
		"t.type",
		"t.body",
		// One-time-code tokens only. COALESCE for the same reason as
		// new_email: NULL on every other type, scanned into plain strings.
		"COALESCE(t.code_hash, '') AS code_hash",
		"t.code_attempts",
		"COALESCE(t.channel, '') AS channel",
		"t.started_at",
		"t.expired_after",
		"t.is_used",
		"t.created_at",
		"t.created_by",
		"t.updated_at",
		"t.updated_by",
	}

	// A person is a nodes row; there is no separate people table. The
	// display name is nodes.name, where it used to be a given_name and
	// family_name pair on the old profile table. Coalesced because a LEFT
	// JOIN that misses would otherwise fail the scan rather than yield an
	// empty creator.
	//
	// The creator's email is no longer selected. It was never read outside
	// this repository — the hydrated creator is serialised into the token's
	// JSON and nothing else touches it — and an address does not belong in a
	// payload that exists to name who issued an invitation.
	var creatorFields = []string{
		"COALESCE(cn.id, 0)",
		"COALESCE(cn.type, '')",
		"COALESCE(cn.name, '')",
	}

	var editorFields = []string{
		"COALESCE(en.id, 0)",
		"COALESCE(en.type, '')",
		"COALESCE(en.name, '')",
	}

	fields = append(fields, creatorFields...)
	fields = append(fields, editorFields...)

	return repo.sb.Select(fields...).From("tokens t").
		LeftJoin("nodes cn ON t.created_by = cn.id").
		LeftJoin("nodes en ON t.updated_by = en.id")
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
		&token.NewEmail,
		&token.Type,
		&token.Body,
		&token.CodeHash,
		&token.CodeAttempts,
		&token.Channel,
		&token.StartedAt,
		&token.ExpiredAfter,
		&token.IsUsed,
		&token.CreatedAt,
		&token.CreatedBy,
		&token.UpdatedAt,
		&token.UpdatedBy,
		&token.Creator.ID,
		&token.Creator.Type,
		&token.Creator.Name,
		&token.Editor.ID,
		&token.Editor.Type,
		&token.Editor.Name,
	); err != nil {
		return nil, err
	}

	token.SetStatus()

	return token, nil
}
