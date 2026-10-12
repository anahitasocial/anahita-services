package federation

import (
	"fmt"
	"strings"
	"testing"
)

var links = Links{
	Mention: func(user, host string) string {
		switch {
		case host == "" && user == "ana":
			return "https://api.example.org/ap/users/1111"
		case host == "mastodon.social" && user == "alice":
			return "https://mastodon.social/users/alice"
		}
		return ""
	},
	Hashtag: func(tag string) string {
		return "https://example.org/hashtags/" + tag
	},
}

func TestToHTML(t *testing.T) {
	cases := map[string]struct{ source, want string }{
		"a paragraph": {
			"Hello *there*, **world**.",
			"<p>Hello <em>there</em>, <strong>world</strong>.</p>",
		},
		"two paragraphs and a line break": {
			"One\ntwo\n\nThree",
			"<p>One\ntwo</p>\n<p>Three</p>",
		},
		"a link": {
			"See [the site](https://example.com/a?b=1&c=2).",
			`<p>See <a href="https://example.com/a?b=1&amp;c=2">the site</a>.</p>`,
		},
		"a bare address": {
			"https://example.com/page",
			`<p><a href="https://example.com/page">https://example.com/page</a></p>`,
		},
		"somebody here": {
			"Thanks @ana!",
			`<p>Thanks <span class="h-card"><a href="https://api.example.org/ap/users/1111" class="u-url mention">@<span>ana</span></a></span>!</p>`,
		},
		"somebody elsewhere": {
			"cc @alice@mastodon.social.",
			`<p>cc <span class="h-card"><a href="https://mastodon.social/users/alice" class="u-url mention">@<span>alice</span></a></span>.</p>`,
		},
		"somebody nobody knows is left as written": {
			"Hello @nobody and @ana@elsewhere.example",
			"<p>Hello @nobody and @ana@elsewhere.example</p>",
		},
		"a hashtag": {
			"Sowing #Seeds today",
			`<p>Sowing <a href="https://example.org/hashtags/seeds" class="mention hashtag" rel="tag">#<span>Seeds</span></a> today</p>`,
		},
		"a name in code is code": {
			"Run `@ana #seeds` and\n\n    @ana #seeds",
			"<p>Run <code>@ana #seeds</code> and</p>\n<pre><code>@ana #seeds\n</code></pre>",
		},
		"a name in a link's text is the link's": {
			"[ask @ana](https://example.com)",
			`<p><a href="https://example.com">ask @ana</a></p>`,
		},
		"a list and a quotation": {
			"> quoted\n\n- one\n- two",
			"<blockquote>\n<p>quoted</p>\n</blockquote>\n<ul>\n<li>one</li>\n<li>two</li>\n</ul>",
		},
		"struck through": {
			"~~gone~~",
			"<p><del>gone</del></p>",
		},
	}

	for name, c := range cases {
		if got := ToHTML(c.source, links).HTML; got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", name, got, c.want)
		}
	}
}

// What is typed into a post is never passed on as HTML.
func TestToHTMLPassesNothingOn(t *testing.T) {
	for name, source := range map[string]string{
		"a script":           `Hi <script>alert(1)</script> there`,
		"a script on a link": `[x](javascript:alert(1))`,
		"a data link":        `[x](data:text/html;base64,PHNjcmlwdD4=)`,
		"an event":           `<p onclick="alert(1)">Hi</p>`,
		"a picture":          `![a cat](https://tracker.example/p.gif)`,
		"a typed picture":    `<img src="https://tracker.example/p.gif" onerror="alert(1)">`,
		"a frame":            `<iframe src="https://evil.example"></iframe>`,
		"a style":            `<p style="position:fixed">Hi</p>`,
		"a class of its own": `<span class="invisible admin">x</span>`,
	} {
		got := ToHTML(source, links).HTML
		for _, bad := range []string{"<script", "javascript:", "data:", "onclick", "onerror", "<img", "<iframe", "style=", "admin", "tracker.example"} {
			if strings.Contains(got, bad) {
				t.Errorf("%s: %q got through: %s", name, bad, got)
			}
		}
	}
}

func TestToHTMLListsWhoAndWhatIsNamed(t *testing.T) {
	got := ToHTML("@ana and @Ana again, @alice@mastodon.social, @nobody, #Seeds and #seeds, `#code`", links)

	if want := "[{@ana https://api.example.org/ap/users/1111} {@alice@mastodon.social https://mastodon.social/users/alice}]"; fmt.Sprint(got.Mentions) != want {
		t.Errorf("mentions %v", got.Mentions)
	}
	if want := "[{#seeds https://example.org/hashtags/seeds}]"; fmt.Sprint(got.Hashtags) != want {
		t.Errorf("hashtags %v", got.Hashtags)
	}

	// Without anybody to ask, names are text and nothing is listed.
	plain := ToHTML("@ana #seeds", Links{})
	if plain.HTML != "<p>@ana #seeds</p>" || len(plain.Mentions)+len(plain.Hashtags) != 0 {
		t.Errorf("got %+v", plain)
	}
}

