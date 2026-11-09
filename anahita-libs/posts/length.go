// Package posts holds what is the same of every short post wherever it is
// written: how long it may be, and how its length is counted (roadmap row
// 144), so that every service and the web app count it alike.
package posts

import (
	"regexp"
	"unicode/utf8"
)

// MaxLength is how many characters a short post may have: a note, a reply,
// a thread's note, and the words of a photo, video or audio post. As on
// Mastodon and Threads; anything longer is a thread, an article or a topic.
const MaxLength = 500

// LinkLength is what a link counts as, however long its address: as on
// Mastodon, which shows a link shortened.
const LinkLength = 23

var (
	// A link as Markdown writes it: its words count, not its address.
	markdownLink = regexp.MustCompile(`\[([^\]\n]*)\]\((https?://[^)\s]+)\)`)
	// An address standing alone.
	address = regexp.MustCompile(`https?://[^\s<>)\]]+`)
)

// Length is how long a post's words are, as the limit counts them: each
// character once, Markdown's marks as typed, a link standing alone as
// LinkLength, and a Markdown link as its words.
func Length(markdown string) int {
	length := 0
	rest := markdownLink.ReplaceAllStringFunc(markdown, func(link string) string {
		words := markdownLink.FindStringSubmatch(link)[1]
		length += utf8.RuneCountInString(words)
		return ""
	})
	rest = address.ReplaceAllStringFunc(rest, func(string) string {
		length += LinkLength
		return ""
	})
	return length + utf8.RuneCountInString(rest)
}

// Fits is whether a post's words are within MaxLength.
func Fits(markdown string) bool {
	return Length(markdown) <= MaxLength
}
