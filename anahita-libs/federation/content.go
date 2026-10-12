// Package federation is what the services that speak to other servers
// share, whatever protocol they speak.
//
// This file is the text of a post, there and back. A post here is written in
// Markdown and shown by the web app; other servers send and expect HTML.
// Both ways the HTML is somebody else's to trust: what is sent will be shown
// to readers of servers this one has never heard of, and what arrives was
// written by anybody at all. So nothing leaves or enters without being cut
// down to the few harmless things a post is made of.
package federation

import (
	"bytes"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"anahita-libs/data/entities"
)

// Links says where the people and hashtags named in a post are, for the
// links made of them.
type Links struct {
	// Mention is the address of somebody named: for @ana, ("ana", ""); for
	// @alice@mastodon.social, ("alice", "mastodon.social"). Empty for
	// somebody who cannot be found, whose name is then left as written.
	Mention func(user, host string) string
	// Hashtag is the address of a hashtag's page, given without its #.
	Hashtag func(tag string) string
}

// Named is a person or a hashtag that a post's text names, and where they
// are. What a protocol lists beside the text, so that the server receiving
// it knows who to tell.
type Named struct {
	// Name is as written, with its sign: "@alice@mastodon.social", "#garden".
	Name string
	// Href is the address the name links to.
	Href string
}

// Rendered is a post's text as other servers are sent it.
type Rendered struct {
	HTML     string
	Mentions []Named
	Hashtags []Named
}

var (
	markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

	hashtagRegex = regexp.MustCompile(entities.HashtagRegexPattern)

	// What a post may be made of, going out and coming in: paragraphs and
	// line breaks, emphasis, quotations, lists, code, headings and links.
	// No pictures (those travel beside the text, as attachments), nothing
	// that runs, nothing that styles, and links to the web only.
	policy = func() *bluemonday.Policy {
		p := bluemonday.NewPolicy()
		p.AllowElements(
			"p", "br", "strong", "b", "em", "i", "del", "s", "u",
			"blockquote", "ul", "ol", "li", "code", "pre",
			"h1", "h2", "h3", "h4", "h5", "h6", "span",
		)
		p.AllowAttrs("href").OnElements("a")
		p.AllowURLSchemes("http", "https")
		p.RequireParseableURLs(true)
		p.AllowRelativeURLs(false)
		// How the servers that speak ActivityPub mark the people and
		// hashtags in a post, and the parts of a long link they hide. These
		// names and no others.
		p.AllowAttrs("class").Matching(regexp.MustCompile(`^(?:h-card|u-url|mention|hashtag|invisible|ellipsis)(?: (?:h-card|u-url|mention|hashtag|invisible|ellipsis))*$`)).OnElements("a", "span")
		p.AllowAttrs("rel").Matching(regexp.MustCompile(`^tag$`)).OnElements("a")
		p.RequireNoFollowOnLinks(false)
		return p
	}()
)

// ToHTML turns a post's Markdown into the HTML other servers are sent, with
// the people and hashtags it names made into links and listed.
//
// Names are found in what is read, not in what was typed: one inside a code
// span or a link's own text is part of that and is left alone. Whatever HTML
// was typed into the Markdown is not passed on as HTML; it arrives as the
// text it is.
func ToHTML(source string, links Links) Rendered {
	var out bytes.Buffer
	if err := markdown.Convert([]byte(source), &out); err != nil {
		// Markdown has no errors of its own; a writer's would land here.
		// Send the text as it is, escaped, sooner than nothing.
		out.Reset()
		out.WriteString("<p>" + html.EscapeString(source) + "</p>")
	}

	rendered := Rendered{Mentions: []Named{}, Hashtags: []Named{}}

	nodes, err := html.ParseFragment(&out, &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		rendered.HTML = policy.Sanitize(out.String())
		return rendered
	}

	var linked bytes.Buffer
	for _, node := range nodes {
		link(node, links, &rendered)
		_ = html.Render(&linked, node)
	}

	rendered.HTML = strings.TrimSpace(policy.Sanitize(linked.String()))
	return rendered
}

// ToMarkdown turns the HTML of a post that arrived from another server into
// the Markdown a post here is kept as, having first cut it down to what a
// post may be made of.
//
// It is safe to keep and to show whatever was sent: a script, a picture
// that reports who opened it, a link that is not to the web are all gone
// before anything is converted.
func ToMarkdown(source string) string {
	clean := policy.Sanitize(source)

	converted, err := htmltomarkdown.ConvertString(clean)
	if err != nil {
		// What is left when every tag is taken away.
		return strings.TrimSpace(html.UnescapeString(bluemonday.StrictPolicy().Sanitize(clean)))
	}

	return strings.TrimSpace(converted)
}

