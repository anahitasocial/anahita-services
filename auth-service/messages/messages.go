// Package messages holds the copy for every notification this service sends.
//
// Constants only, no logic. Notifiers format them; handlers never build copy.
//
// # Two kinds of placeholder, and why they are different
//
// `%s` / `%d` are ordinary fmt verbs, filled by the notifier with values
// specific to that message — names, a passkey nickname, a session count. Their
// order matters and is documented above each constant.
//
// SiteToken is the installation's own name, and is deliberately NOT a verb.
//
// The site name has been added to these messages once before and removed again,
// because it was written as a `%s` on constants that the notifier passed to the
// mailer raw rather than through Sprintf — so real recipients read "Reset your
// password for %s" and "Welcome to %s". Four files still carried NOTE blocks
// recording that.
//
// A token avoids reopening it. Subjects and titles can name the site without
// becoming format strings at all, bodies keep the verb order they already have,
// and adding the site to a message cannot shift an argument list underneath a
// value that was already correct. Substitution happens in one place —
// Env.site in the notifiers package — rather than at thirty call sites.
package messages

// SiteToken is replaced with the installation's name wherever it appears.
//
// The braces are conventional rather than functional: these strings are values
// passed to the mail service, not templates it parses, so the token is only
// ever replaced by our own ReplaceAll.
const SiteToken = "{{site}}"
