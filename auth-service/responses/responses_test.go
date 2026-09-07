package responses

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"auth/data/entities"
)

// Response mappers decide what leaves the service. Two things are worth
// pinning: that they do not leak secret material a caller should never see,
// and that collection mappers return an empty array rather than null — the
// settings UI maps over them directly and null is a runtime error there.

// ---------------------------------------------------------------------
// What must never leave
// ---------------------------------------------------------------------

// The token body IS the credential in a reset or invite link. A response shape
// that serialises it hands anyone who can read the API a working link.
func TestInviteResponseDoesNotLeakTheTokenBody(t *testing.T) {
	token := entities.NewToken()
	token.ID = 7
	token.Body = strings.Repeat("a", 128)
	token.RecipientEmail = "invitee@example.com"
	token.Status = "pending"
	token.CreatedAt = time.Now()

	r := &InviteResponse{}
	encoded, err := json.Marshal(r.MapInviteItem(token))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(encoded), token.Body) {
		t.Errorf("the invite token body is exposed in the response:\n%s", encoded)
	}
	if !strings.Contains(string(encoded), "invitee@example.com") {
		t.Error("the recipient is missing, so the admin list would be useless")
	}
}

func TestTokenResponseExposesOnlyTheRecipient(t *testing.T) {
	token := entities.NewToken()
	token.Body = strings.Repeat("b", 128)
	token.RecipientEmail = "ada@example.com"

	encoded, err := json.Marshal((&TokenResponse{}).MapToken(token))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(encoded), token.Body) {
		t.Errorf("the token body is exposed:\n%s", encoded)
	}
}

// A client secret is shown exactly once, at creation or rotation. Every other
// mapping of the same entity must omit it — otherwise a list endpoint hands
// out every client's credentials.
func TestOAuthClientSecretIsOnlyExposedOnCreation(t *testing.T) {
	client := &entities.OAuthClient{
		ID:           1,
		ClientID:     "anahita-web",
		Name:         "Anahita Web",
		SecretHash:   "$2a$10$super-secret-hash-value",
		RedirectURIs: []string{"http://localhost:3000/oauth/callback"},
		Active:       true,
	}

	r := &OAuthClientResponse{}

	listed, err := json.Marshal(r.MapClient(client))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(listed), "super-secret-hash-value") {
		t.Errorf("the stored client secret hash leaked into an ordinary response:\n%s", listed)
	}

	// The creation shape deliberately carries a secret — but the one passed
	// in, not the one on the entity.
	created, err := json.Marshal(r.MapClientWithSecret(client, "freshly-minted"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(created), "freshly-minted") {
		t.Error("the newly minted secret is missing from the creation response")
	}
	if strings.Contains(string(created), "super-secret-hash-value") {
		t.Error("the stored secret hash leaked alongside the minted one")
	}
}

// A private key must never leave the process. The signing-key responses are
// the one place an admin endpoint returns key rows at all.
func TestSigningKeyEntityHidesItsPrivateKey(t *testing.T) {
	key := &entities.OAuthSigningKey{
		KeyID:      "kid-1",
		Algorithm:  entities.SigningAlgRS256,
		PublicKey:  "-----BEGIN PUBLIC KEY-----\npub\n-----END PUBLIC KEY-----",
		PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\nPRIVATE\n-----END RSA PRIVATE KEY-----",
	}

	encoded, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(encoded), "PRIVATE") {
		t.Errorf("the signing private key is serialisable:\n%s", encoded)
	}
	// The public half is also tagged out — JWKS builds its own shape.
	if strings.Contains(string(encoded), "BEGIN PUBLIC KEY") {
		t.Errorf("the public key PEM is serialised raw:\n%s", encoded)
	}
}

// ---------------------------------------------------------------------
// Nil and empty handling
// ---------------------------------------------------------------------

// Collection mappers that return nil serialise as `null`, and the settings UI
// maps over the result directly.
func TestCollectionMappersReturnAnArrayNotNull(t *testing.T) {
	t.Run("passkeys", func(t *testing.T) {
		encoded, err := json.Marshal((&WebAuthnCredentialResponse{}).MapCredentials(nil))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(encoded) != "[]" {
			t.Errorf("got %s, want []", encoded)
		}
	})

	t.Run("invites", func(t *testing.T) {
		encoded, err := json.Marshal((&InviteResponse{}).MapInviteItems(nil))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(encoded) != "[]" {
			t.Errorf("got %s, want []", encoded)
		}
	})
}

