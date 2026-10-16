package apidoc

import "testing"

// THE RULE THIS GUARDS: docs/api/anahita.json is every service's reference
// in one, as the files beside it are now, and docs/api/index.html is the
// page made of it. Run `make api_docs` after any of them changes.
func TestTheCombinedReference(t *testing.T) {
	CheckCombined(t, "../../docs/api", "../../docs/api/anahita.json")
	CheckPage(t, "../../docs/api", "anahita.json", "../../docs/api/index.html")
}
