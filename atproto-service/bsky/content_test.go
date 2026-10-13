package bsky

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func post(t *testing.T, raw string) *Post {
	t.Helper()
	p := &Post{}
	if err := json.Unmarshal([]byte(raw), p); err != nil {
		t.Fatal(err)
	}
	return p
}

// THE RULE THIS GUARDS: nothing a post says is passed on as HTML of its
// author's making. Every word is escaped, and only what the list beside
// the words marks as a link to the web becomes one.
func TestAPostsWordsAsHTML(t *testing.T) {
	// "é" is two bytes: the stretches are counted in bytes, not letters.
	const text = "Café <b>news</b> at example.com/long… by @alice.test #beans\nsecond"
	stretch := func(words, feature string) string {
		start := strings.Index(text, words)
		return fmt.Sprintf(`{"index":{"byteStart":%d,"byteEnd":%d},"features":[%s]}`, start, start+len(words), feature)
	}
	raw, _ := json.Marshal(text)
	p := post(t, `{"text":`+string(raw)+`,"facets":[`+
		stretch("example.com/long…", `{"$type":"app.bsky.richtext.facet#link","uri":"https://example.com/long/address?a=1&b=2"}`)+`,`+
		stretch("@alice.test", `{"$type":"app.bsky.richtext.facet#mention","did":"did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"}`)+`,`+
		stretch("#beans", `{"$type":"app.bsky.richtext.facet#tag","tag":"beans"}`)+`]}`)

	want := `<p>Café &lt;b&gt;news&lt;/b&gt; at <a href="https://example.com/long/address?a=1&amp;b=2">example.com/long…</a> by ` +
		`<a href="https://bsky.app/profile/did:plc:aaaaaaaaaaaaaaaaaaaaaaaa">@alice.test</a> #beans<br>second</p>`
	if got := p.HTML(); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestStretchesThatCannotBeAreLeftAsWords(t *testing.T) {
	for name, facets := range map[string]string{
		"past the end":     `{"index":{"byteStart":2,"byteEnd":99},"features":[{"$type":"app.bsky.richtext.facet#link","uri":"https://example.com"}]}`,
		"backwards":        `{"index":{"byteStart":4,"byteEnd":2},"features":[{"$type":"app.bsky.richtext.facet#link","uri":"https://example.com"}]}`,
		"not to the web":   `{"index":{"byteStart":0,"byteEnd":4},"features":[{"$type":"app.bsky.richtext.facet#link","uri":"javascript:alert(1)"}]}`,
		"nobody's name":    `{"index":{"byteStart":0,"byteEnd":4},"features":[{"$type":"app.bsky.richtext.facet#mention","did":"\"><script>"}]}`,
		"a kind not known": `{"index":{"byteStart":0,"byteEnd":4},"features":[{"$type":"app.bsky.richtext.facet#other","uri":"https://example.com"}]}`,
	} {
		p := post(t, `{"text":"some words","facets":[`+facets+`]}`)
		if got := p.HTML(); got != "<p>some words</p>" {
			t.Errorf("%s: %s", name, got)
		}
	}

	// Two that overlap: the first is a link, the second is left.
	p := post(t, `{"text":"abcdefgh","facets":[
		{"index":{"byteStart":0,"byteEnd":4},"features":[{"$type":"app.bsky.richtext.facet#link","uri":"https://one.example"}]},
		{"index":{"byteStart":2,"byteEnd":6},"features":[{"$type":"app.bsky.richtext.facet#link","uri":"https://two.example"}]}]}`)
	if got := p.HTML(); got != `<p><a href="https://one.example">abcd</a>efgh</p>` {
		t.Errorf("overlapping: %s", got)
	}

	if got := post(t, `{"text":"  "}`).HTML(); got != "" {
		t.Errorf("no words: %q", got)
	}
}

func TestWhatAPostCarries(t *testing.T) {
	const cid = "bafkreihwihm6kpd6zuwhhlro75p5qks5qtrcu55jp3gddbfjsieiv7wuka"

	pictures := post(t, `{"text":"x","embed":{"$type":"app.bsky.embed.images","images":[
		{"alt":"A row of beans","image":{"$type":"blob","ref":{"$link":"`+cid+`"},"mimeType":"image/jpeg","size":1}},
		{"alt":"not one","image":{"ref":{"$link":"../../etc"},"mimeType":"image/jpeg"}},
		{"alt":"a film","image":{"ref":{"$link":"`+cid+`"},"mimeType":"video/mp4"}}]}}`)
	got := pictures.Pictures()
	if len(got) != 1 || got[0].CID != cid || got[0].Alt != "A row of beans" {
		t.Errorf("pictures %+v", got)
	}
	if at := PictureURL("https://cdn.bsky.app", "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa", cid); at != "https://cdn.bsky.app/img/feed_fullsize/plain/did:plc:aaaaaaaaaaaaaaaaaaaaaaaa/"+cid+"@jpeg" {
		t.Errorf("fetched from %s", at)
	}

	card := post(t, `{"text":"x","embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.com/story","title":"T","description":"D"}}}`)
	if card.Link() != "https://example.com/story" || card.Quoted() != "" || len(card.Pictures()) != 0 {
		t.Errorf("card %q, quoted %q", card.Link(), card.Quoted())
	}
	if plain := post(t, `{"text":"x","embed":{"$type":"app.bsky.embed.external","external":{"uri":"http://example.com/"}}}`); plain.Link() != "" {
		t.Errorf("a card over plain http was taken: %s", plain.Link())
	}

	const quotedURI = "at://did:plc:bbbbbbbbbbbbbbbbbbbbbbbb/app.bsky.feed.post/3l3qo2vutsw2b"
	const quotedPage = "https://bsky.app/profile/did:plc:bbbbbbbbbbbbbbbbbbbbbbbb/post/3l3qo2vutsw2b"
	quote := post(t, `{"text":"x","embed":{"$type":"app.bsky.embed.record","record":{"uri":"`+quotedURI+`","cid":"c"}}}`)
	if quote.Quoted() != quotedPage {
		t.Errorf("quoted %q", quote.Quoted())
	}
	both := post(t, `{"text":"x","embed":{"$type":"app.bsky.embed.recordWithMedia","record":{"record":{"uri":"`+quotedURI+`"}},
		"media":{"$type":"app.bsky.embed.images","images":[{"alt":"","image":{"ref":{"$link":"`+cid+`"},"mimeType":"image/png"}}]}}}`)
	if both.Quoted() != quotedPage || len(both.Pictures()) != 1 {
		t.Errorf("quoted %q with %d pictures", both.Quoted(), len(both.Pictures()))
	}
	// A list or a feed can be quoted too; only a post has a page that is
	// led to.
	if other := post(t, `{"text":"x","embed":{"$type":"app.bsky.embed.record","record":{"uri":"at://did:plc:bbbbbbbbbbbbbbbbbbbbbbbb/app.bsky.graph.list/3l3"}}}`); other.Quoted() != "" {
		t.Errorf("a list was quoted as a post: %s", other.Quoted())
	}
}

func TestWhatAPostSaysOfItself(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	p := post(t, `{"text":"x","createdAt":"2026-10-11T08:30:00.123Z","langs":["fr","en"],"reply":{"root":{"uri":"at://x"}},
		"labels":{"$type":"com.atproto.label.defs#selfLabels","values":[{"val":"nudity"}]}}`)
	if !p.IsReply() || !p.IsSensitive() || p.Language() != "fr" {
		t.Errorf("reply %v, sensitive %v, language %q", p.IsReply(), p.IsSensitive(), p.Language())
	}
	if got := p.Created(now); !got.Equal(time.Date(2026, 10, 11, 8, 30, 0, 123000000, time.UTC)) {
		t.Errorf("created %v", got)
	}

	plain := post(t, `{"text":"x","createdAt":"2031-01-01T00:00:00Z","reply":null}`)
	if plain.IsReply() || plain.IsSensitive() || plain.Language() != "" {
		t.Error("a plain post was read as more")
	}
	// One that says it is from the future, or says nothing readable, is
	// from now.
	if !plain.Created(now).Equal(now) || !post(t, `{"createdAt":"soon"}`).Created(now).Equal(now) {
		t.Error("a time that cannot be was kept")
	}
}

func TestWhatIsTypedToFindSomebody(t *testing.T) {
	const did = "did:plc:z72i7hdynmk6r22z27h6tvur"
	for typed, want := range map[string]string{
		"alice.bsky.social":    "alice.bsky.social",
		" @Alice.Bsky.Social ": "alice.bsky.social",
		did:                    did,
		"https://bsky.app/profile/alice.bsky.social":              "alice.bsky.social",
		"https://bsky.app/profile/" + did + "/post/3l3qo2vutsw2b": did,
		"at://" + did + "/app.bsky.feed.post/3l3":                 did,
		"did:web:example.com":                                     "did:web:example.com",
		"alice":                                                   "",
		"alice@mastodon.example":                                  "",
		"https://example.com/alice.bsky.social":                   "",
		"did:plc:short":                                           "",
		"alice.bsky.social/../x":                                  "",
		"":                                                        "",
		strings.Repeat("a", 250) + ".example":                     "",
	} {
		if got := ParseActor(typed); got != want {
			t.Errorf("%q: %q, want %q", typed, got, want)
		}
	}

	if did, rkey, ok := ParsePostURI("at://" + did + "/app.bsky.feed.post/3l3qo2vutsw2b"); !ok || rkey != "3l3qo2vutsw2b" || did == "" {
		t.Error("a post's address was not read")
	}
	for _, bad := range []string{"https://bsky.app/x", "at://" + did, "at://" + did + "/app.bsky.feed.like/3l3", "at://nobody/app.bsky.feed.post/3l3", "at://" + did + "/app.bsky.feed.post/a/b"} {
		if _, _, ok := ParsePostURI(bad); ok {
			t.Errorf("%s was read as a post's address", bad)
		}
	}
}
