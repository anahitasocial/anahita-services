package handlers

import (
	"log"
	"net"
	"time"

	"anahita-libs/ratelimit"
)

// ---------------------------------------------------------------------
// Rate limiting
// ---------------------------------------------------------------------
//
// A fixed-window counter per (scope, subject), kept in the session
// store. Scopes are declared here rather than passed as strings at the
// call site so that the limit and the window travel with the name —
// a caller cannot accidentally apply the reset budget to the forgot
// flow, and reading rateLimits below tells you every limit the service
// enforces.
//
// Subjects are hashed into the key. Two reasons: an email address or a
// reset token in a Redis key is a copy of that value sitting in a
// second system with a different retention story, and subjects are
// caller-supplied and unbounded, so hashing is also what keeps key
// length fixed.

type rateLimitScope string

const (
	// rateLimitScopePasswordEmail caps reset emails per address.
	// Without it, POST /password/forgot is an unauthenticated way to
	// send mail to any address on demand.
	// Password sign-in. There was no limit at all on POST /oauth/login, so a
	// password could be guessed at whatever rate the server answered.
	// Counted per identifier typed and per address, both before the account
	// is looked up, so the limit says nothing about whether one exists. The
	// identifier's count is cleared by a successful sign-in.
	rateLimitScopeLoginIdentifier rateLimitScope = "login_identifier"
	rateLimitScopeLoginIP         rateLimitScope = "login_ip"

	// Asking for a sign-in code. Each one is an email, so this is what stops
	// the form being used to flood somebody's inbox, and it caps how many
	// five-guess codes can be requested for one account in an hour.
	rateLimitScopeLoginCodeEmail rateLimitScope = "login_code_email"
	rateLimitScopeLoginCodeIP    rateLimitScope = "login_code_ip"

	rateLimitScopePasswordEmail rateLimitScope = "pw_forgot_email"

	// rateLimitScopePasswordIP caps reset requests per source address.
	// Catches the case the per-email counter cannot: spraying MANY
	// addresses passes every fresh email check, so something has to
	// count the sprayer rather than the targets.
	rateLimitScopePasswordIP rateLimitScope = "pw_forgot_ip"

	// rateLimitScopePasswordReset caps "is this the current password?"
	// probes against one reset link. Keyed on the token, so a fresh
	// link starts with a fresh budget — see the long note in
	// passwordHandler.Reset for why that distinction matters.
	rateLimitScopePasswordReset rateLimitScope = "pw_reset_token"

	// rateLimitScopePasswordEdit caps password changes per person on
	// the authenticated endpoint.
	rateLimitScopePasswordEdit rateLimitScope = "pw_edit_person"

	// rateLimitScopeReauth caps step-up attempts per person.
	//
	// /reauth/password compares a submitted password against the
	// stored hash and says whether it matched. That is a password
	// oracle for anyone holding a session, which is the one thing a
	// step-up gate cannot assume its caller does not have — a stolen
	// unlocked browser is a session. Unmetered, it would let that
	// attacker walk off with the plaintext to try elsewhere, which is
	// worth more to them than the account they already hold.
	rateLimitScopeReauth rateLimitScope = "reauth_person"

	// rateLimitScopeInvitePerson caps invitations sent by one person.
	//
	// Inviting sends mail to an address the recipient never gave us, so an
	// unmetered invite endpoint is an authenticated way to relay mail — and
	// the account doing it need not be malicious, only compromised.
	//
	// Distinct from the pending-invitation cap, which bounds the STOCK of
	// outstanding invitations. This bounds the FLOW: somebody capped at five
	// outstanding can still send five, have them accepted within a minute,
	// and send five more, which the cap cannot see.
	rateLimitScopeInvitePerson rateLimitScope = "invite_person"

	// rateLimitScopeInviteEmail caps how often ONE address can be invited,
	// by anybody.
	//
	// The per-person counter cannot see this case: somebody who does not want
	// to be invited is not made happier by the invitations arriving from
	// different members, each of them within their own budget. Same shape as
	// pw_forgot_email, and for the same reason — it guards the recipient
	// rather than the sender.
	rateLimitScopeInviteEmail rateLimitScope = "invite_email"

	// rateLimitScopeSignupConfirm caps confirmation resends per address.
	//
	// Unauthenticated and it sends mail, so without a limit it is a way to
	// flood the inbox of anybody who has an unconfirmed account here.
	rateLimitScopeSignupConfirm rateLimitScope = "signup_confirm_email"

	// rateLimitScopeSignupIP caps accounts created from one address.
	//
	// Signup is unauthenticated and sends mail, so the source address is the
	// only subject there is to meter. Nothing else about a caller is known at
	// this point — that is the whole difficulty of the endpoint.
	rateLimitScopeSignupIP rateLimitScope = "signup_ip"

	// rateLimitScopeSignupSubnet caps accounts created from one NETWORK.
	//
	// The per-address counter cannot see the case it exists for: a caller with
	// a range to spend walks one address per account and never trips it. A /24
	// and a /64 are the smallest blocks routinely allocated to one customer,
	// so this meters roughly "one subscriber" without reaching across an ISP.
	//
	// Looser than the per-address budget rather than tighter, because a
	// university, an office or a mobile carrier NAT legitimately shares one
	// block — the point is to bound a farm, not to punish a shared exit.
	rateLimitScopeSignupSubnet rateLimitScope = "signup_subnet"
)

