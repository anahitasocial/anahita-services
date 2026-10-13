package entities

import (
	"regexp"
	"strings"

	"anahita-libs/site"
)

const MentionRegexPattern = `(?:^|\s)@([\w-]+)`

// A host after a second @: letters, digits, dots and hyphens, with a dot in
// it, beginning and ending in a letter or digit. So the full stop that ends
// a sentence is not taken for part of the name.
const mentionHostPattern = `^@([A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)+)`

var (
	mentionRegex     = regexp.MustCompile(MentionRegexPattern)
	mentionHostRegex = regexp.MustCompile(mentionHostPattern)
)

// Mention is somebody named in a piece of text.
type Mention struct {
	// User is the name after the first @.
	User string
	// Host is the name after a second @, in lower case: mastodon.social in
	// @alice@mastodon.social. Empty when there was none, which is somebody
	// here.
	Host string
	// Start and End are where it is in the text, from its first @ to just
	// past its last character.
	Start, End int
}

// IsLocal says whether the person named is on this installation: no host was
// given, or the one given is ours.
func (m Mention) IsLocal(domain string) bool {
	return m.Host == "" || strings.EqualFold(m.Host, domain)
}

// ParseMentions finds everybody named in a piece of text, in the order they
// are named: @ana, and @alice@mastodon.social.
//
// A name begins the text or follows a space, as it always has, so the
// address in "write to ana@example.org" names nobody.
//
// THE SECOND @ IS WHY THIS EXISTS. Read as it used to be, "@alice@mastodon.
// social" named whoever is called alice here, who was then told that
// somebody had mentioned them. Somebody elsewhere who shares a name with
// somebody here is not them. So a name followed by @ and a host is that
// person, there; and a name followed by @ and something that is no host is
// nobody at all.
func ParseMentions(text string) []Mention {
	mentions := []Mention{}

	for _, at := range mentionRegex.FindAllStringSubmatchIndex(text, -1) {
		mention := Mention{
			User:  text[at[2]:at[3]],
			Start: at[2] - 1,
			End:   at[3],
		}

		rest := text[mention.End:]
		if strings.HasPrefix(rest, "@") {
			host := mentionHostRegex.FindStringSubmatch(rest)
			if host == nil {
				continue
			}
			mention.Host = strings.ToLower(host[1])
			mention.End += len(host[0])
		}

		mentions = append(mentions, mention)
	}

	return mentions
}

type MentionMixinInterface interface {
	GetMentions() []string
}

// GetMentions is the usernames of the people here that the node's text
// names. Somebody named on another server is not among them, whatever they
// are called.
func (a *AnahitaNode) GetMentions() []string {
	if a.Body == "" {
		return []string{}
	}

	domain := site.FromEnv().Domain

	a.Mentions = []string{}
	for _, mention := range ParseMentions(a.Body) {
		if mention.IsLocal(domain) {
			a.Mentions = append(a.Mentions, mention.User)
		}
	}

	// make sure that m.Mentions has unnique values
	a.Mentions = UniqueString(a.Mentions)

	return a.Mentions
}
