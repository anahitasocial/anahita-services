package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	libs_constants "anahita-libs/constants"
)

// seedFilePath is the shipped default read by SeedDefaultClients at
// boot, relative to this package.
const seedFilePath = "../../../auth-defaults/oauth_seed_clients.json"

func loadSeedDoc(t *testing.T) seedClientsDocument {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(seedFilePath))
	if err != nil {
		t.Fatalf("reading seed file: %v", err)
	}
	var doc seedClientsDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing seed file: %v", err)
	}
	if len(doc.Clients) == 0 {
		t.Fatal("seed file defines no clients")
	}
	return doc
}

// TestSeedClientScopesAreKnown catches a typo in the shipped JSON
// before it reaches a container. An unregistered scope on a client row
// is invisible until authorize time, where it rejects the entire
// request with an opaque invalid_scope for real users.
func TestSeedClientScopesAreKnown(t *testing.T) {
	for _, c := range loadSeedDoc(t).Clients {
		if len(c.Scopes) == 0 {
			t.Errorf("client %q registers no scopes", c.ClientID)
		}
		for _, scope := range c.Scopes {
			if !libs_constants.OAuthScopeExists(scope) {
				t.Errorf("client %q registers unknown scope %q", c.ClientID, scope)
			}
		}
	}
}

// TestSeedClientsAreSeedable mirrors the rejections seedSingleClient
// makes at boot, so a bad seed file fails in CI rather than crashing
// auth-service on startup.
func TestSeedClientsAreSeedable(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range loadSeedDoc(t).Clients {
		if c.ClientID == "" {
			t.Error("a seeded client has an empty client_id")
		}
		if seen[c.ClientID] {
			t.Errorf("duplicate client_id %q — the second would be skipped as already-existing", c.ClientID)
		}
		seen[c.ClientID] = true

		// Confidential clients need a secret that can't be committed;
		// seedSingleClient hard-rejects them.
		if c.Confidential {
			t.Errorf("client %q is confidential; seeding rejects those", c.ClientID)
		}
		if len(c.RedirectURIs) == 0 {
			t.Errorf("client %q registers no redirect URIs", c.ClientID)
		}
	}
}

// TestMobileClientCannotAdministerOAuth pins a deliberate security
// decision: possession of oauth:write is equivalent to owning the
// identity provider, since its holder can add a redirect URI to any
// client and harvest authorization codes for every person on the site.
// It is confined to the one client that can be deactivated centrally.
func TestMobileClientCannotAdministerOAuth(t *testing.T) {
	for _, c := range loadSeedDoc(t).Clients {
		if c.ClientID != "anahita-mobile" {
			continue
		}
		for _, scope := range c.Scopes {
			if scope == libs_constants.OAuthScopeOAuthRead || scope == libs_constants.OAuthScopeOAuthWrite {
				t.Errorf("anahita-mobile registers %q; OAuth administration belongs to the web client only", scope)
			}
		}
		return
	}
	t.Skip("no anahita-mobile client in the seed file")
}
