package entities

import "time"

// Consent is one recorded acceptance — of the Terms of Service, or of the
// Privacy Policy. Both are the same shape, so both use this.
//
// WHETHER somebody needs to accept again is not decided here. The web app holds
// the current version of each document and compares, numerically and part by
// part, so that 1.0 -> 1.1 asks again and a rollback does not. The server only
// records what was accepted and the circumstances of it.
//
// A boolean on its own answers "did they agree?" and nothing else. It cannot
// say WHICH text they agreed to, so a revision has no way to find who still
// needs asking; and it cannot say from where, so it is an assertion rather
// than a record. The version and the audit trio are what make it evidence.
type Consent struct {
	Accepted bool

	// AcceptedAt is nil when nothing was accepted.
	//
	// Deliberately a pointer rather than a zero time: the pair
	// (Accepted=false, AcceptedAt=<a date>) is unreadable, and a zero time is
	// also '0000-00-00' to MySQL, which strict mode refuses outright.
	AcceptedAt *time.Time

	// Version of the text that was shown. Compared against the version in
	// force to decide who needs re-prompting.
	Version string

	// The audit trail. Written once and never read by the application — they
	// exist for the question "who accepted this, and from where", which is
	// asked by a person looking at a row, not by code.
	IP        string
	UserAgent string
	OS        string
}

// Accept records an acceptance as of now.
func (c *Consent) Accept(version, ip, userAgent, os string) {
	now := time.Now()

	c.Accepted = true
	c.AcceptedAt = &now
	c.Version = version
	c.IP = ip
	c.UserAgent = userAgent
	c.OS = os
}
