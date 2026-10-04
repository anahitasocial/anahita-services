package repositories

import (
	"log"
	"regexp"
)

// ORDER BY cannot be parameterised — a placeholder is a value, a sort column
// is an identifier — so an ordering that reaches a query has been
// concatenated into it. It has to be checked against what we already know
// rather than escaped.
//
// The API layer does check: UserBrowseRequest carries
// `validate:"oneof=id created_at person_type enabled"` and the live endpoint
// rejects anything else. This is the second line, for callers that never
// passed through it — see the fuller note in the graph service's copy.
//
// Rejecting rather than escaping is deliberate: escaping an identifier means
// re-implementing MySQL's quoting rules, and the valid values are a short
// list the code already has.

// orderableColumn matches a bare column reference and nothing else: no
// functions, commas, parentheses or whitespace, so nothing that could carry a
// second expression.
var orderableColumn = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

// safeOrderColumn returns ordering when it is a plain column reference, and
// "" otherwise, so the caller falls back to its own default.
func safeOrderColumn(ordering string) string {
	if ordering == "" {
		return ""
	}

	if !orderableColumn.MatchString(ordering) {
		log.Printf("repositories: refusing ordering %q — not a plain column reference", ordering)
		return ""
	}

	return ordering
}
