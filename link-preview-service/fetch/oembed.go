package fetch

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// YouTube answers a server with a page that says nothing of the video: no
// picture, its title "- YouTube" and a line about YouTube itself. Tried in
// November 2026: the same address and the same headers read the whole page
// from a home connection, and that page from banu.io's server. What it says
// of a video to anybody who asks is at its oEmbed address, so that is asked
// first, and the page is read only when it has no answer.
const youTubeOEmbed = "https://www.youtube.com/oembed"

var youTubeHosts = map[string]bool{
	"youtube.com":       true,
	"www.youtube.com":   true,
	"m.youtube.com":     true,
	"music.youtube.com": true,
	"youtu.be":          true,
}

// oEmbed is the part of an oEmbed answer a card is made of.
type oEmbed struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	ProviderName string `json:"provider_name"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// youTube asks YouTube about a video. nil for an address that is not on
// YouTube, one it does not know, or anything else that goes wrong: the page
// is read instead.
func (f *Fetcher) youTube(address string) *Page {
	parsed, err := url.Parse(address)
	if err != nil || !youTubeHosts[strings.ToLower(parsed.Hostname())] {
		return nil
	}

	resp, err := f.get(youTubeOEmbed+"?format=json&url="+url.QueryEscape(address), "application/json")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	answer := &oEmbed{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxPageBytes)).Decode(answer); err != nil {
		return nil
	}
	if answer.Title == "" {
		return nil
	}

	return &Page{
		Title:       answer.Title,
		Description: answer.AuthorName,
		SiteName:    answer.ProviderName,
		Image:       answer.ThumbnailURL,
	}
}
