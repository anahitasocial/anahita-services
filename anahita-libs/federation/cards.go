package federation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"anahita-libs/linkpreviews"
)

// The card for a link in a post from another server.
//
// A post written here that has a link in it is given a card: what the page
// is called, what it says of itself and its picture, read by this
// installation and kept in its own storage. A post from elsewhere is given
// one the same way and by the same service, link-preview-service, asked at
// an address the gateway does not send anybody to. Nothing the other
// server says of the link is used: Mastodon makes its own cards too, and
// what a card says of a page is what this installation found there.

// Cards finds the preview of a link, or nil when there is nothing to show.
type Cards interface {
	Preview(ctx context.Context, address string) *linkpreviews.Preview
}

// How long a card is waited for. A post is kept without one before it is
// kept late: its card is not what it is.
const cardWait = 6 * time.Second

// previewClient asks link-preview-service.
type previewClient struct {
	base   string
	client *http.Client
}

// NewCards is what asks link-preview-service at `base`
// ("http://link-preview-service"). Nil when there is no address: an
// installation without the service has no cards, here as everywhere.
func NewCards(base string) Cards {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil
	}
	return &previewClient{base: base, client: &http.Client{Timeout: cardWait}}
}

func (c *previewClient) Preview(ctx context.Context, address string) *linkpreviews.Preview {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/internal/link-previews?url="+url.QueryEscape(address), nil)
	if err != nil {
		return nil
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil
	}
	preview := &linkpreviews.Preview{}
	if json.Unmarshal(raw, preview) != nil || !preview.Usable() {
		return nil
	}
	return preview
}

var (
	// A link as Markdown has it: [what it says](where it leads).
	markdownLink = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^)\s]+)\)`)
)

// CardLink is the link in a post that its card is of: the first that is a
// link to a page. Not somebody's name, which leads to their profile, nor a
// hashtag left as a link.
func CardLink(markdown string) string {
	// With the links to people and hashtags taken out, the first address
	// left is the one.
	rest := markdownLink.ReplaceAllStringFunc(markdown, func(link string) string {
		parts := markdownLink.FindStringSubmatch(link)
		if strings.HasPrefix(parts[1], "@") || strings.HasPrefix(parts[1], "#") {
			return ""
		}
		return " " + parts[2] + " "
	})
	return linkpreviews.FirstURL(rest)
}

// CardMeta gives a post from elsewhere the card of the link in it, in the
// meta a note keeps one in, or nothing when it has no link or the page
// says nothing of itself. `address` is the link when the post names one
// apart from its words, as a Bluesky post does; empty for the first link
// in the words.
func CardMeta(cards Cards, body, address string) []byte {
	if cards == nil {
		return nil
	}
	if address == "" {
		address = CardLink(body)
	}
	if address == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cardWait)
	defer cancel()

	preview := cards.Preview(ctx, address)
	if preview == nil {
		return nil
	}
	return linkpreviews.IntoMeta(nil, linkpreviews.NewSnapshot(preview))
}
