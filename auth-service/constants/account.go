package constants

// AccountGraceDays is how long a deleted account can be restored before the
// sweeper purges it.
//
// Thirty days, matching what GitHub, Google and Slack settle on. The number is
// a balance between two failures: too short and somebody who deletes in anger,
// or by accident, has no realistic chance to come back; too long and the data
// of people who genuinely wanted to leave sits around, which is the opposite of
// what they asked for and a problem under any erasure obligation.
//
// Read by the sweeper to pick its cutoff and by the login restore page to tell
// the person the date. Changing it moves both together, and shortens the window
// for accounts already deleted — it is not stored per account.
const AccountGraceDays = 30
