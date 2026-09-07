package notifiers

import (
	"strings"

	"auth/config"
	"auth/data/repositories"
	"auth/data/services"
	"auth/messages"

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

// site substitutes this installation's name into a message.
//
// Every subject, title and body passes through here, including the ones with
// no token in them — applying it uniformly is cheaper than remembering which
// constants carry one, and a ReplaceAll that matches nothing costs nothing.
//
// This is the whole mechanism for the site name, and it is deliberately not a
// fmt verb. The site was previously written as a `%s` on constants the
// notifiers handed to the mailer unformatted, so recipients read "Welcome to
// %s" and "Reset your password for %s". A token cannot be knocked out of
// alignment by adding an argument, and cannot be forgotten at a call site,
// because it is substituted here rather than there.
func (env *Env) site(s string) string {
	return strings.ReplaceAll(s, messages.SiteToken, env.Config.GetSiteName())
}

// send is the one path out of this package.
//
// Every notifier funnels through it so that the From address, the template
// name and the four data keys are decided once. Before this, each of the
// nineteen methods repeated the same six lines, which is how one of them ends
// up passing an unformatted subject.
//
// subject, title and body arrive already formatted by the caller — they are
// the parts that differ per message — and are site-substituted here.
func (env *Env) send(recipient, subject, title, body, actionLabel, actionURL string) error {
	data := map[string]string{
		"title":       env.site(title),
		"body":        env.site(body),
		"actionLabel": env.site(actionLabel),
		"actionURL":   actionURL,
	}

	return env.Srv.MailGRPC.SendFromTemplate(
		// The From header carries the site name as a display part, so mail
		// arrives from `Banu <system@banu.io>` rather than a bare address.
		// providers/smtp.go parses the address back out for the envelope.
		env.Config.GetMailFrom(),
		[]string{recipient},
		[]string{},
		[]string{},
		env.site(subject),
		"message_action",
		data,
	)
}

type Notifiers struct {
	Auth           *authNotifier
	Invite         *inviteNotifier
	Password       *passwordNotifier
	EmailChange    *emailChangeNotifier
	AccountDelete  *accountDeleteNotifier
	UsernameChange *usernameChangeNotifier
	Signup         *signupNotifier
	GroupLifecycle *groupLifecycleNotifier
	Reauth         *reauthNotifier
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
		GroupLifecycle: newGroupLifecycleNotifier(env),
		Reauth:         newReauthNotifier(env),
		Totp:           newTotpNotifier(env),
		WebAuthn:       newWebAuthnNotifier(env),
	}
}
