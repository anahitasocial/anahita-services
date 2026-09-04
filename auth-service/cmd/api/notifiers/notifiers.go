package notifiers

import (
	"auth/config"
	"auth/data/repositories"
	"auth/data/services"

	lib_entities "anahita-libs/data/entities"
	"anahita-libs/event"
)

type Env struct {
	Config    *config.Config
	Repos     *repositories.Repositories
	Viewer    *lib_entities.Person
	Srv       *services.Services
	Publisher *event.Publisher
}

type Notifiers struct {
	Auth           *authNotifier
	Invite         *inviteNotifier
	Password       *passwordNotifier
	EmailChange    *emailChangeNotifier
	AccountDelete  *accountDeleteNotifier
	UsernameChange *usernameChangeNotifier
	Signup         *signupNotifier
	Totp           *totpNotifier
	WebAuthn       *webAuthnNotifier
}

func NewNotifiers(env *Env) *Notifiers {
	return &Notifiers{
		Auth:           newAuthNotifier(env),
		Invite:         newInviteNotifier(env),
		Password:       newPasswordNotifier(env),
		EmailChange:    newEmailChangeNotifier(env),
		AccountDelete:  newAccountDeleteNotifier(env),
		UsernameChange: newUsernameChangeNotifier(env),
		Signup:         newSignupNotifier(env),
		Totp:           newTotpNotifier(env),
		WebAuthn:       newWebAuthnNotifier(env),
	}
}
