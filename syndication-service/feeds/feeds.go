// Package feeds finds a website's feed and reads it.
//
// A website is the feed of a website (docs/plans/done/website.md): RSS, Atom or JSON
// Feed, whatever made it. Every address is fetched with safehttp, which goes
// only to public addresses on the web's two ports, so that somebody naming
// an address cannot send this service into the installation's own network.
package feeds

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"anahita-libs/safehttp"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
)

const (
	// The most read of a page or a feed, and how long each fetch may take.
	maxBody      = 5 << 20
	fetchTimeout = 10 * time.Second
	maxRedirects = 3
	userAgent    = "Anahita/1.0 (Website; +https://anahita.io)"
	accept       = "application/rss+xml, application/atom+xml, application/feed+json, application/json;q=0.9, application/xml;q=0.8, text/xml;q=0.8, text/html;q=0.5"

	// ExcerptLength is the most of an item's text a post carries, in
	// characters: the opening, cut at a word. The rest is on the site.
	ExcerptLength = 300
	// The most of a title kept: a post's title is at most 100.
	titleLength = 100
)

var (
	// ErrNoFeed is a page that is not a feed and names none.
	ErrNoFeed = errors.New("no feed")
	// ErrUnreadable is an address that could not be fetched, or answered
	// with an error.
	ErrUnreadable = errors.New("could not be read")
	// ErrPrivate is a feed behind a login, or one whose address carries a
	// secret. A website's posts are public, so a private feed is never
	// taken: what it holds was not meant for everybody.
	ErrPrivate = errors.New("a private feed")
)

// The names a secret goes by in an address: a paid podcast's feed, a
// private repository's, a tracker's.
var secretParams = map[string]bool{
	"token": true, "access_token": true, "auth": true, "key": true, "apikey": true,
	"api_key": true, "secret": true, "password": true, "pass": true, "signature": true,
	"sig": true, "private": true, "private_token": true, "feed_token": true,
}

// IsPrivate says whether an address carries what should not be made public:
// a user and password, or a parameter that is a secret.
func IsPrivate(address string) bool {
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	if u.User != nil {
		return true
	}
	for name := range u.Query() {
		if secretParams[strings.ToLower(name)] {
			return true
		}
	}
	return false
}

// The kinds of feed a page may name with <link rel="alternate">.
var feedTypes = map[string]bool{
	"application/rss+xml":   true,
	"application/atom+xml":  true,
	"application/feed+json": true,
	"application/json":      true,
	"application/rdf+xml":   true,
}

// Reader fetches pages and feeds.
type Reader struct {
	Client *http.Client
	parser *gofeed.Parser
}

// NewReader fetches with a client that goes only to public addresses.
func NewReader() *Reader {
	return &Reader{
		Client: safehttp.NewClient(safehttp.Options{Timeout: fetchTimeout, MaxRedirects: maxRedirects}),
		parser: gofeed.NewParser(),
	}
}

// Found is a website's feed, as connecting one shows it.
type Found struct {
	// Where the feed is read, after any redirect.
	FeedURL string
	// What the feed calls itself, and the site it says it is of.
	Title   string
	SiteURL string
	// What the feed says of itself, for the website's description.
	Description string
}

// Find reads an address and answers the feed it is, or the feed the page
// names: the first <link rel="alternate"> of a kind of feed.
func (r *Reader) Find(ctx context.Context, address string) (*Found, error) {
	body, final, err := r.get(ctx, address, "", "")
	if err != nil {
		return nil, err
	}

	if feed, err := r.parser.Parse(bytes.NewReader(body)); err == nil {
		return found(feed, final), nil
	}

	named := alternateOf(body, final)
	if named == "" {
		return nil, ErrNoFeed
	}
	body, final, err = r.get(ctx, named, "", "")
	if err != nil {
		return nil, err
	}
	feed, err := r.parser.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, ErrNoFeed
	}
	return found(feed, final), nil
}

func found(feed *gofeed.Feed, final string) *Found {
	return &Found{
		FeedURL:     final,
		Title:       clip(plain(feed.Title), titleLength),
		SiteURL:     feed.Link,
		Description: clip(plain(feed.Description), 500),
	}
}

