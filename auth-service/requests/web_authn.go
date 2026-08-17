package requests

// WebAuthnCredentialReadReq addresses a single credential by its
// public UUID. The internal BIGSERIAL is never exposed.
type WebAuthnCredentialReadReq struct {
	ID int64 `param:"id" validate:"required"`
}

// WebAuthnCredentialEditReq renames a credential. Nickname is the only
// mutable field — everything else is either cryptographic material or
// authenticator-reported state.
type WebAuthnCredentialEditReq struct {
	ID       int64  `param:"id" validate:"required"`
	Nickname string `json:"nickname" form:"nickname" validate:"required,min=1,max=64"`
}

type WebAuthnCredentialDeleteReq struct {
	ID int64 `param:"id" validate:"required"`
}

// WebAuthnCredentialUserReq addresses the passkeys belonging to one
// user. Shared by the admin count and revoke-all endpoints, which
// differ only in verb.
//
// Addressed by user ID rather than the numeric UserID, matching
// how every other user-scoped route is addressed. The permission
// layer decides whether the viewer may act on that user; this only
// says who is meant.
type WebAuthnCredentialUserReq struct {
	UserID int64 `param:"user_id" validate:"required"`
}

// No request type for register/begin: it takes no input. The nickname
// it used to carry is now derived server-side after the ceremony,
// because nothing that determines a good name is known until then.
type WebAuthnCredentialRequest struct {
	Read   *WebAuthnCredentialReadReq
	Edit   *WebAuthnCredentialEditReq
	Delete *WebAuthnCredentialDeleteReq
	User   *WebAuthnCredentialUserReq
}

func NewWebAuthnCredentialRequest() *WebAuthnCredentialRequest {
	return &WebAuthnCredentialRequest{
		Read:   &WebAuthnCredentialReadReq{},
		Edit:   &WebAuthnCredentialEditReq{},
		Delete: &WebAuthnCredentialDeleteReq{},
		User:   &WebAuthnCredentialUserReq{},
	}
}
