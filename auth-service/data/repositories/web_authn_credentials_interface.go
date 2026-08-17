package repositories

import (
	"auth/data/entities"
)

// WebAuthnCredentialsInterface is the contract the handlers depend on.
//
// Handlers take this rather than *WebAuthnCredentialsRepository so the
// ceremony logic can be tested without a database. That matters more
// here than for most repositories: the interesting paths in
// oauth_webauthn.go are the ones a real database makes awkward to
// reach — a credential whose person_uuid disagrees with the
// authenticator's user handle, a person who was disabled between
// enrolment and sign-in, a sign counter that went backwards. Those are
// two-line stubs against an interface and a fixture wrestling match
// against sqlmock.
//
// Method set mirrors the concrete implementation exactly. Nothing is
// omitted, so swapping the Repos field type is the whole migration.
type WebAuthnCredentialsInterface interface {
	// Browse returns every credential enrolled by a person, newest
	// first. Used both by the settings list and, during a login
	// ceremony, to build the full credential set the library verifies
	// against.
	Browse(personID int64) ([]*entities.WebAuthnCredential, error)

	// Read fetches one credential by its public UUID. Returns
	// ErrWebAuthnCredentialNotFound when absent — sql.ErrNoRows does
	// not escape the implementation.
	ReadByID(id int64) (*entities.WebAuthnCredential, error)

	// ReadByCredentialID fetches by the raw authenticator-generated
	// credential ID. This is the login lookup: the browser supplies
	// this value in the assertion and it resolves to a person.
	ReadByCredentialID(credentialID []byte) (*entities.WebAuthnCredential, error)

	// Add persists a newly registered credential and returns the row
	// with its generated ID, UUID, and timestamps.
	Add(cred *entities.WebAuthnCredential) (*entities.WebAuthnCredential, error)

	// Edit updates the nickname along with authenticator state.
	// Credential ID and public key are not writable through this
	// method, by design.
	Edit(cred *entities.WebAuthnCredential) (*entities.WebAuthnCredential, error)

	// TouchAfterLogin records a successful assertion: refreshed
	// counter, clone warning, backup flags, and last_used_at.
	//
	// Separate from Edit because the assertion carries no nickname —
	// reusing Edit on the login path would blank the person's label on
	// every sign-in.
	TouchAfterLogin(cred *entities.WebAuthnCredential) error

	// Delete removes a single credential.
	Delete(id int64) error

	// DeleteByPersonID removes every credential a person holds and
	// returns how many rows went. Zero is a success, not an error.
	//
	// Called by admin revoke-all, password reset, and person
	// deletion. That last one must call it explicitly: there is no FK
	// to people (that table lives behind PersonGRPC, not in this
	// database), so no CASCADE does it automatically.
	DeleteByPersonID(personID int64) (int64, error)

	// Count returns how many credentials a person has enrolled, for
	// the per-person cap check.
	Count(personID int64) (int, error)
}

// Compile-time assertion that the concrete repository satisfies the
// interface.
//
// Worth the line: without it, a signature drift between the two
// surfaces as a confusing error at the Repos assignment in the
// container, some distance from the method that actually changed.
var _ WebAuthnCredentialsInterface = (*WebAuthnCredentialsRepository)(nil)
