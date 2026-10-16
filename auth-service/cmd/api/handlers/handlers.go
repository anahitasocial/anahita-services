package handlers

type Handlers struct {
	AccountDeleted *accountDeletedHandler
	Agreements     *agreementsHandler
	SignupRequests *signupRequestsHandler
	Accounts       *accountsHandler
	AuthLog        *authLogHandler
	EmailChange    *emailChangeHandler
	Invite         *inviteHandler
	NodeInfo       *nodeInfoHandler
	Onboarding     *onboardingHandler
	Is             *isHandler
	OAuth          *oauthHandler
	Password       *passwordHandler
	Reauth         *reauthHandler
	Signup         *signupHandler
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
		Accounts:       newAccountsHandler(env),
		AuthLog:        newAuthLogHandler(env),
		EmailChange:    newEmailChangeHandler(env),
		Invite:         newInviteHandler(env),
		NodeInfo:       newNodeInfoHandler(env),
		Onboarding:     newOnboardingHandler(env),
		Is:             newIsHandlers(env),
		OAuth:          newOAuthHandler(env),
		Password:       newPasswordHandler(env),
		Reauth:         newReauthHandler(env),
		Signup:         newSignupHandler(env),
		Totp:           newTotpHandler(env),
		UsernameChange: newUsernameChangeHandler(env),
		TotpValidator:  newTotpValidatorHandler(env),
		WebAuthn:       newWebAuthnHandler(env),
	}
}
