package constants

import libs "anahita-libs/constants"

// AccountGraceDays is how long a deleted account can be restored before the
// sweeper purges it: ACCOUNT_GRACE_DAYS, 30 unless said, the one number the
// sweeper's cutoff and every date said here share (anahita-libs/constants).
//
// Read by the login restore page and the emails to tell the person the date.
// Changing it moves the date of every account already deleted: it is not
// stored per account.
func AccountGraceDays() int {
	return libs.AccountGraceDays()
}
