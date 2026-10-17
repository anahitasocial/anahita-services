package feeds

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

// site answers each address from what it holds, with any headers given.
type site map[string]page

type page struct {
	status int
	body   string
	header http.Header
}

func (s site) RoundTrip(req *http.Request) (*http.Response, error) {
	p, ok := s[req.URL.String()]
	if !ok {
		p = page{status: http.StatusNotFound}
	}
	if p.status == 0 {
		p.status = http.StatusOK
	}
	header := http.Header{}
	for key, values := range p.header {
		for _, value := range values {
			header.Add(key, value)
		}
	}
	p.header = header
	if etag := p.header.Get("ETag"); etag != "" && req.Header.Get("If-None-Match") == etag {
		p = page{status: http.StatusNotModified, header: p.header}
	}
	return &http.Response{StatusCode: p.status, Body: io.NopCloser(strings.NewReader(p.body)), Header: p.header, Request: req}, nil
}

func readerOf(s site) *Reader {
	return &Reader{Client: &http.Client{Transport: s}, parser: gofeed.NewParser()}
}

const rss = `<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>Garden notes</title><link>https://example.com/</link><description>What grows</description>
  <item>
    <title>Beans &amp; peas</title>
    <link>/2026/beans</link>
    <guid>urn:example:1</guid>
    <description><![CDATA[<p>Sown in <b>March</b>, up in April.</p><script>x()</script>]]></description>
    <category>Open Science</category><category>open-science</category><category>Beans</category>
    <pubDate>Mon, 12 Oct 2026 09:00:00 GMT</pubDate>
  </item>
  <item><title>No address</title></item>
</channel></rss>`

func TestAPageNamesItsFeed(t *testing.T) {
	r := readerOf(site{
		"https://example.com/": {body: `<html><head><title>x</title>
			<link rel="stylesheet" href="/s.css">
			<link rel="alternate" type="application/rss+xml" href="/feed.xml">
			</head><body></body></html>`, header: http.Header{"Content-Type": {"text/html"}}},
		"https://example.com/feed.xml": {body: rss},
	})

	found, err := r.Find(context.Background(), "https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if found.FeedURL != "https://example.com/feed.xml" || found.Title != "Garden notes" || found.SiteURL != "https://example.com/" {
		t.Errorf("found %+v", found)
	}
}

func TestAFeedIsTakenAsItIs(t *testing.T) {
	r := readerOf(site{"https://example.com/feed.xml": {body: rss}})
	found, err := r.Find(context.Background(), "https://example.com/feed.xml")
	if err != nil || found.FeedURL != "https://example.com/feed.xml" {
		t.Fatalf("%+v, %v", found, err)
	}
}

func TestAPageWithNoFeedIsSaidSo(t *testing.T) {
	r := readerOf(site{"https://example.com/": {body: `<html><head></head><body><link rel="alternate" type="application/rss+xml" href="/late.xml"></body></html>`}})
	if _, err := r.Find(context.Background(), "https://example.com/"); !errors.Is(err, ErrNoFeed) {
		t.Errorf("err %v, want ErrNoFeed (a link in the body is not the page's feed)", err)
	}
	if _, err := r.Find(context.Background(), "https://example.com/missing"); !errors.Is(err, ErrUnreadable) {
		t.Errorf("err %v, want ErrUnreadable", err)
	}
}

func TestItemsAreWhatAPostIsMadeOf(t *testing.T) {
	r := readerOf(site{"https://example.com/feed.xml": {body: rss, header: http.Header{"ETag": {`"v1"`}}}})
	read, err := r.Read(context.Background(), "https://example.com/feed.xml", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if read.ETag != `"v1"` || len(read.Items) != 1 {
		t.Fatalf("read %+v: an item with no address is left out", read)
	}

	item := read.Items[0]
	if item.Title != "Beans & peas" || item.Excerpt != "Sown in March, up in April." || item.Link != "https://example.com/2026/beans" {
		t.Errorf("item %+v", item)
	}
	if item.Key != KeyOf("urn:example:1") {
		t.Errorf("an item is kept by its id")
	}
	if strings.Join(item.Tags, " ") != "OpenScience Beans" {
		t.Errorf("tags %v: one word each, each once whatever its case", item.Tags)
	}
	if !item.Published.Equal(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("published %v", item.Published)
	}

	again, err := r.Read(context.Background(), "https://example.com/feed.xml", `"v1"`, "")
	if err != nil || !again.NotModified || again.ETag != `"v1"` {
		t.Errorf("an unchanged feed: %+v, %v", again, err)
	}
}

func TestTextIsCutAtAWord(t *testing.T) {
	long := strings.Repeat("word ", 100)
	cut := clip(long, ExcerptLength)
	if len([]rune(cut)) > ExcerptLength || !strings.HasSuffix(cut, "word…") {
		t.Errorf("cut to %q", cut[len(cut)-20:])
	}
	if clip("short", 10) != "short" {
		t.Error("short text was cut")
	}
}

func TestAFeedNamesItsHub(t *testing.T) {
	body := []byte(`<rss xmlns:atom="http://www.w3.org/2005/Atom"><channel>
		<atom:link rel="self" type="application/rss+xml" href="https://example.com/feed/"/>
		<atom:link href='https://pubsubhubbub.appspot.com/' rel='hub'/>
	</channel></rss>`)
	if hub, self := hubOf(body, "https://example.com/feed"); hub != "https://pubsubhubbub.appspot.com/" || self != "https://example.com/feed/" {
		t.Errorf("hub %q, self %q", hub, self)
	}
	if hub, self := hubOf([]byte(rss), "https://example.com/feed.xml"); hub != "" || self != "" {
		t.Errorf("a feed with no hub: %q %q", hub, self)
	}
}