// Read is a feed as it was read.
type Read struct {
	// The feed has not changed since it was last read: nothing else is set
	// but what it said the last time.
	NotModified bool
	// What to send the next time, so that an unchanged feed is not sent
	// again.
	ETag         string
	LastModified string
	Items        []Item
	// The WebSub hub the feed names, and the address it names itself by,
	// which is what is subscribed to. Empty when it names no hub.
	Hub  string
	Self string
}

// Item is one entry of a feed, as a post is made of it.
type Item struct {
	// What it is known by: its id, or its address when it has none. Hashed,
	// as it is kept.
	Key string
	// Its title, plain. An item with none is given the opening of its text.
	Title string
	// The opening of its text, plain, cut at a word.
	Excerpt string
	// Where it is on the site.
	Link string
	// Its categories, as hashtags are made of them: one word each.
	Tags []string
	// When it says it was published, or updated; zero when it says neither.
	Published time.Time
}

// Read reads a feed, sending what was said the last time so that a server
// can answer that nothing has changed.
func (r *Reader) Read(ctx context.Context, feedURL, etag, lastModified string) (*Read, error) {
	body, _, res, err := r.fetch(ctx, feedURL, etag, lastModified)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotModified {
		return &Read{NotModified: true, ETag: etag, LastModified: lastModified}, nil
	}

	feed, err := r.parser.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoFeed, err)
	}

	read := &Read{ETag: res.Header.Get("ETag"), LastModified: res.Header.Get("Last-Modified")}
	read.Hub, read.Self = hubOf(body, feedURL)
	for _, entry := range feed.Items {
		if item, ok := itemOf(entry, feedURL); ok {
			read.Items = append(read.Items, item)
		}
	}
	return read, nil
}

func (r *Reader) get(ctx context.Context, address, etag, lastModified string) ([]byte, string, error) {
	body, final, _, err := r.fetch(ctx, address, etag, lastModified)
	return body, final, err
}

func (r *Reader) fetch(ctx context.Context, address, etag, lastModified string) ([]byte, string, *http.Response, error) {
	if IsPrivate(address) {
		return nil, "", nil, ErrPrivate
	}
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, "", nil, ErrUnreadable
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", nil, ErrUnreadable
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	res, err := r.Client.Do(req)
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	defer res.Body.Close()

	final := res.Request.URL.String()
	if res.StatusCode == http.StatusNotModified {
		return nil, final, res, nil
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, final, res, fmt.Errorf("%w: %s answered %d", ErrPrivate, final, res.StatusCode)
	}
	if res.StatusCode != http.StatusOK {
		return nil, final, res, fmt.Errorf("%w: %s answered %d", ErrUnreadable, final, res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return nil, final, res, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return body, final, res, nil
}

// alternateOf is the first feed a page names in its head, made absolute.
func alternateOf(page []byte, base string) string {
	from, err := url.Parse(base)
	if err != nil {
		return ""
	}

	tokens := html.NewTokenizer(bytes.NewReader(page))
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tokens.TagName()
			if string(name) == "body" {
				return ""
			}
			if string(name) != "link" || !hasAttr {
				continue
			}
			var rel, kind, href string
			for {
				key, value, more := tokens.TagAttr()
				switch string(key) {
				case "rel":
					rel = strings.ToLower(string(value))
				case "type":
					kind = strings.ToLower(strings.TrimSpace(string(value)))
				case "href":
					href = strings.TrimSpace(string(value))
				}
				if !more {
					break
				}
			}
			if !hasWord(rel, "alternate") || !feedTypes[kind] || href == "" {
				continue
			}
			to, err := from.Parse(href)
			if err != nil {
				continue
			}
			return to.String()
		}
	}
}

func hasWord(list, word string) bool {
	for _, each := range strings.Fields(list) {
		if each == word {
			return true
		}
	}
	return false
}

func itemOf(entry *gofeed.Item, feedURL string) (Item, bool) {
	link := absolute(entry.Link, feedURL)
	key := strings.TrimSpace(entry.GUID)
	if key == "" {
		key = link
	}
	if key == "" || link == "" {
		return Item{}, false
	}

	text := entry.Description
	if text == "" {
		text = entry.Content
	}
	excerpt := clip(plain(text), ExcerptLength)

	title := clip(plain(entry.Title), titleLength)
	if title == "" {
		title = clip(excerpt, titleLength)
	}
	if title == "" {
		return Item{}, false
	}

	var published time.Time
	switch {
	case entry.PublishedParsed != nil:
		published = *entry.PublishedParsed
	case entry.UpdatedParsed != nil:
		published = *entry.UpdatedParsed
	}

	return Item{
		Key:       KeyOf(key),
		Title:     title,
		Excerpt:   excerpt,
		Link:      link,
		Tags:      tagsOf(entry.Categories),
		Published: published,
	}, true
}

