package repositories

import "strings"

// newlines writes every line break of a text as \n, whatever sent it.
//
// A browser sends a form's text with \r\n, as forms are sent; an API client
// may send either; an old Mac wrote \r. Kept as they came, the same post was
// stored differently by who wrote it, and a code block carried \r into what
// a reader copied. Only the line breaks change: a text is shown, and given
// back to be edited, as it was composed (the user, 2026-10-16).
func newlines(text string) string {
	if !strings.Contains(text, "\r") {
		return text
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}
