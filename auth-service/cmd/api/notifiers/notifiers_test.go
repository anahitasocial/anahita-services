package notifiers

import (
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"auth/config"
	"auth/data/entities"
	"auth/data/repositories"
	"auth/data/services"
	authrequests "auth/requests"

	libservices "anahita-libs/data/services"
)

// fakeUsers stands in for the users repository, which one notifier needs:
// EmailAccept broadcasts to every administrator and looks them up.
//
// Embeds the interface so only the method in use is written; anything else
// this package starts calling panics by name rather than passing silently
// against a zero value.
type fakeUsers struct {
	repositories.UsersInterface

	admins []*entities.User
}

func (f *fakeUsers) Browse(req *authrequests.UserBrowseRequest) ([]*entities.User, error) {
	return f.admins, nil
}

// The formatting guard.
//
// This exists because the failure it catches has already reached real
// recipients twice. Message constants carried a `%s` for a site name, and the
// notifier handed the subject to the mailer WITHOUT formatting it — so people
// waiting on a password reset were sent an email titled "Reset your password
// for %s". The verbs were removed; the site name is now a token substituted in
// one place, and these tests are what keep it that way.
//
// Every notifier method is listed. A new one that skips Env.send, forgets a
// Sprintf, or mismatches an argument list fails here rather than in somebody's
// inbox.

const testSiteName = "Banu"

// recordingMail captures what would have gone out.
type recordingMail struct {
	mu   sync.Mutex
	sent []sentMail
}

type sentMail struct {
	From    string
	To      []string
	Subject string
	Data    map[string]string
}

func (m *recordingMail) SendFromTemplate(from string, to, cc, bcc []string, subject, template string, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fields, _ := data.(map[string]string)
	m.sent = append(m.sent, sentMail{From: from, To: to, Subject: subject, Data: fields})
	return nil
}

func (m *recordingMail) Send(from string, to, cc, bcc []string, subject, message string) (string, error) {
	return "ok", nil
}

var _ libservices.MailGRPCInterface = (*recordingMail)(nil)

func newTestNotifiers(t *testing.T) (*Notifiers, *recordingMail) {
	t.Helper()

	mail := &recordingMail{}

	admin := testUser()
	admin.ID = 99
	admin.Email = "admin@example.com"
	admin.Name = "Grace Hopper"

	return NewNotifiers(&Env{
		Config: &config.Config{
			Env:         "prod",
			DomainName:  "banu.io",
			SiteName:    testSiteName,
			SystemEmail: "system@banu.io",
		},
		Repos: &repositories.Repositories{
			Users: &fakeUsers{admins: []*entities.User{admin}},
		},
		Srv: &services.Services{MailGRPC: mail},
	}), mail
}

func testUser() *entities.User {
	u := entities.NewUser()
	u.ID = 42
	u.Email = "ada@example.com"
	u.Name = "Ada Lovelace"
	u.Alias = "ada"
	return u
}

func testToken() *entities.Token {
	tk := entities.NewToken()
	tk.ID = 7
	tk.Body = strings.Repeat("a", 128)
	tk.RecipientEmail = "ada@example.com"
	tk.NewEmail = "new@example.com"
	return tk
}

