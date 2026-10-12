// Package shell fills the web app's index.html with each page's own title,
// description and picture, so that a link to a profile or a post shows as
// itself where it is shared. The same page is sent to everybody: a browser
// starts the app from it, and a crawler, which runs no JavaScript, reads
// the tags.
package shell

import (
	"errors"
	"html"
	"regexp"
	"strings"
)

// Tags is what a page says of itself.
type Tags struct {
	Title       string
	Description string
	// The page's own address, and its picture's. Absolute.
	URL   string
	Image string
	// What the picture shows.
	ImageAlt string
	// "website", "profile" or "article".
	Type string
	// A picture that is the content, a photo or a cover, is shown large;
	// one that stands for somebody, an avatar, small.
	LargeImage bool
}

// Template is the app's index.html, with the tags it was built with taken
// out and the place they go marked.
type Template struct {
	before string
	after  string
	// What the build said of the whole site: the default for every page.
	SiteName        string
	SiteDescription string
}

var (
	titleTag  = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	describes = regexp.MustCompile(`(?is)<meta\s+name="description"\s+content="(.*?)"\s*/?>`)
	// Every tag this package writes, wherever the build has one of its own.
	written = regexp.MustCompile(`(?is)\s*<(?:title>.*?</title|meta\s+(?:property="og:[^"]*"|name="(?:description|twitter:[^"]*)")[^>]*|link\s+rel="canonical"[^>]*)>`)
)

var ErrNoHead = errors.New("the page has no </head> to put tags before")

// NewTemplate reads an index.html.
func NewTemplate(page string) (*Template, error) {
	t := &Template{}

	if found := titleTag.FindStringSubmatch(page); found != nil {
		t.SiteName = html.UnescapeString(strings.TrimSpace(found[1]))
	}
	if found := describes.FindStringSubmatch(page); found != nil {
		t.SiteDescription = html.UnescapeString(strings.TrimSpace(found[1]))
	}

	end := strings.Index(strings.ToLower(page), "</head>")
	if end < 0 {
		return nil, ErrNoHead
	}
	t.before = written.ReplaceAllString(page[:end], "")
	t.after = page[end:]

	return t, nil
}

// Render is the page with these tags in its head. Everything is escaped:
// a name or a post is somebody's text, and here it is written into HTML.
func (t *Template) Render(tags Tags) string {
	var b strings.Builder
	b.Grow(len(t.before) + len(t.after) + 1024)
	b.WriteString(t.before)

	title := t.SiteName
	if tags.Title != "" && tags.Title != t.SiteName {
		title = tags.Title + " - " + t.SiteName
	}
	ogTitle := tags.Title
	if ogTitle == "" {
		ogTitle = t.SiteName
	}
	description := tags.Description
	if description == "" {
		description = t.SiteDescription
	}
	kind := tags.Type
	if kind == "" {
		kind = "website"
	}

	b.WriteString("<title>" + html.EscapeString(title) + "</title>")
	meta := func(attribute, name, content string) {
		if content == "" {
			return
		}
		b.WriteString(`<meta ` + attribute + `="` + name + `" content="` + html.EscapeString(content) + `"/>`)
	}
	meta("name", "description", description)
	meta("property", "og:title", ogTitle)
	meta("property", "og:description", description)
	meta("property", "og:type", kind)
	meta("property", "og:site_name", t.SiteName)
	meta("property", "og:url", tags.URL)
	meta("property", "og:image", tags.Image)
	if tags.Image != "" {
		meta("property", "og:image:alt", tags.ImageAlt)
	}
	if tags.Image != "" && tags.LargeImage {
		meta("name", "twitter:card", "summary_large_image")
	} else {
		meta("name", "twitter:card", "summary")
	}
	if tags.URL != "" {
		b.WriteString(`<link rel="canonical" href="` + html.EscapeString(tags.URL) + `"/>`)
	}

	b.WriteString(t.after)
	return b.String()
}
