package constants

import (
	"os"
	"strconv"
	"strings"
)

// PurgeReasonSpam is an account removed as spam by a moderator (roadmap row
// 102): destroyed at once, without the days a deleted account waits, and
// recorded as removed-as-spam with who decided.
const PurgeReasonSpam = "spam"

// EnvSpamRemovalMaxDays names the setting for how new an account must be to
// be removed as spam.
const EnvSpamRemovalMaxDays = "SPAM_REMOVAL_MAX_DAYS"

// SpamRemovalMaxDaysDefault is how new, unless the installation says
// otherwise: spam is new, and an account that has been here longer is
// disabled or deleted, with the thirty days to bring it back (the user,
// 2026-10-19).
const SpamRemovalMaxDaysDefault = 30

// SpamRemovalMaxDays is how many days after it was made an account may still
// be removed as spam (SPAM_REMOVAL_MAX_DAYS, 30 unless said).
func SpamRemovalMaxDays() int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(EnvSpamRemovalMaxDays)))
	if err != nil || value <= 0 {
		return SpamRemovalMaxDaysDefault
	}
	return value
}