// KeyOf is how an item is kept: a hash of what it is known by, which may
// be long.
func KeyOf(known string) string {
	sum := sha256.Sum256([]byte(known))
	return hex.EncodeToString(sum[:])
}

func absolute(link, base string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}
	from, err := url.Parse(base)
	if err != nil {
		return ""
	}
	to, err := from.Parse(link)
	if err != nil || (to.Scheme != "http" && to.Scheme != "https") {
		return ""
	}
	return to.String()
}

// The most hashtags one item is given from its categories; a post is filed
// under at most ten in all.
const maxTags = 5

// tagsOf makes hashtags of categories: letters and digits only, the words of
// one run together ("Open Science" is #OpenScience), each once.
func tagsOf(categories []string) []string {
	seen := map[string]bool{}
	var tags []string
	for _, category := range categories {
		var tag strings.Builder
		for _, word := range strings.FieldsFunc(category, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}) {
			first, size := utf8.DecodeRuneInString(word)
			tag.WriteRune(unicode.ToUpper(first))
			tag.WriteString(word[size:])
		}
		name := tag.String()
		if name == "" || utf8.RuneCountInString(name) > 50 || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		tags = append(tags, name)
		if len(tags) == maxTags {
			break
		}
	}
	return tags
}

var spaces = regexp.MustCompile(`\s+`)

// Tags inside a line of text, which part no words.
var inline = map[string]bool{
	"a": true, "abbr": true, "b": true, "bdi": true, "bdo": true, "cite": true,
	"code": true, "em": true, "i": true, "kbd": true, "mark": true, "q": true,
	"s": true, "small": true, "span": true, "strong": true, "sub": true,
	"sup": true, "time": true, "u": true,
}

// plain is text with its HTML taken out and its spaces made single.
func plain(text string) string {
	var out strings.Builder
	tokens := html.NewTokenizer(strings.NewReader(text))
	skip := 0
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(spaces.ReplaceAllString(out.String(), " "))
		case html.StartTagToken:
			name, _ := tokens.TagName()
			n := string(name)
			if n == "script" || n == "style" {
				skip++
			}
			if !inline[n] {
				out.WriteByte(' ')
			}
		case html.EndTagToken:
			name, _ := tokens.TagName()
			n := string(name)
			if (n == "script" || n == "style") && skip > 0 {
				skip--
			}
			if !inline[n] {
				out.WriteByte(' ')
			}
		case html.TextToken:
			if skip == 0 {
				out.Write(tokens.Text())
			}
		}
	}
}

// clip cuts text to at most n characters, at a word, with an ellipsis when
// anything was cut.
func clip(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	runes := []rune(text)
	cut := string(runes[:n-1])
	if i := strings.LastIndexFunc(cut, unicode.IsSpace); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRightFunc(cut, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) }) + "…"
}

// A link in a feed: <atom:link> in RSS, <link> in Atom.
var feedLink = regexp.MustCompile(`(?i)<(?:atom:)?link\b[^>]*>`)
var linkAttr = regexp.MustCompile(`(?i)\b(rel|href)\s*=\s*("[^"]*"|'[^']*')`)

// hubOf is the WebSub hub a feed names (rel="hub") and the address it
// names itself by (rel="self"), the feed's own when it names none. Only an
// https or http hub is taken.
func hubOf(body []byte, feedURL string) (hub, self string) {
	for _, tag := range feedLink.FindAll(body, 50) {
		var rel, href string
		for _, attr := range linkAttr.FindAllSubmatch(tag, -1) {
			value := html.UnescapeString(strings.Trim(string(attr[2]), `"'`))
			if strings.EqualFold(string(attr[1]), "rel") {
				rel = strings.ToLower(value)
			} else {
				href = strings.TrimSpace(value)
			}
		}
		switch {
		case hasWord(rel, "hub") && hub == "":
			hub = absolute(href, feedURL)
		case hasWord(rel, "self") && self == "":
			self = absolute(href, feedURL)
		}
	}
	if hub == "" {
		return "", ""
	}
	if self == "" {
		self = feedURL
	}
	return hub, self
}
