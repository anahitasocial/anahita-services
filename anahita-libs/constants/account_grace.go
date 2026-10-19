package constants

import (
	"os"
	"strconv"
	"strings"
)

// How long a deleted profile can be brought back before it is purged: a
// person by signing in again, a group or an event by an administrator.
//
// Thirty days unless the installation says otherwise in ACCOUNT_GRACE_DAYS
// (the user, 2026-10-19), as GitHub, Google and Slack settle on. Too short,
// and somebody who deletes in anger or by accident has no real chance to
// come back; too long, and the data of somebody who wanted to leave stays,
// which is the opposite of what they asked for.
//
// One number for everything that says it or acts on it: the sweeper's cutoff
// (graph-grpc-service), the dates auth-service tells people, NodeInfo's
// accountGraceDays, which the web app shows, and the site's promise. It is not
// stored per profile: changing it moves the date of every profile already
// waiting, shortening their time when it is made shorter.
const (
	EnvAccountGraceDays     = "ACCOUNT_GRACE_DAYS"
	AccountGraceDaysDefault = 30
)

// AccountGraceDays is the days a deleted profile can be brought back.
func AccountGraceDays() int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(EnvAccountGraceDays)))
	if err != nil || value <= 0 {
		return AccountGraceDaysDefault
	}
	return value
}
