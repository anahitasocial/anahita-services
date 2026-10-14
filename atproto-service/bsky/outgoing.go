package bsky

import (
	"regexp"
	"strings"
	"time"
)

// What is written here, made into what Bluesky takes.
//
// A post here is Markdown. A Bluesky post is plain words, with a list
// beside them saying which stretch is a link and which a hashtag, and may
// be three hundred letters long. So the Markdown's marks are taken out,
// each link is left as its words with the address beside them, and what
// is too long is cut, with a mark to say so.

// PostMax is how long a post may be, in letters.
const PostMax = 300

// OutPost is a post as it is written to Bluesky.
type OutPost struct {
	Type      string     `json:"$type"`
	Text      string     `json:"text"`
	Facets    []OutFacet `json:"facets,omitempty"`
	CreatedAt string     `json:"createdAt"`
	Langs     []string   `json:"langs,omitempty"`
	Reply     *OutReply  `json:"reply,omitempty"`
	Embed     *OutEmbed  `json:"embed,omitempty"`
}

// OutEmbed is what a post carries besides its words: the card of a link,
// its pictures, or its video.
type OutEmbed struct {
	Type     string       `json:"$type"`
	External *OutExternal `json:"external,omitempty"`
	Images   []OutImage   `json:"images,omitempty"`
	// A video, and its shape.
	Video       *OutBlob  `json:"video,omitempty"`
	AspectRatio *OutRatio `json:"aspectRatio,omitempty"`
}

// OutRatio is the shape of a picture or a video, which lets it be laid out
// before it has loaded.
type OutRatio struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// OutExternal is the card of a link.
type OutExternal struct {
	URI         string   `json:"uri"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Thumb       *OutBlob `json:"thumb,omitempty"`
}

// OutImage is one picture of a post.
type OutImage struct {
	Alt         string   `json:"alt"`
	Image       *OutBlob `json:"image"`
	AspectRatio *struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"aspectRatio,omitempty"`
}

