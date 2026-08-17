package handlers

type Handlers struct {
	Authlog       *authLogHandler
	Invite        *inviteHandler
	Is            *isHandler
	OAuth         *oauthHandler
	Password      *passwordHandler
	Signup        *signupHandler
	Token         *tokenHandler
	Totp          *totpHandler
	TotpValidator *totpValidatorHandler
	WebAuthn      *webAuthnHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Authlog:       newAuthLogHandler(env),
		Invite:        newInviteHandler(env),
		Is:            newIsHandlers(env),
		OAuth:         newOAuthHandler(env),
		Password:      newPasswordHandler(env),
		Signup:        newSignupHandler(env),
		Token:         newTokenHandler(env),
		Totp:          newTotpHandler(env),
		TotpValidator: newTotpValidatorHandler(env),
		WebAuthn:      newWebAuthnHandler(env),
	}
}