// A nil entity maps to nil rather than to a zero-valued item, so a caller
// cannot mistake "not found" for "found, with empty fields".
func TestSingleMappersPropagateNil(t *testing.T) {
	if (&UserResponse{}).MapNode(nil) != nil {
		t.Error("UserResponse.MapNode invented a user from nil")
	}
	if (&UserResponse{}).MapNodeItem(nil) != nil {
		t.Error("UserResponse.MapNodeItem invented a user from nil")
	}
	if (&OAuthClientResponse{}).MapClient(nil) != nil {
		t.Error("OAuthClientResponse.MapClient invented a client from nil")
	}
	if (&WebAuthnCredentialResponse{}).MapCredential(nil) != nil {
		t.Error("WebAuthnCredentialResponse.MapCredential invented a credential from nil")
	}
}

// Transports feed a UI that iterates them.
func TestPasskeyTransportsAreNeverNull(t *testing.T) {
	cred := &entities.WebAuthnCredential{Nickname: "iCloud Keychain"}

	encoded, err := json.Marshal((&WebAuthnCredentialResponse{}).MapCredential(cred))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"transports":null`) {
		t.Errorf("transports serialised as null:\n%s", encoded)
	}
}

// Introspection has one job at the boundary: an absent or unknown token is
// reported inactive, never as an error and never as a partially populated
// record a caller might read fields off.
func TestIntrospectMapsNilToInactive(t *testing.T) {
	r := &OAuthIntrospectResponse{}

	item := r.MapTokenInfo(nil)
	if item == nil {
		t.Fatal("nil token info produced a nil response")
	}
	if item.Active {
		t.Error("an unknown token was reported active")
	}
	if item.Sub != "" || item.ClientID != "" || item.Scope != "" {
		t.Errorf("an inactive response carried claims: %+v", item)
	}

	if r.MapInactive().Active {
		t.Error("MapInactive returned active")
	}
}

// ---------------------------------------------------------------------
// OAuth redirects
// ---------------------------------------------------------------------

// The redirect URI is registered, but the code, state and error description
// are not necessarily safe to concatenate: state is client-supplied and comes
// straight back, so it must be escaped rather than pasted.
func TestBuildRedirectEscapesParameters(t *testing.T) {
	r := &OAuthAuthorizeResponse{}

	raw := r.BuildRedirect("https://app.example.com/callback", "code-123", "a b&c=d")

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("the redirect does not parse: %v", err)
	}

	q := parsed.Query()
	if q.Get("code") != "code-123" {
		t.Errorf("code = %q", q.Get("code"))
	}
	// Round-trips intact: an unescaped & would have split this into two
	// parameters and truncated the state.
	if q.Get("state") != "a b&c=d" {
		t.Errorf("state = %q, want it to survive escaping", q.Get("state"))
	}
}

// An empty state must be omitted rather than sent as state= — RFC 6749 says
// state is returned only if the client sent one.
func TestBuildRedirectOmitsAnEmptyState(t *testing.T) {
	raw := (&OAuthAuthorizeResponse{}).BuildRedirect("https://app.example.com/callback", "code-123", "")

	if strings.Contains(raw, "state") {
		t.Errorf("an empty state was included: %s", raw)
	}
}

// A registered redirect_uri may already carry a query string, in which case
// the separator must be & rather than a second ?.
func TestBuildRedirectAppendsToAnExistingQuery(t *testing.T) {
	raw := (&OAuthAuthorizeResponse{}).BuildRedirect(
		"https://app.example.com/callback?tenant=acme", "code-123", "xyz")

	if strings.Count(raw, "?") != 1 {
		t.Errorf("produced a malformed URL with two question marks: %s", raw)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("does not parse: %v", err)
	}
	q := parsed.Query()
	if q.Get("tenant") != "acme" {
		t.Error("the existing query parameter was lost")
	}
	if q.Get("code") != "code-123" {
		t.Error("the code was not appended")
	}
}

func TestBuildErrorRedirect(t *testing.T) {
	raw := (&OAuthAuthorizeResponse{}).BuildErrorRedirect(
		"https://app.example.com/callback", "xyz", "access_denied", "user denied the request")

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("does not parse: %v", err)
	}

	q := parsed.Query()
	if q.Get("error") != "access_denied" {
		t.Errorf("error = %q", q.Get("error"))
	}
	if q.Get("error_description") != "user denied the request" {
		t.Errorf("description = %q, want the spaces preserved", q.Get("error_description"))
	}
	if q.Get("state") != "xyz" {
		t.Error("state was dropped from the error redirect")
	}

	// An error redirect must never carry a code.
	if q.Has("code") {
		t.Error("an error redirect carried an authorization code")
	}
}