// everyNotification drives one send per notifier method.
//
// Deliberately a hand-written list rather than reflection: a method added
// without a line here is a method nobody checked, and a reflective walk would
// hide that by passing zero values and asserting nothing meaningful.
func everyNotification(n *Notifiers) []struct {
	name string
	call func() error
} {
	user := testUser()
	inviter := testUser()
	inviter.Name = "Charles Babbage"

	authLog := &entities.AuthLog{
		Name: "Firefox", Device: "laptop", OS: "Linux",
		City: "Vancouver", Country: "Canada",
	}

	return []struct {
		name string
		call func() error
	}{
		{"Auth.EmailLogin", func() error { return n.Auth.EmailLogin(user, authLog) }},
		{"Auth.EmailSessionRevoked", func() error {
			return n.Auth.EmailSessionRevoked(user, "Firefox on Linux", "Vancouver, Canada")
		}},
		{"Invite.EmailInvite", func() error {
			return n.Invite.EmailInvite(inviter, "invitee@example.com", testToken())
		}},
		{"Invite.EmailAccept", func() error { return n.Invite.EmailAccept(user, inviter) }},
		{"Invite.EmailRevoked", func() error {
			return n.Invite.EmailRevoked(inviter, "invitee@example.com")
		}},
		{"Password.EmailToken", func() error { return n.Password.EmailToken(user, testToken()) }},
		{"Password.EmailResetSuccess", func() error { return n.Password.EmailResetSuccess(user) }},
		// Zero, one and many: the session count picks between two constants,
		// one of which carries a verb the other does not.
		{"Password.EmailUpdateSuccess/0", func() error { return n.Password.EmailUpdateSuccess(user, 0) }},
		{"Password.EmailUpdateSuccess/1", func() error { return n.Password.EmailUpdateSuccess(user, 1) }},
		{"Password.EmailUpdateSuccess/3", func() error { return n.Password.EmailUpdateSuccess(user, 3) }},
		{"EmailChange.EmailConfirm", func() error { return n.EmailChange.EmailConfirm(user, testToken()) }},
		{"EmailChange.EmailChangedOld", func() error {
			return n.EmailChange.EmailChangedOld(user, "old@example.com")
		}},
		{"EmailChange.EmailChangedNew", func() error { return n.EmailChange.EmailChangedNew(user) }},
		{"AccountDelete.EmailScheduled", func() error {
			return n.AccountDelete.EmailScheduled(user, time.Now().Add(30*24*time.Hour))
		}},
		{"AccountDelete.EmailRestored", func() error { return n.AccountDelete.EmailRestored(user) }},
		{"UsernameChange.EmailChanged", func() error {
			return n.UsernameChange.EmailChanged(user, "oldhandle")
		}},
		{"Signup.EmailSignup", func() error { return n.Signup.EmailSignup(user) }},
		{"Totp.EmailTotpEnabled", func() error { return n.Totp.EmailTotpEnabled(user) }},
		{"Totp.EmailTotpDisabled", func() error { return n.Totp.EmailTotpDisabled(user) }},
		{"Totp.EmailTotpDisabledByAdmin", func() error { return n.Totp.EmailTotpDisabledByAdmin(user) }},
		{"Totp.EmailRecoveryCodesIssued", func() error {
			return n.Totp.EmailRecoveryCodesIssued(user, 16)
		}},
		// None, one and many: three bodies, and the count is wording rather
		// than a verb on two of them.
		{"Totp.EmailRecoveryCodeUsed/0", func() error { return n.Totp.EmailRecoveryCodeUsed(user, 0) }},
		{"Totp.EmailRecoveryCodeUsed/1", func() error { return n.Totp.EmailRecoveryCodeUsed(user, 1) }},
		{"Totp.EmailRecoveryCodeUsed/5", func() error { return n.Totp.EmailRecoveryCodeUsed(user, 5) }},
		{"Totp.EmailLockedOut", func() error { return n.Totp.EmailLockedOut(user) }},
		{"Reauth.EmailLockedOut", func() error { return n.Reauth.EmailLockedOut(user) }},
		{"GroupLifecycle.EmailGroupDeleted", func() error {
			return n.GroupLifecycle.EmailGroupDeleted(user, "Cycling Club", "cycling-club")
		}},
		{"GroupLifecycle.EmailGroupRestored", func() error {
			return n.GroupLifecycle.EmailGroupRestored(user, "Cycling Club", "cycling-club")
		}},
		{"WebAuthn.EmailPasskeyAdded", func() error {
			return n.WebAuthn.EmailPasskeyAdded(user, "iCloud Keychain")
		}},
		{"WebAuthn.EmailPasskeyRemoved", func() error {
			return n.WebAuthn.EmailPasskeyRemoved(user, "iCloud Keychain")
		}},
		// Singular and plural again, for the same reason.
		{"WebAuthn.EmailPasskeysRevoked/1", func() error { return n.WebAuthn.EmailPasskeysRevoked(user, 1) }},
		{"WebAuthn.EmailPasskeysRevoked/4", func() error { return n.WebAuthn.EmailPasskeysRevoked(user, 4) }},
	}
}

// unformattedVerb finds a surviving fmt verb. Deliberately narrow — %s, %d, %v,
// %q and the width/flag forms — so that a legitimate percent in prose ("50%
// faster") does not trip it.
var unformattedVerb = regexp.MustCompile(`%[-+# 0-9.]*[sdvqt]`)

