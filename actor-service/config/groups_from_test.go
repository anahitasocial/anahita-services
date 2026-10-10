package config

import (
	"anahita-libs/constants"
	"strings"
	"testing"
)

// Who may create a group.
//
// The ranking itself is tested in anahita-libs/constants; what matters here is
// that this service resolves the setting the way auth-service does, since the
// two read the same variable independently — one to enforce it, one to
// publish it — and a disagreement would mean the document advertises a policy
// the server does not apply.

func TestGetGroupsFromDefaultsToAdministrators(t *testing.T) {
	// Unset is the case every existing installation upgrades into, so it has
	// to mean what happened before the setting existed.
	config := &Config{}

	if got := config.GetGroupsFrom(); got != constants.RoleLevelAdministrators {
		t.Errorf("GetGroupsFrom() = %q, want %q", got, constants.RoleLevelAdministrators)
	}

	if config.MayCreateGroup(constants.PersonTypeRegistered) {
		t.Error("a registered person should not create groups by default")
	}
	if !config.MayCreateGroup(constants.PersonTypeAdmin) {
		t.Error("an administrator should create groups by default")
	}
	if !config.MayCreateGroup(constants.PersonTypeSuperAdmin) {
		t.Error("a super administrator should create groups by default")
	}
}

func TestMayCreateGroupHonoursTheLevel(t *testing.T) {
	registered := &Config{GroupsFrom: constants.RoleLevelRegistered}

	if !registered.MayCreateGroup(constants.PersonTypeRegistered) {
		t.Error("registered level should admit a registered person")
	}
	if registered.MayCreateGroup(constants.PersonTypeGuest) {
		t.Error("no level admits a guest")
	}

	// The case the setting exists for, and the one an administrator shortcut
	// in the permission would silently break: `super-administrators` has to
	// exclude ordinary administrators, or it is a level that does nothing.
	superOnly := &Config{GroupsFrom: constants.RoleLevelSuperAdministrators}

	if superOnly.MayCreateGroup(constants.PersonTypeAdmin) {
		t.Error("super-administrators must exclude an ordinary administrator")
	}
	if !superOnly.MayCreateGroup(constants.PersonTypeSuperAdmin) {
		t.Error("super-administrators must admit a super administrator")
	}
}

func TestGroupsFromForgivesCaseAndSpace(t *testing.T) {
	config := &Config{GroupsFrom: "  Registered \n"}

	if got := config.GetGroupsFrom(); got != constants.RoleLevelRegistered {
		t.Errorf("GetGroupsFrom() = %q, want %q", got, constants.RoleLevelRegistered)
	}
}

func TestValidateRefusesAnUnknownGroupsFrom(t *testing.T) {
	// A typo must stop the service rather than be coerced. An unknown level
	// ranks above every person type, so booting on one would refuse every
	// group creation with nothing to explain why.
	config := validConfig()
	config.GroupsFrom = "moderators"

	err := config.Validate()
	if err == nil {
		t.Fatal("Validate() accepted an unknown level")
	}

	// The message has to name the alternatives; an operator who mistyped one
	// value cannot guess the set from a rejection alone.
	for _, level := range constants.RoleLevelValues {
		if !strings.Contains(err.Error(), level) {
			t.Errorf("error does not name %q: %v", level, err)
		}
	}
}

func TestValidateAcceptsUnsetAndEveryKnownGroupsFrom(t *testing.T) {
	config := validConfig()
	config.GroupsFrom = ""

	if err := config.Validate(); err != nil {
		t.Errorf("unset GROUPS_FROM should be legal: %v", err)
	}

	for _, level := range constants.RoleLevelValues {
		config.GroupsFrom = level
		if err := config.Validate(); err != nil {
			t.Errorf("GROUPS_FROM=%q should be legal: %v", level, err)
		}
	}
}

// Everything else Validate insists on, so these tests fail on the one field
// they are about rather than on something else that is missing.
func validConfig() *Config {
	return &Config{
		SecureCookieKey:  strings.Repeat("k", 32),
		AmqpAddr:         "amqp://localhost",
		FileServiceAddr:  "file:50051",
		GraphServiceAddr: "graph:50051",
		OAuthIssuer:      "https://auth.example.com",
		OAuthJWKSURL:     "https://auth.example.com/.well-known/jwks.json",
	}
}

// Events default the other way from groups: anybody signed in may make one
// until an installation says otherwise.
func TestGetEventsFromDefaultsToRegistered(t *testing.T) {
	config := &Config{}
	if got := config.GetEventsFrom(); got != constants.RoleLevelRegistered {
		t.Errorf("GetEventsFrom() = %q, want %q", got, constants.RoleLevelRegistered)
	}

	// And says what it was set to, however it was written.
	set := &Config{EventsFrom: " Administrators "}
	if got := set.GetEventsFrom(); got != constants.RoleLevelAdministrators {
		t.Errorf("GetEventsFrom() = %q, want %q", got, constants.RoleLevelAdministrators)
	}

	// The two settings do not move each other.
	both := &Config{GroupsFrom: constants.RoleLevelSuperAdministrators}
	if got := both.GetEventsFrom(); got != constants.RoleLevelRegistered {
		t.Errorf("with GROUPS_FROM set, GetEventsFrom() = %q, want %q", got, constants.RoleLevelRegistered)
	}
}