type rateLimit struct {
	max    int64
	window time.Duration
}

// rateLimits holds the budget for every scope.
//
// The windows are deliberately uneven. The forgot-password counters
// guard an outbound mail path, where the cost of a false positive is
// low and the abuse is slow and wide, so they run for an hour. The two
// password counters guard a bcrypt comparison that answers a yes/no
// question about a credential, so they run for as long as the thing
// they protect stays useful — the reset window is the token's own
// lifetime, and there is no point letting a probe budget outlive the
// link it is spent on.
var rateLimits = map[rateLimitScope]rateLimit{
	rateLimitScopeLoginIdentifier: {max: 10, window: 15 * time.Minute},
	rateLimitScopeLoginIP:         {max: 50, window: 15 * time.Minute},
	rateLimitScopeLoginCodeEmail:  {max: 5, window: time.Hour},
	rateLimitScopeLoginCodeIP:     {max: 20, window: time.Hour},
	rateLimitScopePasswordEmail:   {max: 3, window: time.Hour},
	rateLimitScopePasswordIP:      {max: 10, window: time.Hour},
	rateLimitScopePasswordReset:   {max: 5, window: 15 * time.Minute},
	rateLimitScopePasswordEdit:    {max: 5, window: 15 * time.Minute},
	rateLimitScopeReauth:          {max: 5, window: 15 * time.Minute},

	// Generous per person, because inviting in bursts is the legitimate
	// pattern — an administrator onboarding a department does thirty in an
	// afternoon, and a budget that makes real work feel broken gets raised
	// until it means nothing. The pending-invitation cap is what bounds an
	// ordinary member; this bounds the relay.
	rateLimitScopeInvitePerson: {max: 30, window: time.Hour},

	// Tight per recipient, and over a day rather than an hour. Being invited
	// twice in a week is an oversight; three times in a day is somebody being
	// mailed by a site they have chosen not to join.
	rateLimitScopeInviteEmail: {max: 3, window: 24 * time.Hour},

	// Same shape as the forgot-password counter, and for the same reason:
	// three is enough for a link that went to spam, and few enough that the
	// endpoint is not a mail cannon.
	rateLimitScopeSignupConfirm: {max: 3, window: time.Hour},

	// Three accounts an hour from one address covers a family, a couple
	// sharing a laptop, and somebody who mistyped their email twice. It does
	// not cover a script.
	rateLimitScopeSignupIP: {max: 3, window: time.Hour},

	// Ten an hour across a whole /24 or /64. High enough that an office or a
	// carrier NAT is not the thing that trips it, low enough that a range
	// walked one address at a time still runs out.
	rateLimitScopeSignupSubnet: {max: 10, window: time.Hour},
}