// Nothing that reaches a recipient may contain an unconsumed verb, a Go
// formatting error, or an unsubstituted token.
//
// Both halves of the historical bug are covered: `%!s(MISSING)` is a Sprintf
// with too few arguments, and a bare `%s` is a constant that was never
// formatted at all — which is the form that actually shipped.
func TestNoNotificationLeaksAFormatPlaceholder(t *testing.T) {
	n, mail := newTestNotifiers(t)

	for _, tc := range everyNotification(n) {
		t.Run(tc.name, func(t *testing.T) {
			mail.sent = nil

			if err := tc.call(); err != nil {
				t.Fatalf("send: %v", err)
			}
			if len(mail.sent) == 0 {
				t.Fatal("nothing was sent")
			}

			for _, msg := range mail.sent {
				fields := map[string]string{
					"subject":     msg.Subject,
					"title":       msg.Data["title"],
					"body":        msg.Data["body"],
					"actionLabel": msg.Data["actionLabel"],
					"actionURL":   msg.Data["actionURL"],
				}

				for name, value := range fields {
					if strings.Contains(value, "%!") {
						t.Errorf("%s contains a Go formatting error: %q", name, value)
					}
					if m := unformattedVerb.FindString(value); m != "" {
						t.Errorf("%s contains an unconsumed verb %q: %q", name, m, value)
					}
					if strings.Contains(value, "{{") {
						t.Errorf("%s contains an unsubstituted token: %q", name, value)
					}
				}
			}
		})
	}
}

// The site name has to actually arrive. Without this, a message that lost its
// token would pass the test above by containing no placeholder at all.
func TestNotificationsNameTheSite(t *testing.T) {
	n, mail := newTestNotifiers(t)

	// Not every message needs the site in it — a passkey removal reads fine
	// without one — but these are the ones where it is the trust signal.
	wantSite := map[string]bool{
		"Auth.EmailLogin":            true,
		"Invite.EmailInvite":         true,
		"Password.EmailToken":        true,
		"Password.EmailResetSuccess": true,
		"Signup.EmailSignup":         true,
		"Totp.EmailTotpDisabled":     true,
	}

	for _, tc := range everyNotification(n) {
		if !wantSite[tc.name] {
			continue
		}

		t.Run(tc.name, func(t *testing.T) {
			mail.sent = nil
			if err := tc.call(); err != nil {
				t.Fatalf("send: %v", err)
			}

			msg := mail.sent[0]
			combined := msg.Subject + " " + msg.Data["title"] + " " + msg.Data["body"]
			if !strings.Contains(combined, testSiteName) {
				t.Errorf("the site is never named:\nsubject: %q\nbody: %q",
					msg.Subject, msg.Data["body"])
			}
		})
	}
}

// Every message goes out with the site as the From display name, so a
// recipient can tell which installation wrote to them from the inbox list.
func TestEveryNotificationSetsTheFromDisplayName(t *testing.T) {
	n, mail := newTestNotifiers(t)

	for _, tc := range everyNotification(n) {
		t.Run(tc.name, func(t *testing.T) {
			mail.sent = nil
			if err := tc.call(); err != nil {
				t.Fatalf("send: %v", err)
			}

			for _, msg := range mail.sent {
				if !strings.Contains(msg.From, testSiteName) {
					t.Errorf("From = %q, want it to carry the site name", msg.From)
				}
				if !strings.Contains(msg.From, "system@banu.io") {
					t.Errorf("From = %q, want it to carry the system address", msg.From)
				}
			}
		})
	}
}

// Every message must be addressed to somebody, carry a subject, and offer an
// action. An empty actionURL renders a button that goes nowhere.
func TestEveryNotificationIsComplete(t *testing.T) {
	n, mail := newTestNotifiers(t)

	for _, tc := range everyNotification(n) {
		t.Run(tc.name, func(t *testing.T) {
			mail.sent = nil
			if err := tc.call(); err != nil {
				t.Fatalf("send: %v", err)
			}

			for _, msg := range mail.sent {
				if len(msg.To) != 1 || msg.To[0] == "" {
					t.Errorf("recipients = %v, want exactly one address", msg.To)
				}
				if strings.TrimSpace(msg.Subject) == "" {
					t.Error("empty subject")
				}
				for _, key := range []string{"title", "body", "actionLabel", "actionURL"} {
					if strings.TrimSpace(msg.Data[key]) == "" {
						t.Errorf("empty %s", key)
					}
				}
				if !strings.HasPrefix(msg.Data["actionURL"], "http") {
					t.Errorf("actionURL = %q, want an absolute URL", msg.Data["actionURL"])
				}
			}
		})
	}
}
