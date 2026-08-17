// Package signing defines the abstraction over JWT signing and
// verification. Two backends are provided:
//
//   - local: signs using a private key stored in oauth_signing_keys
//     and read out of the database. Used in development, CI, and any
//     deployment that doesn't have access to AWS KMS.
//
//   - kms: delegates signing to AWS KMS via the kms:Sign API. The
//     private key never leaves the HSM. Used in cloud-deployed
//     environments.
//
// Selection happens once at startup based on Config.OAuthSignerBackend.
// Handlers depend on the Signer/Verifier interfaces and are never aware
// of which backend is in use.
package signing

import "context"

// Signer signs JWT signing inputs (the base64url-encoded header and
// payload, joined with a ".") with the currently active signing key.
//
// A Signer instance is constructed for a single algorithm. Algorithm
// migrations (e.g. RS256 → ES256) happen via deployment of a new
// signer, not at runtime — this keeps the interface flat and avoids
// algorithm-confusion bugs. Existing keys signed under the old
// algorithm continue to verify until they age out of the JWKS document.
type Signer interface {
	// Algorithm returns the JWA algorithm name this signer produces
	// (e.g. "ES256", "RS256"). Used to set the "alg" header on JWTs.
	// Static for the lifetime of the signer.
	Algorithm() string

	// ActiveKeyID returns the kid of the key currently used for
	// signing. Callers stamp this onto the "kid" header of the JWT
	// before signing so verifiers can look up the right public key.
	//
	// May change between calls during a key rotation. Returns
	// ErrNoActiveSigningKey if the backend has no key configured.
	ActiveKeyID(ctx context.Context) (string, error)

	// Sign signs the given signing input with the active key.
	//
	// signingInput is the JWT signing input: base64url(header) + "." +
	// base64url(payload). The signer treats it as opaque bytes — no
	// JWT parsing happens here.
	//
	// Returns the raw signature bytes, not base64-encoded. The caller
	// (or the golang-jwt SigningMethod adapter) is responsible for
	// base64url-encoding the signature when assembling the final JWT
	// string.
	Sign(ctx context.Context, signingInput []byte) ([]byte, error)
}
