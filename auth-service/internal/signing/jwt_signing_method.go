package signing

import (
	"context"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

// jwtSigningMethod adapts a Signer to golang-jwt's jwt.SigningMethod
// interface so existing code that uses jwt.NewWithClaims and
// token.SignedString continues to work with one line changed.
//
// Verification is intentionally NOT implemented — Verify always returns
// an error. Verification goes through the Verifier interface (and, for
// downstream microservices, through the anahita-libs JWKS middleware).
// This split keeps the signing path narrow: the adapter only needs the
// caller's context, which jwt.SigningMethod's verify path does not
// provide.
//
// Usage:
//
//	method := signing.NewJWTSigningMethod(signer, ctx)
//	kid, err := signer.ActiveKeyID(ctx)
//	if err != nil { ... }
//
//	token := jwt.NewWithClaims(method, claims)
//	token.Header["kid"] = kid
//	token.Header["typ"] = "at+JWT"
//
//	signed, err := token.SignedString(nil) // method has the signer; key arg ignored
type jwtSigningMethod struct {
	signer Signer
	ctx    context.Context
}

// NewJWTSigningMethod returns a jwt.SigningMethod that delegates
// signing to the given Signer. The context is captured for use during
// the signing operation — the caller should pass a request-scoped
// context where appropriate.
//
// A new instance should be created per signing operation rather than
// reused across requests, because of the captured context.
func NewJWTSigningMethod(ctx context.Context, signer Signer) jwt.SigningMethod {
	return &jwtSigningMethod{
		signer: signer,
		ctx:    ctx,
	}
}

// Alg returns the JWA algorithm name of the underlying signer.
// Required by jwt.SigningMethod; called by golang-jwt to set the
// "alg" header on the JWT.
func (m *jwtSigningMethod) Alg() string {
	return m.signer.Algorithm()
}

// Sign delegates to the underlying Signer. The key argument is
// ignored — the signer holds its own key material (a private key in
// the database for local mode, or an AWS KMS key reference for kms
// mode). Callers pass nil for the key.
func (m *jwtSigningMethod) Sign(signingString string, key interface{}) ([]byte, error) {
	return m.signer.Sign(m.ctx, []byte(signingString))
}

// Verify is not implemented for this signing method. Verification
// goes through the Verifier interface, which has access to the kid
// header (this method does not) and can dispatch to the right public
// key.
//
// Returning an error here means a token signed with this method
// cannot be verified using golang-jwt's jwt.Parse — which is the
// intended behavior. Callers should use Verifier.Verify or the
// anahita-libs JWKS middleware instead.
var errVerifyNotSupported = errors.New("signing: jwt.SigningMethod.Verify not supported; use Verifier")

func (m *jwtSigningMethod) Verify(signingString string, signature []byte, key interface{}) error {
	return errVerifyNotSupported
}
