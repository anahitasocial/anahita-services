package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"auth/constants"
	"auth/data/entities"

	libs_constants "anahita-libs/constants"
)

// THE RULE THIS GUARDS (roadmap row 170): somebody who joins by invitation
// is signed in with the same session sign-in writes, indexed against them,
// so that a password reset or their account's deletion ends it. Written by
// hand, it was not, and nothing could revoke it until it expired.
func TestAcceptingAnInvitationSignsInWithARevocableSession(t *testing.T) {
	serviceRoot(t)

	admin := entities.NewUser()
	admin.ID = 1
	admin.Email = "holden@example.org"
	admin.Alias = "holden"
	admin.PersonType = libs_constants.PersonTypeAdmin

	invite := resetToken(admin.ID, "naomi@example.org")
	invite.Type = constants.TokenTypeInvite
	invite.Body = strings.Repeat("i", 128)
	invite.SetStatus()

	te := newTestEnv(admin)
	te.Tokens = newFakeTokens(invite)
	te.Env.Repos.Tokens = te.Tokens
	te.Env.Config.RegistrationMode = constants.RegistrationModeInvite
	te.Env.Config.SiteRules = "off"
	h := &inviteHandler{Env: te.Env}

	c, rec := postForm(newTestEcho(), "/invites/accept", url.Values{
		"token":    {invite.Body},
		"username": {"naomi"},
		"name":     {"Naomi Nagata"},
		"password": {"a long passphrase for the test"},
	})
	if err := h.Accept(c); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want a redirect into the app:\n%s", rec.Code, rec.Body.String())
	}

	joined, err := te.Users.ReadByEmail("naomi@example.org")
	if err != nil {
		t.Fatalf("no account was made: %v", err)
	}
	if sessions := te.Sess.sets[personSessionIndexKey(joined.ID)]; len(sessions) != 1 {
		t.Fatalf("the new session is indexed %d times, want once: a password reset could not end it", len(sessions))
	}
	if !(&oauthHandler{Env: te.Env}).consumeFreshSignIn(joined.ID) {
		t.Error("joining was not taken for a sign-in, and its email would not be sent")
	}
}
