package signing

import "errors"

// Sentinel errors. Callers can use errors.Is to check; the anahita-libs
// middleware maps these to 401 responses with appropriate WWW-Authenticate
// headers per RFC 6750.
var (
	// ErrNoActiveSigningKey is returned by Sign or ActiveKeyID when
	// the backend has no key configured. Local: oauth_signing_keys
	// table has no row matching the active filter. KMS: the configured
	// alias doesn't resolve. Either way, this is a startup
	// misconfiguration and should be treated as fatal.
	ErrNoActiveSigningKey = errors.New("signing: no active signing key")

	// ErrUnknownKID is returned by Verify when the kid in the JWT
	// header does not match any known signing key. This usually means
	// the token was signed by a different issuer, or by a key that's
	// been deleted (not just deactivated — deactivated keys remain in
	// the table for verification until they expire).
	ErrUnknownKID = errors.New("signing: unknown kid")

	// ErrAlgorithmMismatch is returned by Verify when the alg in the
	// JWT header does not match the algorithm associated with the
	// kid's key. Defends against alg-confusion attacks.
	ErrAlgorithmMismatch = errors.New("signing: algorithm mismatch")

	// ErrInvalidSignature is returned by Verify when the signature
	// fails the cryptographic check. The caller should treat this as
	// a 401 Unauthorized — the token is forged or corrupted.
	ErrInvalidSignature = errors.New("signing: invalid signature")

	// ErrUnsupportedAlgorithm is returned when the configured key uses
	// an algorithm the implementation cannot handle (e.g. a local-mode
	// key with alg="ES512" before that's wired up). Treat as a startup
	// error.
	ErrUnsupportedAlgorithm = errors.New("signing: unsupported algorithm")
)
