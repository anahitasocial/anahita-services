// Package linkpreviews is what the services share about previews of links:
// what a preview is, where one is kept while it is fresh, and the copy of
// one that a note keeps for good.
//
// link-preview-service fetches a page and writes its preview here.
// text-service reads it from here when a note is saved, and never from what
// the browser sent: a preview's words are the page's, as this installation
// read them, and not whatever somebody typed into a request.
package linkpreviews

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// How long a preview is kept before the page is read again, and how long a
// failure is remembered before it is tried again.
const (
	FreshFor      = 7 * 24 * time.Hour
	FailedFor     = time.Hour
	MaxURLLength  = 2000
	MaxTitle      = 200
	MaxText       = 300
	MaxSiteName   = 100
	StatusOK      = "ok"
	StatusFailed  = "failed"
	StatusBlocked = "blocked"
	// Where the service serves a preview's picture from, by its key.
	ImagePath = "/link-previews/image/"
)

// Preview is what was read off a page.
type Preview struct {
	// The address as it was asked for, and where it led.
	URL         string `json:"url"`
	FinalURL    string `json:"final_url,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	// The picture, as a key in this installation's own storage. Never the
	// page's own address for it: nobody reading a note is sent to another
	// site for a picture.
	ImageKey string `json:"image_key,omitempty"`
	Status   string `json:"status"`
}

// Usable reports whether there is something to show: the page was read and
// said at least what it is called.
func (p *Preview) Usable() bool {
	return p != nil && p.Status == StatusOK && p.Title != ""
}

// Cache keeps things a while. anahita-libs/sessions has the Redis one.
type Cache interface {
	Get(key string) (string, bool, error)
	Set(key string, value string, ttl time.Duration) error
}

// Store keeps previews while they are fresh.
type Store struct {
	cache Cache
}

func NewStore(cache Cache) *Store {
	return &Store{cache: cache}
}

// Key is where the preview of an address is kept.
func Key(address string) string {
	sum := sha256.Sum256([]byte(address))
	return "linkpreview:" + hex.EncodeToString(sum[:])
}

// Get returns the preview kept for an address, if one is.
func (s *Store) Get(address string) (*Preview, bool) {
	if s == nil || s.cache == nil {
		return nil, false
	}

	raw, found, err := s.cache.Get(Key(address))
	if err != nil || !found {
		return nil, false
	}

	preview := &Preview{}
	if err := json.Unmarshal([]byte(raw), preview); err != nil {
		return nil, false
	}

	return preview, true
}

// Put keeps a preview: a good one for a week, a failure for an hour.
func (s *Store) Put(preview *Preview) error {
	if s == nil || s.cache == nil || preview == nil {
		return nil
	}

	raw, err := json.Marshal(preview)
	if err != nil {
		return err
	}

	ttl := FailedFor
	if preview.Status == StatusOK {
		ttl = FreshFor
	}

	return s.cache.Set(Key(preview.URL), string(raw), ttl)
}

// Normalise tidies an address as somebody typed or pasted it, and says
// whether it is one a preview may be asked for: http or https, with a host,
// and not too long. It decides nothing about where the host is; that is the
// fetcher's, at the moment it connects.
func Normalise(address string) (string, bool) {
	address = strings.TrimSpace(address)
	if address == "" || len(address) > MaxURLLength {
		return "", false
	}

	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}

	parsed.Fragment = ""
	parsed.Host = strings.ToLower(parsed.Host)

	return parsed.String(), true
}

var firstURL = regexp.MustCompile(`https?://[^\s<>"')\]]+`)

// FirstURL finds the first link in a text, tidied, or "" when it has none.
// What follows a link in a sentence, a full stop or a comma, is not part of
// it.
func FirstURL(text string) string {
	found := firstURL.FindString(text)
	found = strings.TrimRight(found, ".,;:!?")

	address, ok := Normalise(found)
	if !ok {
		return ""
	}

	return address
}

// Snapshot is the copy of a preview a note keeps: as the page was when the
// note was written, and not as it is now.
type Snapshot struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	ImageKey    string `json:"image_key,omitempty"`
}

// NewSnapshot takes the copy of a preview to keep.
func NewSnapshot(preview *Preview) *Snapshot {
	if !preview.Usable() {
		return nil
	}

	return &Snapshot{
		URL:         preview.URL,
		Title:       Clip(preview.Title, MaxTitle),
		Description: Clip(preview.Description, MaxText),
		SiteName:    Clip(preview.SiteName, MaxSiteName),
		ImageKey:    preview.ImageKey,
	}
}

// The key a note's meta keeps its snapshot under.
const metaKey = "link_preview"

// FromMeta reads the snapshot out of a note's meta, or nil.
func FromMeta(meta []byte) *Snapshot {
	if len(meta) == 0 {
		return nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(meta, &fields); err != nil {
		return nil
	}
	raw, ok := fields[metaKey]
	if !ok {
		return nil
	}

	snapshot := &Snapshot{}
	if err := json.Unmarshal(raw, snapshot); err != nil || snapshot.URL == "" || snapshot.Title == "" {
		return nil
	}

	return snapshot
}

// IntoMeta puts a snapshot into a note's meta, beside whatever else is
// there, or takes it out when the snapshot is nil. Meta that cannot be read
// is begun again.
func IntoMeta(meta []byte, snapshot *Snapshot) []byte {
	fields := map[string]json.RawMessage{}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &fields)
		if fields == nil {
			fields = map[string]json.RawMessage{}
		}
	}

	if snapshot == nil {
		delete(fields, metaKey)
	} else {
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return meta
		}
		fields[metaKey] = raw
	}

	out, err := json.Marshal(fields)
	if err != nil {
		return meta
	}

	return out
}

// Clip cuts a text to at most this many characters, on one line, with an
// ellipsis where it was cut.
func Clip(text string, most int) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= most {
		return text
	}

	runes := []rune(text)

	return strings.TrimSpace(string(runes[:most-1])) + "…"
}