func TestToMarkdown(t *testing.T) {
	cases := map[string]struct{ source, want string }{
		"paragraphs": {
			"<p>One</p><p>Two<br>three</p>",
			"One\n\nTwo  \nthree",
		},
		"emphasis and a link": {
			`<p>Hello <strong>there</strong>, <em>you</em>: <a href="https://example.com/a">a page</a></p>`,
			"Hello **there**, *you*: [a page](https://example.com/a)",
		},
		"somebody named, as Mastodon sends them": {
			`<p><span class="h-card" translate="no"><a href="https://mastodon.social/@alice" class="u-url mention">@<span>alice</span></a></span> hello</p>`,
			"[@alice](https://mastodon.social/@alice) hello",
		},
		"a hashtag, as Mastodon sends one": {
			`<p>Sowing <a href="https://mastodon.social/tags/seeds" class="mention hashtag" rel="tag">#<span>seeds</span></a></p>`,
			"Sowing [#seeds](https://mastodon.social/tags/seeds)",
		},
		"a long link, as Mastodon sends one": {
			`<p><a href="https://example.com/a/long/address" target="_blank" rel="nofollow noopener"><span class="invisible">https://</span><span class="ellipsis">example.com/a/lo</span><span class="invisible">ng/address</span></a></p>`,
			"[https://example.com/a/long/address](https://example.com/a/long/address)",
		},
		"a list": {
			"<ul><li>one</li><li>two</li></ul>",
			"- one\n- two",
		},
		"nothing but text": {
			"just words",
			"just words",
		},
		"nothing": {
			"",
			"",
		},
	}

	for name, c := range cases {
		if got := ToMarkdown(c.source); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", name, got, c.want)
		}
	}
}

// In a post, a hashtag is this installation's as well.
func TestPostToMarkdownKeepsHashtagsAsHashtags(t *testing.T) {
	cases := map[string]struct{ source, want string }{
		"a hashtag, as Mastodon sends one": {
			`<p>Sowing <a href="https://mastodon.social/tags/seeds" class="mention hashtag" rel="tag">#<span>seeds</span></a></p>`,
			"Sowing #seeds",
		},
		"hashtags alone": {
			`<p><a href="https://gts.test/tags/gardening" class="mention hashtag" rel="tag">#<span>gardening</span></a> <a href="https://gts.test/tags/soil_2026" rel="tag">#soil_2026</a></p>`,
			"#gardening #soil_2026",
		},
		// A hashtag here is Latin letters, digits and some emoji
		// (entities.HashtagRegexPattern). One that would not be a hashtag
		// here is left the link it came as.
		"a hashtag that is not one here stays a link": {
			`<p><a href="https://gts.test/tags/باغ" rel="tag">#باغ</a></p>`,
			"[#باغ](https://gts.test/tags/%D8%A8%D8%A7%D8%BA)",
		},
		"a link that only reads like a hashtag stays a link": {
			`<p>See <a href="https://example.com/issues/1">#1</a></p>`,
			"See [#1](https://example.com/issues/1)",
		},
		"a link marked as a hashtag that says something else stays a link": {
			`<p><a href="https://example.com/win" class="hashtag" rel="tag">click here</a></p>`,
			"[click here](https://example.com/win)",
		},
	}

	for name, c := range cases {
		if got := PostToMarkdown(c.source); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", name, got, c.want)
		}
	}
}

// What arrives is anybody's. None of this may be kept.
func TestToMarkdownKeepsNothingThatRuns(t *testing.T) {
	for name, source := range map[string]string{
		"a script":        `<p>Hi</p><script>alert(1)</script>`,
		"a script's link": `<a href="javascript:alert(1)">x</a>`,
		"a data link":     `<a href="data:text/html,<script>alert(1)</script>">x</a>`,
		"an event":        `<p onmouseover="alert(1)">Hi</p>`,
		"a picture":       `<p>Hi <img src="https://tracker.example/p.gif"></p>`,
		"a frame":         `<iframe src="https://evil.example"></iframe>`,
		"a form":          `<form action="https://evil.example"><input name="password"></form>`,
		"a style":         `<style>body{display:none}</style><p>Hi</p>`,
		"an object":       `<object data="https://evil.example/x.swf"></object>`,
		"unclosed":        `<p>Hi <script>alert(1)`,
	} {
		got := ToMarkdown(source)
		for _, bad := range []string{"<script", "javascript:", "data:", "alert", "onmouseover", "<img", "![", "<iframe", "<form", "<input", "<style", "display:none", "<object", "tracker.example", "evil.example"} {
			if strings.Contains(got, bad) {
				t.Errorf("%s: %q was kept: %s", name, bad, got)
			}
		}
	}
}

// A post that goes out and comes back reads as it did.
func TestThereAndBack(t *testing.T) {
	for _, source := range []string{
		"Hello **there**, *you*.",
		"One\n\nTwo",
		"See [the site](https://example.com/a).",
		"- one\n- two",
		"> quoted",
	} {
		if got := ToMarkdown(ToHTML(source, Links{}).HTML); got != source {
			t.Errorf("%q came back as %q", source, got)
		}
	}
}
