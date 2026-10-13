package bsky

import (
	"encoding/json"
	"html"
	"net/url"
	"sort"
	"strings"
	"time"
)

// What a post says, made into what is kept here.
//
// A Bluesky post is plain words, with a list beside them saying which
// stretch is a link, which somebody's name and which a hashtag. Here a
// post is Markdown. The words are first made into HTML, with every one of
// them escaped, and from there go the way a post from any other server
// does: cut down to what a post may be made of, then converted.

// HTML is a post's words as HTML: links as links, people's names as links
// to their pages, hashtags left as they were written, and line breaks
// kept.
func (p *Post) HTML() string {
	return richText(p.Text, p.Facets)
}

// PlainHTML is words with nothing marked in them, as HTML: what an
// account says of itself.
func PlainHTML(text string) string {
	return richText(text, nil)
}

func richText(text string, facets []Facet) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}

	type stretch struct {
		start, end int
		href       string
	}
	var linked []stretch
	for _, facet := range facets {
		start, end := facet.Index.ByteStart, facet.Index.ByteEnd
		if start < 0 || end <= start || end > len(text) {
			continue
		}
		for _, feature := range facet.Features {
			href := ""
			switch feature.Type {
			case "app.bsky.richtext.facet#link":
				if parsed, err := url.Parse(feature.URI); err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" {
					href = feature.URI
				}
			case "app.bsky.richtext.facet#mention":
				if ValidDID(feature.DID) {
					href = ProfilePage(feature.DID)
				}
			}
			if href != "" {
				linked = append(linked, stretch{start, end, href})
				break
			}
		}
	}
	sort.Slice(linked, func(i, j int) bool { return linked[i].start < linked[j].start })

	var out strings.Builder
	out.WriteString("<p>")
	at := 0
	for _, each := range linked {
		// One that begins inside the one before is left as words.
		if each.start < at {
			continue
		}
		out.WriteString(escaped(text[at:each.start]))
		out.WriteString(`<a href="` + html.EscapeString(each.href) + `">` + escaped(text[each.start:each.end]) + `</a>`)
		at = each.end
	}
	out.WriteString(escaped(text[at:]))
	out.WriteString("</p>")
	return out.String()
}

// escaped is words safe to put in HTML, with their line breaks kept. A
// stretch cut in the middle of a letter is mended, not passed on broken.
func escaped(text string) string {
	text = html.EscapeString(strings.ToValidUTF8(text, ""))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n\n", "</p><p>")
	return strings.ReplaceAll(text, "\n", "<br>")
}

// IsReply says whether a post answers another.
func (p *Post) IsReply() bool {
	return len(p.Reply) > 0 && string(p.Reply) != "null"
}

// Answers is the address of the post this one answers, or "".
func (p *Post) Answers() string {
	if !p.IsReply() {
		return ""
	}
	var reply struct {
		Parent struct {
			URI string `json:"uri"`
		} `json:"parent"`
	}
	if json.Unmarshal(p.Reply, &reply) != nil {
		return ""
	}
	if _, _, ok := ParsePostURI(reply.Parent.URI); !ok {
		return ""
	}
	return reply.Parent.URI
}

// What an author says of their own post that makes its pictures ones not
// to be shown plainly.
var sensitiveLabels = map[string]bool{
	"porn": true, "sexual": true, "nudity": true, "graphic-media": true, "gore": true,
}

// IsSensitive says whether its author marked a post as one to be warned
// of.
func (p *Post) IsSensitive() bool {
	if p.Labels == nil {
		return false
	}
	for _, label := range p.Labels.Values {
		if sensitiveLabels[label.Val] {
			return true
		}
	}
	return false
}

// Language is the first language its author said a post is in, or "".
func (p *Post) Language() string {
	if len(p.Langs) == 0 || len(p.Langs[0]) > 35 {
		return ""
	}
	return p.Langs[0]
}

// Created is when its author says a post was written, and never later
// than `now`: the time is the author's word, and a post is not kept as
// one from the future.
func (p *Post) Created(now time.Time) time.Time {
	created, err := time.Parse(time.RFC3339, p.CreatedAt)
	if err != nil || created.After(now) {
		return now
	}
	return created.UTC()
}

// media is what a post carries that is not another post: its own embed,
// or the half of one that came with a quote.
func (p *Post) media() *Embed {
	if p.Embed == nil {
		return nil
	}
	if p.Embed.Media != nil {
		return p.Embed.Media
	}
	return p.Embed
}

// PictureOf is one picture of a post: which file it is, and what its
// author said it shows.
type PictureOf struct {
	CID string
	Alt string
}

// Pictures is the pictures a post carries, in the order they are shown.
func (p *Post) Pictures() []PictureOf {
	media := p.media()
	if media == nil {
		return nil
	}
	var pictures []PictureOf
	for _, each := range media.Images {
		if !cidPattern.MatchString(each.Image.Ref.Link) || !strings.HasPrefix(each.Image.MimeType, "image/") {
			continue
		}
		pictures = append(pictures, PictureOf{CID: each.Image.Ref.Link, Alt: each.Alt})
	}
	return pictures
}

// PictureURL is where a picture of a post is fetched from: `cdn` is the
// network's store of them ("https://cdn.bsky.app").
func PictureURL(cdn, did, cid string) string {
	return cdn + "/img/feed_fullsize/plain/" + did + "/" + cid + "@jpeg"
}

// Link is the page a post carries a card of, or "": one named apart from
// its words, which may not have the address in them at all.
func (p *Post) Link() string {
	media := p.media()
	if media == nil || media.External == nil {
		return ""
	}
	parsed, err := url.Parse(media.External.URI)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || len(media.External.URI) > 2000 {
		return ""
	}
	return media.External.URI
}

// Quoted is the page of the post this one quotes, or "".
func (p *Post) Quoted() string {
	if p.Embed == nil || len(p.Embed.Record) == 0 {
		return ""
	}
	// Alone, it is {uri}; with pictures beside it, {record: {uri}}.
	var quoted struct {
		URI    string `json:"uri"`
		Record struct {
			URI string `json:"uri"`
		} `json:"record"`
	}
	if json.Unmarshal(p.Embed.Record, &quoted) != nil {
		return ""
	}
	uri := quoted.URI
	if uri == "" {
		uri = quoted.Record.URI
	}
	did, rkey, ok := ParsePostURI(uri)
	if !ok {
		return ""
	}
	return PostPage(did, rkey)
}