// signupSubnet reduces an address to the block it sits in, so a caller
// spending one address per account is still counted together.
//
// /24 for IPv4 and /64 for IPv6: the smallest blocks routinely allocated to a
// single customer. Going wider reaches across an ISP and meters strangers;
// going narrower is the per-address counter again.
//
// An address that does not parse returns "", which rateLimitExceeded treats as
// "nothing to key on" and does not count — the right answer, since lumping
// every unparseable caller into one bucket would let them exhaust a budget on
// everybody's behalf.
func signupSubnet(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}

	if v4 := parsed.To4(); v4 != nil {
		return v4.Mask(net.CIDRMask(24, 32)).String() + "/24"
	}

	return parsed.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

func rateLimitKey(scope rateLimitScope, subject string) string {
	return ratelimit.Key(string(scope), subject)
}

// rateLimitExceeded counts one attempt against a scope and reports
// whether the caller is now over budget.
//
// It COUNTS. Every call is an attempt spent, so it belongs on the path
// where the attempt actually happens and nowhere else — calling it to
// "check" without intending to consume quietly halves the budget.
//
// An empty subject is not counted and never limited: it means the
// caller had nothing to key on (no client IP, say), and lumping every
// such request into one shared counter would let anonymous traffic
// exhaust a budget on everyone's behalf.
//
// Fails OPEN. A store that is down, or the cookie backend that has
// nowhere to count, both report "not exceeded" and log. This is the
// uncomfortable half of the design and it is chosen rather than
// overlooked: failing closed turns a Redis outage into a service where
// nobody can reset a password or change one, and account recovery is
// exactly the thing that must keep working on a bad day. The exposure
// while the store is down is the exposure this service had before
// these counters existed.
func (env *Env) rateLimitExceeded(scope rateLimitScope, subject string) bool {
	exceeded, _ := env.rateLimitCheck(scope, subject)
	return exceeded
}

// rateLimitCheck is rateLimitExceeded plus whether this call is the one that
// crossed the line.
//
// The second return exists for the handlers that want to TELL somebody a limit
// tripped. Mailing on every rejected attempt would turn a guessing attack into
// a mail flood aimed at the person being attacked — burying the warning in the
// noise it generates, and making the notification the more effective attack of
// the two. Notifying only on the first crossing gives exactly one message per
// window.
//
// "First" is read off the counter rather than kept in a second key: the count
// passes through limit.max exactly once per window, because the window's TTL
// is set on the first increment and the counter only rises.
//
// A counter failure reports neither exceeded nor first. Failing open on the
// limit is the existing behaviour — an outage in the session store should not
// lock everyone out of signing in — and it follows that a limit which did not
// trip cannot have just tripped.
func (env *Env) rateLimitCheck(scope rateLimitScope, subject string) (exceeded, first bool) {
	limit, ok := rateLimits[scope]
	if !ok {
		log.Printf("rate limit: unknown scope %q, not limiting", scope)
		return false, false
	}

	// The counting itself is shared with the other services: see
	// anahita-libs/ratelimit.
	return ratelimit.Check(env.Session, string(scope), subject,
		ratelimit.Limit{Max: limit.max, Window: limit.window})
}

// rateLimitReset drops a counter.
//
// For budgets that exist to bound failed attempts: once the thing the
// attempts were aiming at has succeeded, the failures before it are no
// longer evidence of anything. Best-effort — a counter that survives
// costs the person a wait, not access.
func (env *Env) rateLimitReset(scope rateLimitScope, subject string) {
	ratelimit.Reset(env.Session, string(scope), subject)
}
