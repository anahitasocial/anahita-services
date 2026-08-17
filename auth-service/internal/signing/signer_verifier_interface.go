package signing

import "context"

// Verifier verifies JWT signatures using the public key identified by
// kid. It does not parse JWTs — the caller has already decoded the
// header (to extract kid and alg) and split out the signing input from
// the signature.
//
// In both backends, the verifier reads public keys from the
// oauth_signing_keys table. KMS-backed keys persist their public key
// locally at provisioning time so verifiers never make a KMS call on
// the hot path. This means downstream microservices can verify tokens
// with database access alone — no KMS IAM permissions required.
//
// Note: most microservices will not call Verifier directly. They use
// the anahita-libs OAuth middleware, which fetches public keys from
// auth-service's JWKS endpoint and caches them. Verifier is used by
// auth-service itself (e.g. validating refresh-token-bound access
// tokens during refresh) and by anything that needs first-party
// verification without a JWKS round-trip.
type Verifier interface {
	// Verify checks that signature is a valid signature over
	// signingInput, using the public key identified by kid.
	//
	// alg is the algorithm declared in the JWT header. The verifier
	// must reject if alg does not match the algorithm associated with
	// the kid — this prevents alg-confusion attacks where an attacker
	// swaps the alg header to coerce the verifier into using the wrong
	// primitive.
	//
	// Returns nil on a valid signature. Returns ErrUnknownKID if no
	// key matches kid, ErrAlgorithmMismatch if alg disagrees with the
	// key's declared algorithm, or ErrInvalidSignature if the
	// cryptographic check fails.
	Verify(ctx context.Context, kid string, alg string, signingInput, signature []byte) error
}