// OutBlob names a file that was uploaded to the account's server, as that
// server named it.
type OutBlob struct {
	Type string `json:"$type"`
	Ref  struct {
		Link string `json:"$link"`
	} `json:"ref"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

// GetImages is the pictures a post carries so far.
func (e *OutEmbed) GetImages() []OutImage {
	if e == nil {
		return nil
	}
	return e.Images
}

// BlobMax is the most a picture may weigh.
const BlobMax = 1000000

// ImagesMax is the most pictures a post may carry.
const ImagesMax = 4

// VideoMax and VideoSecondsMax are the heaviest and longest video sent to
// Bluesky. Bluesky takes more weight, up to 100 MB; what is sent is read
// whole and uploaded in one go, and a video heavier than this goes as a
// card that leads to it instead.
const (
	VideoMax        = 50 * 1024 * 1024
	VideoSecondsMax = 180
)

// WithVideo gives a post a video, uploaded, with its shape. A post carries
// a video or pictures or a card, one of them.
func (p *OutPost) WithVideo(blob *OutBlob, width, height int) {
	if blob == nil || p.Embed != nil {
		return
	}
	p.Embed = &OutEmbed{Type: "app.bsky.embed.video", Video: blob}
	if width > 0 && height > 0 {
		p.Embed.AspectRatio = &OutRatio{Width: width, Height: height}
	}
}

// WithCard gives a post the card of a link: Bluesky draws none of its
// own from an address in a post's words. `thumb` is its picture, uploaded,
// or nil.
func (p *OutPost) WithCard(address, title, description string, thumb *OutBlob) {
	if !strings.HasPrefix(address, "https://") || title == "" || p.Embed != nil {
		return
	}
	p.Embed = &OutEmbed{Type: "app.bsky.embed.external", External: &OutExternal{
		URI:         address,
		Title:       string([]rune(title)[:min(len([]rune(title)), 300)]),
		Description: string([]rune(description)[:min(len([]rune(description)), 1000)]),
		Thumb:       thumb,
	}}
}

// AddImage gives a post a picture, uploaded, with what it shows and its
// shape, which lets it be laid out before it has loaded.
func (p *OutPost) AddImage(blob *OutBlob, alt string, width, height int) {
	if blob == nil {
		return
	}
	if p.Embed == nil || p.Embed.Type != "app.bsky.embed.images" {
		p.Embed = &OutEmbed{Type: "app.bsky.embed.images"}
	}
	if len(p.Embed.Images) >= ImagesMax {
		return
	}
	image := OutImage{Alt: string([]rune(alt)[:min(len([]rune(alt)), 2000)]), Image: blob}
	if width > 0 && height > 0 {
		image.AspectRatio = &struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		}{width, height}
	}
	p.Embed.Images = append(p.Embed.Images, image)
}

// OutReply says what a post answers, and what began the thread.
type OutReply struct {
	Root   Ref `json:"root"`
	Parent Ref `json:"parent"`
}

// OutFacet marks one stretch of a post's words.
type OutFacet struct {
	Index struct {
		ByteStart int `json:"byteStart"`
		ByteEnd   int `json:"byteEnd"`
	} `json:"index"`
	Features []map[string]string `json:"features"`
}

// OutSubject is a like or a repost as it is written: of which post.
type OutSubject struct {
	Type      string `json:"$type"`
	Subject   Ref    `json:"subject"`
	CreatedAt string `json:"createdAt"`
}

// Stamp is a time as Bluesky writes one.
func Stamp(at time.Time) string {
	return at.UTC().Format("2006-01-02T15:04:05.000Z")
}

var (
	// A link as Markdown has it, an address standing alone, or a hashtag.
	outToken = regexp.MustCompile(`\[([^\]\n]*)\]\((https?://[^)\s]+)\)|(https?://[^\s<>)\]]+)|(^|[\s(])#([\p{L}\p{N}_]{1,64})`)
	// Marks that mean something in Markdown and nothing in plain words.
	outEmphasis = regexp.MustCompile(`\*\*|__|~~|` + "`")
	outEscape   = regexp.MustCompile(`\\([\\_*#\[\]()~>.!+-])`)
	outHeading  = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	outQuote    = regexp.MustCompile(`(?m)^>\s?`)
)

// plain is words with Markdown's marks taken out.
func plain(markdown string) string {
	markdown = outHeading.ReplaceAllString(markdown, "")
	markdown = outQuote.ReplaceAllString(markdown, "")
	markdown = outEmphasis.ReplaceAllString(markdown, "")
	return outEscape.ReplaceAllString(markdown, "$1")
}

// NewOutPost makes a post of what was written here.
func NewOutPost(markdown, language string, at time.Time) *OutPost {
	markdown = strings.TrimSpace(strings.ReplaceAll(markdown, "\r\n", "\n"))

	var (
		text   strings.Builder
		facets []OutFacet
	)
	mark := func(words string, feature map[string]string) {
		facet := OutFacet{Features: []map[string]string{feature}}
		facet.Index.ByteStart = text.Len()
		text.WriteString(words)
		facet.Index.ByteEnd = text.Len()
		facets = append(facets, facet)
	}

	at0 := 0
	for _, m := range outToken.FindAllStringSubmatchIndex(markdown, -1) {
		text.WriteString(plain(markdown[at0:m[0]]))
		at0 = m[1]

		switch {
		case m[2] >= 0: // [words](address)
			words := plain(markdown[m[2]:m[3]])
			if strings.TrimSpace(words) == "" {
				words = markdown[m[4]:m[5]]
			}
			mark(words, map[string]string{"$type": "app.bsky.richtext.facet#link", "uri": markdown[m[4]:m[5]]})
		case m[6] >= 0: // an address alone
			address := strings.TrimRight(markdown[m[6]:m[7]], ".,;:!?")
			at0 = m[6] + len(address)
			mark(address, map[string]string{"$type": "app.bsky.richtext.facet#link", "uri": address})
		default: // #hashtag, after what came before it
			text.WriteString(markdown[m[8]:m[9]])
			tag := markdown[m[10]:m[11]]
			mark("#"+tag, map[string]string{"$type": "app.bsky.richtext.facet#tag", "tag": tag})
		}
	}
	text.WriteString(plain(markdown[at0:]))

	post := &OutPost{Type: PostCollection, Text: text.String(), CreatedAt: Stamp(at)}
	if language != "" && language != "und" && len(language) <= 35 {
		post.Langs = []string{language}
	}

	// Cut to what a post may be, at a letter and not inside one. A stretch
	// that the cut runs through is left as plain words.
	if letters := []rune(post.Text); len(letters) > PostMax {
		post.Text = strings.TrimRight(string(letters[:PostMax-1]), " \n") + "…"
	}
	limit := len(post.Text)
	for _, facet := range facets {
		if facet.Index.ByteEnd <= limit-len("…") || (facet.Index.ByteEnd <= limit && limit == text.Len()) {
			post.Facets = append(post.Facets, facet)
		}
	}
	return post
}
