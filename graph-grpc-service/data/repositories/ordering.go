package repositories

import (
	"log"
	"regexp"
)

// ORDER BY cannot be parameterised — a placeholder is a value, and a sort
// column is an identifier — so an ordering that reaches a query has been
// concatenated into it. That makes it the one piece of a request that must be
// checked against a list of things we already know rather than escaped.
//
// The HTTP services do check: their request structs carry
// `validate:"oneof=created_on modified_on enabled"` and the live API rejects
// anything else with a 400. This is the second line, and it exists because
// two paths go around the first:
//
//   - graph-grpc receives an ordering over gRPC and does not re-validate it.
//     Any service on the internal network, or anything that can reach the
//     port, supplies it directly.
//   - anahita-libs/requests.Browse declares Ordering with NO validate tag at
//     all. Nothing orders by it today, so it is harmless — but the next
//     handler to bind that struct and pass it to a repository that sorts
//     would inherit an injection with nothing in the diff to suggest it.
//
// Rejecting rather than escaping is deliberate. Escaping an identifier means
// re-implementing MySQL's quoting rules, and a sort column is drawn from a
// list the code already knows.

// orderableColumn matches a bare column reference: an optional table alias
// and a name. Nothing else — no functions, no commas, no parentheses, no
// whitespace, so nothing that could carry a second expression.
var orderableColumn = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

// safeOrderColumn returns ordering when it is a plain column reference, and
// "" otherwise. A caller receiving "" should fall back to its own default
// rather than pass anything through.
//
// The rejection is logged: a value reaching here that does not pass means
// either an attack or a caller sending something the API layer would have
// refused, and both are worth seeing.
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
