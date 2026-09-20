package handlers

type Handlers struct {
	AccountDeleted *accountDeletedHandler
	Agreements     *agreementsHandler
	SignupRequests *signupRequestsHandler
	AuthLog        *authLogHandler
	EmailChange    *emailChangeHandler
	Invite         *inviteHandler
	NodeInfo       *nodeInfoHandler
	Is             *isHandler
	OAuth          *oauthHandler
	Password       *passwordHandler
	Reauth         *reauthHandler
	Signup         *signupHandler
	Token          *tokenHandler
	Totp           *totpHandler
	UsernameChange *usernameChangeHandler
	TotpValidator  *totpValidatorHandler
	WebAuthn       *webAuthnHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		AccountDeleted: newAccountDeletedHandler(env),
		Agreements:     newAgreementsHandler(env),
		SignupRequests: newSignupRequestsHandler(env),
		AuthLog:        newAuthLogHandler(env),
		EmailChange:    newEmailChangeHandler(env),
		Invite:         newInviteHandler(env),
		NodeInfo:       newNodeInfoHandler(env),
		Is:             newIsHandlers(env),
		OAuth:          newOAuthHandler(env),
		Password:       newPasswordHandler(env),
		Reauth:         newReauthHandler(env),
		Signup:         newSignupHandler(env),
		Token:          newTokenHandler(env),
		Totp:           newTotpHandler(env),
		UsernameChange: newUsernameChangeHandler(env),
		TotpValidator:  newTotpValidatorHandler(env),
		WebAuthn:       newWebAuthnHandler(env),
	}
}
