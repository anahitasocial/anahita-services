// Package templates carries the message bodies used by the Send RPC, compiled
// into the binary.
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
