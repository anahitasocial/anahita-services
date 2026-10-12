package feeds

import (
	"bytes"
	"encoding/json"
)

// The pictures a feed may be given, by name. The web app draws each one; a
// name is all that is kept. They follow what a feed can be made of: kinds of
// profile and of post, a hashtag, a place, a keyword, a language, and a few
// for a feed that is none of those in particular.
var Icons = []string{
	"feed",
	"people", "group", "event",
	"note", "article", "topic", "photo",
	"hashtag", "place", "keyword", "language",
	"star", "heart", "news", "world",
}

// The picture of a feed that was given none.
const DefaultIcon = "feed"

// ValidIcon reports whether name is one of the pictures there are.
func ValidIcon(name string) bool {
	for _, icon := range Icons {
		if icon == name {
			return true
		}
	}

	return false
}

// Stored is what a feed's node keeps in its meta: what the feed shows, and
// what belongs to the feed and not to what it shows.
type Stored struct {
	Definition *Definition `json:"definition"`
	Icon       string      `json:"icon"`
}

// NewStored puts a definition and a picture together to be kept. A picture
// that is not one of ours is the default.
func NewStored(def *Definition, icon string) *Stored {
	if !ValidIcon(icon) {
		icon = DefaultIcon
	}

	return &Stored{Definition: def, Icon: icon}
}

// JSON is what goes in the node's meta.
func (s *Stored) JSON() []byte {
	raw, _ := json.Marshal(struct {
		Definition json.RawMessage `json:"definition"`
		Icon       string          `json:"icon"`
	}{s.Definition.JSON(), s.Icon})

	return raw
}

// ReadStored reads a feed's meta back. The definition is checked as it is
// whenever it is read. Meta that is a definition alone, as the first feeds
// were kept, is read as that definition with the default picture.
func ReadStored(raw []byte) (*Stored, error) {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &shape); err != nil {
		return nil, RefusedMalformed
	}

	inner, wrapped := shape["definition"]
	if !wrapped {
		def, err := Parse(raw)
		if err != nil {
			return nil, err
		}
		return NewStored(def, DefaultIcon), nil
	}

	def, err := Parse(inner)
	if err != nil {
		return nil, err
	}

	var icon string
	if value, ok := shape["icon"]; ok {
		_ = json.Unmarshal(value, &icon)
	}

	return NewStored(def, icon), nil
}
