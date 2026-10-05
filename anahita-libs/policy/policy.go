// Package policy holds the settings an installation makes about who may see
// what, which more than one service has to agree on.
//
// They are environment variables, set once in the shared configuration and
// read by every service the same way, like GRPC_TLS in package grpctls. They
// take effect on a restart.
//
//	MIN_CONTENT_ACCESS  "public" or "registered". The widest audience
//	                    anything new may be given. Unset means public.
//
// "registered" is for a school, a company or a private community that wants
// nothing new to be readable by people who are not signed in. It governs what
// is WRITTEN from now on: posts, profiles and groups can no longer be made
// public, and new accounts start as registered. What is already public stays
// public until somebody changes it.
//
// A value that is not understood is read as the stricter one, and said so in
// the log. An installation that set this meant to restrict something, and a
// typo must not be what quietly makes everything public again.
package policy

import (
	"log"
	"os"
	"strings"
	"sync"

	"anahita-libs/constants"
)

const EnvMinContentAccess = "MIN_CONTENT_ACCESS"

var (
	once             sync.Once
	minContentAccess string
)

func load() {
	once.Do(func() {
		minContentAccess = readMinContentAccess(os.Getenv(EnvMinContentAccess))
	})
}

func readMinContentAccess(raw string) string {
	switch value := strings.ToLower(strings.TrimSpace(raw)); value {
	case "", constants.AnahitaAccessPublic:
		return constants.AnahitaAccessPublic
	case constants.AnahitaAccessRegistered:
		return constants.AnahitaAccessRegistered
	default:
		log.Printf("policy: %s is %q, which is neither %q nor %q; treating it as %q",
			EnvMinContentAccess, raw,
			constants.AnahitaAccessPublic, constants.AnahitaAccessRegistered,
			constants.AnahitaAccessRegistered)
		return constants.AnahitaAccessRegistered
	}
}

// MinContentAccess is the widest audience anything new may be given: "public"
// or "registered". It is also what something new gets when nobody says.
func MinContentAccess() string {
	load()
	return minContentAccess
}

// AllowsPublic reports whether new posts, profiles and groups may be public.
func AllowsPublic() bool {
	return MinContentAccess() == constants.AnahitaAccessPublic
}

// AllowsAccess reports whether something may be given this access level now.
// Every level but "public" always may.
func AllowsAccess(access string) bool {
	return access != constants.AnahitaAccessPublic || AllowsPublic()
}

// Allowed returns the levels out of these that may be given now, in the order
// they came.
func Allowed(levels []string) []string {
	allowed := make([]string, 0, len(levels))
	for _, level := range levels {
		if AllowsAccess(level) {
			allowed = append(allowed, level)
		}
	}
	return allowed
}

// SetForTest replaces the settings for the length of a test, and returns a
// function that puts them back. The environment is read once per process, so
// a test cannot change it by setting a variable.
func SetForTest(minContent string) (restore func()) {
	load()
	previous := minContentAccess
	minContentAccess = readMinContentAccess(minContent)
	return func() { minContentAccess = previous }
}
