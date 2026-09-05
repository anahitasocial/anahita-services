// Package templates carries the message bodies, compiled into the binary.
//
// message_action.{html,txt}.tmpl are the live pair — see messageTemplates in
// the handlers package for the registry that maps a wire template name onto
// them. They replaced a template hosted in a Mailgun account, which meant the
// appearance of every email this system sends used to live outside version
// control.
//
// mail.{html,plain}.tmpl are the older pair and are now referenced by nothing:
// they served the Send RPC, which has no callers and no longer renders through
// a template. Left in place rather than deleted, because removing them belongs
// with removing that RPC and its proto fields — one change, separately.
//
// They used to be read from disk through a CWD-relative path
// ("../../templates/mail.plain.tmpl"), which resolved correctly only when the
// process happened to be started from cmd/server. In the container, where
// WORKDIR is /app and the template files are not copied at all, it could never
// resolve — so the Send RPC would have failed on its first real call. It has no
// callers yet, which is the only reason that has not surfaced.
//
// Embedding removes the question. The templates travel with the binary and the
// working directory stops mattering.
package templates

import "embed"

// FS holds the message templates. Parse with template.ParseFS and execute the
// "body" template defined inside each file.
//
//go:embed *.tmpl
var FS embed.FS
