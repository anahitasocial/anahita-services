package config

import (
	"strings"
	"testing"

	"auth/constants"
)

// The mode answers three separate questions, and the reason they are three
// predicates rather than callers comparing strings is that the answers do not
// move together:
//
//	AllowsSelfServeRegistration   `approval` is TRUE — an applicant starts an
//	                              application, and refusing them at the gate
//	                              would make the queue unreachable
//	RequiresRegistrationApproval  only `approval`
//	AllowsInvitations             everything except `closed`
//
// A caller writing `mode == "open"` gets the first one wrong, which is how the
// approval queue would end up built and unreachable.

func TestRegistrationModeAnswers(t *testing.T) {
	for _, tc := range []struct {
		mode        string
		selfServe   bool
		approval    bool
		invitations bool
	}{
		{constants.RegistrationModeOpen, true, false, true},
		{constants.RegistrationModeApproval, true, true, true},
		{constants.RegistrationModeInvite, false, false, true},
		{constants.RegistrationModeClosed, false, false, false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			config := &Config{RegistrationMode: tc.mode}

			if got := config.AllowsSelfServeRegistration(); got != tc.selfServe {
				t.Errorf("AllowsSelfServeRegistration() = %v, want %v", got, tc.selfServe)
			}
			if got := config.RequiresRegistrationApproval(); got != tc.approval {
				t.Errorf("RequiresRegistrationApproval() = %v, want %v", got, tc.approval)
			}
			if got := config.AllowsInvitations(); got != tc.invitations {
				t.Errorf("AllowsInvitations() = %v, want %v", got, tc.invitations)
			}
		})
	}
}

// `invite` and `closed` differ in exactly one answer, and it is the reason
// both modes exist. Pinned on its own because a refactor that collapsed them
// would pass every other test here.
func TestClosedSuspendsInvitationsAndInviteDoesNot(t *testing.T) {
	invite := &Config{RegistrationMode: constants.RegistrationModeInvite}
	closed := &Config{RegistrationMode: constants.RegistrationModeClosed}

	if !invite.AllowsInvitations() {
		t.Error("`invite` refuses invitations, which leaves it doing nothing at all")
	}
	if closed.AllowsInvitations() {
		t.Error("`closed` still accepts invitations, so it is not a lockdown — " +
			"the tokens already sent are exactly what needs stopping")
	}
	if invite.AllowsSelfServeRegistration() || closed.AllowsSelfServeRegistration() {
		t.Error("neither mode takes self-serve registrations")
	}
}

// Unset means closed. Fail-safe, and the asymmetry is the argument: a closed
// installation is noticed within a minute by the operator trying to use it,
// while an open one is noticed when the spam arrives.
func TestUnsetRegistrationModeIsClosed(t *testing.T) {
	config := &Config{}

	if got := config.GetRegistrationMode(); got != constants.RegistrationModeClosed {
		t.Errorf("GetRegistrationMode() = %q, want %q",
			got, constants.RegistrationModeClosed)
	}
	if config.AllowsSelfServeRegistration() {
		t.Error("an unconfigured installation is open to the internet")
	}
	if config.AllowsInvitations() {
		t.Error("an unconfigured installation accepts invitations")
	}
}

// Case and whitespace are operator typing, not intent.
func TestRegistrationModeIsNormalised(t *testing.T) {
	for _, raw := range []string{"OPEN", " open ", "Open"} {
		config := &Config{RegistrationMode: raw}

		if !config.AllowsSelfServeRegistration() {
			t.Errorf("%q was not read as open", raw)
		}
		if err := (&Config{RegistrationMode: raw}).validateBootstrap(nil); len(err) != 0 {
			t.Errorf("%q was rejected by validation: %v", raw, err)
		}
	}
}

// `approval` boots now that the queue exists.
//
// It used to be refused, and the refusal was right at the time: with nowhere
// for an application to go, asking for approval would have given open
// registration on a site the operator believed was gated.
func TestApprovalBootsNowThatTheQueueExists(t *testing.T) {
	problems := (&Config{
		RegistrationMode: constants.RegistrationModeApproval,
	}).validateBootstrap(nil)

	if len(problems) != 0 {
		t.Errorf("approval mode was refused at boot: %v", problems)
	}
}

// A mistyped mode is rejected at boot rather than falling back.
//
// Falling back to `closed` would look like the setting was ignored; falling
// back to `open` would be worse. An operator who wrote "invites" believes they
// have configured something, and the only honest response is to say so before
// the service starts taking requests.
func TestMistypedRegistrationModeIsRefused(t *testing.T) {
	for _, raw := range []string{"invites", "public", "inviteonly", "true"} {
		problems := (&Config{RegistrationMode: raw}).validateBootstrap(nil)

		if len(problems) == 0 {
			t.Errorf("%q was accepted as a registration mode", raw)
			continue
		}

		// The message has to name what was expected. An operator reading
		// "invalid REGISTRATION_MODE" at three in the morning needs the list,
		// not a verdict.
		joined := strings.Join(problems, " ")
		for _, mode := range constants.RegistrationModes {
			if !strings.Contains(joined, mode) {
				t.Errorf("the error for %q does not mention %q: %s", raw, mode, joined)
			}
		}
		if !strings.Contains(joined, raw) {
			t.Errorf("the error for %q does not quote what was given: %s", raw, joined)
		}
	}
}