// link makes links of the names in the text under a node, in place.
func link(node *html.Node, links Links, rendered *Rendered) {
	if node.Type == html.ElementNode {
		switch node.DataAtom {
		case atom.A, atom.Code, atom.Pre:
			return
		}
	}

	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.TextNode {
			linkText(child, links, rendered)
		} else {
			link(child, links, rendered)
		}
		child = next
	}
}

// A name found in a run of text: where it is, and the link to put there.
type found struct {
	start, end int
	link       *html.Node
}

// linkText replaces one run of text with itself in pieces, the names in it
// as links.
func linkText(text *html.Node, links Links, rendered *Rendered) {
	source := text.Data
	var names []found

	if links.Mention != nil {
		for _, mention := range entities.ParseMentions(source) {
			href := links.Mention(mention.User, mention.Host)
			if href == "" {
				continue
			}
			name := source[mention.Start:mention.End]
			rendered.Mentions = appendNamed(rendered.Mentions, Named{Name: strings.ToLower(name), Href: href})
			names = append(names, found{mention.Start, mention.End, mentionLink(mention.User, href)})
		}
	}

	if links.Hashtag != nil {
		for _, at := range hashtagRegex.FindAllStringSubmatchIndex(source, -1) {
			tag := source[at[2]:at[3]]
			href := links.Hashtag(strings.ToLower(tag))
			if href == "" {
				continue
			}
			rendered.Hashtags = appendNamed(rendered.Hashtags, Named{Name: "#" + strings.ToLower(tag), Href: href})
			names = append(names, found{at[2] - 1, at[3], hashtagLink(tag, href)})
		}
	}

	if len(names) == 0 {
		return
	}

	// In the order they are written. A mention and a hashtag cannot overlap:
	// each begins at its own sign and neither sign is part of the other.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j].start < names[j-1].start; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}

	parent := text.Parent
	at := 0
	for _, name := range names {
		if name.start < at {
			continue
		}
		if name.start > at {
			parent.InsertBefore(&html.Node{Type: html.TextNode, Data: source[at:name.start]}, text)
		}
		parent.InsertBefore(name.link, text)
		at = name.end
	}
	if at < len(source) {
		parent.InsertBefore(&html.Node{Type: html.TextNode, Data: source[at:]}, text)
	}
	parent.RemoveChild(text)
}

// mentionLink is a person named, marked up the way the servers that speak
// ActivityPub mark one, which is what makes their apps open the person's
// profile where the reader is and not in a browser:
//
//	<span class="h-card"><a href="…" class="u-url mention">@<span>ana</span></a></span>
func mentionLink(user, href string) *html.Node {
	card := element(atom.Span, html.Attribute{Key: "class", Val: "h-card"})
	anchor := element(atom.A, html.Attribute{Key: "href", Val: href}, html.Attribute{Key: "class", Val: "u-url mention"})
	name := element(atom.Span)
	name.AppendChild(&html.Node{Type: html.TextNode, Data: user})
	anchor.AppendChild(&html.Node{Type: html.TextNode, Data: "@"})
	anchor.AppendChild(name)
	card.AppendChild(anchor)
	return card
}

// hashtagLink is a hashtag, marked up likewise:
//
//	<a href="…" class="mention hashtag" rel="tag">#<span>garden</span></a>
func hashtagLink(tag, href string) *html.Node {
	anchor := element(atom.A,
		html.Attribute{Key: "href", Val: href},
		html.Attribute{Key: "class", Val: "mention hashtag"},
		html.Attribute{Key: "rel", Val: "tag"},
	)
	name := element(atom.Span)
	name.AppendChild(&html.Node{Type: html.TextNode, Data: tag})
	anchor.AppendChild(&html.Node{Type: html.TextNode, Data: "#"})
	anchor.AppendChild(name)
	return anchor
}

func element(tag atom.Atom, attributes ...html.Attribute) *html.Node {
	return &html.Node{Type: html.ElementNode, DataAtom: tag, Data: tag.String(), Attr: attributes}
}

// appendNamed adds a name once, however often the post says it.
func appendNamed(list []Named, named Named) []Named {
	for _, have := range list {
		if have.Name == named.Name {
			return list
		}
	}
	return append(list, named)
}
