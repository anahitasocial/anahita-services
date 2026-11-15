package handlers

import (
	"testing"

	"auth/data/entities"
)

// A sign-in marks the person, and the code exchange after it consumes the
// mark: one sign-in, one email. An exchange with no sign-in before it, a
// browser whose IdP session is alive being handed a new code, finds none.
func TestAFreshSignInIsConsumedOnce(t *testing.T) {
	te := newTestEnv()
	h := &oauthHandler{Env: te.Env}

	if h.consumeFreshSignIn(7) {
		t.Error("a silent re-authorization was taken for a sign-in")
	}

	h.markFreshSignIn(7)
	if !h.consumeFreshSignIn(7) {
		t.Error("the exchange after a sign-in did not find it")
	}
	if h.consumeFreshSignIn(7) {
		t.Error("one sign-in was found twice")
	}

	h.markFreshSignIn(8)
	if h.consumeFreshSignIn(7) {
		t.Error("another person's sign-in was taken for this one's")
	}
}

// When the store cannot be asked, the email is sent: an extra one costs
// less than a sign-in nobody is told of.
func TestAFreshSignInFailsOpen(t *testing.T) {
	te := newTestEnv()
	te.Sess.existsErr = errStoreDown
	h := &oauthHandler{Env: te.Env}

	if !h.consumeFreshSignIn(7) {
		t.Error("an unreadable store kept the email back")
	}
}

// Both ways of signing in pass through writeViewerSession, and leave the
// mark there.
func TestWriteViewerSessionMarksAFreshSignIn(t *testing.T) {
	user := entities.NewUser()
	user.ID = 7
	user.Email = "naomi@example.org"
	te := newTestEnv(user)
	h := &oauthHandler{Env: te.Env}

	if err := h.writeViewerSession(renewalContext(), user); err != nil {
		t.Fatalf("writeViewerSession: %v", err)
	}
	if !h.consumeFreshSignIn(7) {
		t.Error("a sign-in left no mark")
	}
}

// The email is sent for a new session after a sign-in, and for nothing
// else.
func TestLoginEmailDue(t *testing.T) {
	for _, each := range []struct {
		fresh, reused, want bool
	}{
		{fresh: true, reused: false, want: true},
		{fresh: false, reused: false, want: false},
		{fresh: true, reused: true, want: false},
		{fresh: false, reused: true, want: false},
	} {
		send, reason := loginEmailDue(each.fresh, each.reused)
		if send != each.want {
			t.Errorf("fresh %v, reused %v: send %v, want %v", each.fresh, each.reused, send, each.want)
		}
		if !send && reason == "" {
			t.Errorf("fresh %v, reused %v: kept back with no reason", each.fresh, each.reused)
		}
	}
}
