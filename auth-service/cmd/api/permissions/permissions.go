package permissions

import (
	"auth/config"
	"auth/data/repositories"

	"anahita-libs/data/entities"
)

type Env struct {
	Config *config.Config
	Repos  *repositories.Repositories
	Viewer *entities.Person
}

type Permissions struct {
	AuthLog         *AuthLogPerm
	Invite          *InvitePerm
	OAuth           *OAuthPerm
	OAuthSigningKey *OAuthSigningKeyPerm
	Signup          *SignupPermissions
	SignupRequest   *SignupRequestPerm
	Totp            *TotpPerm
	Users           *UsersPermissions
	WebAuthn        *WebAuthnPerm
}

func NewPermissions(env *Env) *Permissions {
	return &Permissions{
		AuthLog:         NewAuthLogPerm(env),
		Invite:          NewInvitePerm(env),
		OAuth:           NewOAuthPerm(env),
		OAuthSigningKey: NewOAuthSigningKeyPerm(env),
		Signup:          NewSignupPermissions(env),
		SignupRequest:   NewSignupRequestPerm(env),
		Totp:            NewTotpPerm(env),
		Users:           NewUsersPermissions(env),
		WebAuthn:        NewWebAuthnPerm(env),
	}
}
