// Package embeds finds what a post shows within its text: an address on a
// line of its own. A post of this installation is shown as itself and a
// page elsewhere by its preview; an address inside a sentence stays a link.
//
// The web app reads the text the same way to draw it
// (anahita-web src/utils/embeds.js). The two say the same, or a post is
// drawn with something its edges do not know of.
package embeds

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"anahita-libs/linkpreviews"
)

// The most of each a post is taken to embed. A post may hold more
// addresses on lines of their own; past these they are links.
const (
	MaxPosts = 20
	MaxPages = 10
)

// The kinds of post an address of this installation can be to, by the
// first part of its path.
var kinds = map[string]string{
	"articles": "article",
	"topics":   "topic",
	"notes":    "note",
	"photos":   "photo",
	"videos":   "video",
}

// A link whose words are its own address, as some editors write an address
// on a line of its own: [https://a.example/](https://a.example/).
var selfLink = regexp.MustCompile(`^\[([^\]]+)\]\((\S+)\)$`)

// Markdown's escapes, taken out of a link's words to compare them with its
// address: \_ is _.
var escaped = regexp.MustCompile(`\\([\\\x60*_{}\[\]()#+\-.!|~<>])`)

// A post's address: /articles/12, or /articles/12-its-title.
var postPath = regexp.MustCompile(`^/([a-z]+)/([0-9]+)(?:-[^/]*)?/?$`)

// Post is a post of this installation a text embeds.
type Post struct {
	Kind string
	ID   int64
}

// Found is what a text embeds, each in the order it comes, once.
type Found struct {
	Posts []Post
	// Pages elsewhere, tidied as link previews keep them.
	Pages []string
	// Profiles of this installation, by alias: /@ana, or /@alice@host for
	// somebody elsewhere. Shown as their card, and named: a mention, as an
	// @handle is, not a citation (actor-service keeps the edges).
	Profiles []string
}

// The most profiles a post is taken to name.
const MaxProfiles = 20

// A profile's address: /@ana and its tabs.
var profilePath = regexp.MustCompile(`^/@([^/]+)(?:/.*)?$`)

// profileAt reads which profile an address of this installation is to.
func profileAt(address string) (string, bool) {
	parsed, err := url.Parse(address)
	if err != nil {
		return "", false
	}
	match := profilePath.FindStringSubmatch(parsed.Path)
	if match == nil || match[1] == "" {
		return "", false
	}
	return match[1], true
}

// Find reads what a text embeds. `ours` says whether an address is one of
// this installation's (site.Addresses.IsOurs).
func Find(text string, ours func(string) bool) Found {
	found := Found{}
	seenPosts := map[int64]bool{}
	seenPages := map[string]bool{}
	seenProfiles := map[string]bool{}

	for _, line := range strings.Split(text, "\n") {
		address, ok := alone(line)
		if !ok {
			continue
		}

		if ours != nil && ours(address) {
			post, ok := postAt(address)
			if ok && !seenPosts[post.ID] && len(found.Posts) < MaxPosts {
				seenPosts[post.ID] = true
				found.Posts = append(found.Posts, post)
			}
			alias, isProfile := profileAt(address)
			if !ok && isProfile && !seenProfiles[alias] && len(found.Profiles) < MaxProfiles {
				seenProfiles[alias] = true
				found.Profiles = append(found.Profiles, alias)
			}
			continue
		}

		page, ok := linkpreviews.Normalise(address)
		if ok && !seenPages[page] && len(found.Pages) < MaxPages {
			seenPages[page] = true
			found.Pages = append(found.Pages, page)
		}
	}

	return found
}

// alone says whether a line is an address and nothing else, as Markdown
// writes one: bare, in angle brackets, or as a link whose words are the
// address.
func alone(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if match := selfLink.FindStringSubmatch(line); match != nil {
		if escaped.ReplaceAllString(match[1], "$1") != match[2] {
			return "", false
		}
		line = match[2]
	}
	if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
		line = strings.TrimSpace(line[1 : len(line)-1])
	}
	if line == "" || strings.ContainsAny(line, " \t") {
		return "", false
	}
	if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
		return "", false
	}
	return line, true
}

// postAt reads which post an address of this installation is to.
func postAt(address string) (Post, bool) {
	parsed, err := url.Parse(address)
	if err != nil {
		return Post{}, false
	}
	match := postPath.FindStringSubmatch(parsed.Path)
	if match == nil {
		return Post{}, false
	}
	kind, ok := kinds[match[1]]
	if !ok {
		return Post{}, false
	}
	id, err := strconv.ParseInt(match[2], 10, 64)
	if err != nil || id <= 0 {
		return Post{}, false
	}
	return Post{Kind: kind, ID: id}, true
}
