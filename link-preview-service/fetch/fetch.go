package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	images "anahita-libs/utils"

	"golang.org/x/net/html/charset"
)

// Fetcher reads pages and their pictures.
type Fetcher struct {
	client    *http.Client
	userAgent string
}

// UserAgent is what the fetcher says it is: the software's name, and
// nothing else.
//
// It said more once, "AnahitaBot/1.0 (+https://the.site)", so that whoever
// runs a page could see which installation was reading it. Large sites'
// edges drop that: tried against one news site in October 2026, a request
// naming almost any address in the User-Agent, or calling itself a bot, had
// its connection reset or left hanging, where the bare name was answered.
// A preview that works matters more than the courtesy.
const UserAgent = "Anahita/1.0"

// New builds a fetcher that goes only to public addresses.
func New() *Fetcher {
	return newFetcher(PublicAddress)
}

func newFetcher(allow Allow) *Fetcher {
	return &Fetcher{client: newClient(allow), userAgent: UserAgent}
}

func (f *Fetcher) get(address, accept string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAllowed, err)
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, ErrNotAllowed
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", accept)

	return f.client.Do(req)
}

// Page fetches a page and reads its head. It returns where the address led
// as well, which a short link does not say.
func (f *Fetcher) Page(address string) (*Page, string, error) {
	resp, err := f.get(address, "text/html,application/xhtml+xml")
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%w: status %d", ErrNotAPage, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	kind, _, _ := mime.ParseMediaType(contentType)
	if kind != "text/html" && kind != "application/xhtml+xml" {
		return nil, "", fmt.Errorf("%w: %s", ErrNotAPage, kind)
	}

	// No more than the top of it, whatever it says of its own length, and
	// read as the characters it says it is written in.
	limited := io.LimitReader(resp.Body, MaxPageBytes)
	body, err := charset.NewReader(limited, contentType)
	if err != nil {
		body = limited
	}

	final := resp.Request.URL

	return parse(body, final), final.String(), nil
}

// Thumbnail fetches a picture and returns it made small and written out
// again, with the name to keep it under and its type.
//
// Nothing of the file that came is kept as it came: the picture is opened
// and a new file is made of it, so whatever else the file carried is left
// behind. A file that is not a picture, or is too big to be one worth
// opening, is refused.
func (f *Fetcher) Thumbnail(address string) ([]byte, string, string, error) {
	resp, err := f.get(address, "image/*")
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("%w: status %d", ErrNotAPage, resp.StatusCode)
	}

	// One byte past the most, to tell a picture of exactly that size from
	// one that is over.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return nil, "", "", err
	}

	upright, _, err := images.Accept(raw, MaxImageBytes, MaxImagePixels)
	if err != nil {
		return nil, "", "", err
	}

	small, err := images.FitImage(upright, ThumbnailWidth)
	if err != nil {
		return nil, "", "", err
	}

	mimeType := http.DetectContentType(small)
	extension := extensionOf(mimeType)
	if extension == "" {
		return nil, "", "", images.ErrUnsupported
	}

	// Named for what is in it, so the same picture linked twice is kept
	// once.
	sum := sha256.Sum256(small)

	return small, hex.EncodeToString(sum[:]) + extension, mimeType, nil
}

func extensionOf(mimeType string) string {
	switch strings.ToLower(mimeType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ""
}
