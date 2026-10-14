// Package feeds holds what a custom feed is made of: the definition somebody
// builds in the editor, and the rules it has to keep.
//
// A definition is data and never query text. feed-service checks it when it
// is saved, and graph-grpc-service reads it again with the same code before
// building a query from it, so nothing reaches the database that was not
// read into the fields below.
package feeds

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"anahita-libs/constants"
)

// The node a custom feed is kept as, with its definition in the node's meta.
const (
	NodeType = "node.feed.feed-service.generator.v1"
	Service  = "feed-service"
	// From a person to a feed they keep among the tabs of their home page,
	// with its place among them in the edge's meta. Keeping somebody else's
	// feed there is how a feed is followed.
	EdgePinType = "edge.feed.pin.v1"
)

// Limits. Constants: a feed is cheap to make and is read often.
const (
	MaxActors    = 50
	MaxHashtags  = 10
	MaxKeywords  = 10
	MaxLocations = 5
	// A keyword's length, in characters.
	MinKeywordLength = 2
	MaxKeywordLength = 50
	// How many feeds one person may own, and keep on their home page. The
	// same number, so that every feed somebody makes has a place there:
	// the feeds on the home page are the ones listed in the menu.
	MaxOwned  = 10
	MaxPinned = 10
	// Posts in a page of a feed.
	PageSize = 20
	MaxPage  = 40
)

// Newest first, the one order there is. The key is kept in the definition so
// that another order is a value added here, and not a change of shape.
const OrderNewest = "newest"

// Definition is what a custom feed shows.
//
// Sources are added together: posts from whoever the feed's owner follows,
// and from the profiles named. With neither, the feed looks at the whole
// site, which it may only do through a hashtag, a keyword or a place.
//
// Filters narrow that, each kind in turn; inside a list, any one will do.
// What is excluded is left out whatever else is true.
type Definition struct {
	Sources Sources `json:"sources"`
	Filters Filters `json:"filters"`
	Exclude Exclude `json:"exclude"`
	// Replies are left out unless asked for: they are read in their
	// thread.
	IncludeReplies bool   `json:"include_replies"`
	Order          string `json:"order"`
}

type Sources struct {
	// Whoever the owner of the feed follows: people, groups and events.
	Following bool `json:"following"`
	// Profiles chosen by hand, of any kind.
	Actors []int64 `json:"actors"`
}

type Filters struct {
	// The kinds of profile a post is on: person, group, event. Empty for
	// any.
	ActorTypes []string `json:"actor_types"`
	// The kinds of post: note, article, topic, photo. Empty for every
	// kind.
	MediaTypes []string `json:"media_types"`
	Hashtags   []string `json:"hashtags_any"`
	Keywords   []string `json:"keywords_any"`
	Locations  []int64  `json:"locations_any"`
	// The language a post was written in, as its author said: "en", "fr".
	Language string `json:"language"`
}

type Exclude struct {
	Hashtags []string `json:"hashtags"`
	Keywords []string `json:"keywords"`
	Actors   []int64  `json:"actors"`
}

// The words a definition uses for kinds of post and of profile, and the
// types they stand for.
var mediaTypes = map[string]string{
	"note":    "node.medium.text-service.note.v1",
	"article": "node.medium.text-service.article.v1",
	"topic":   "node.medium.text-service.topic.v1",
	"photo":   constants.NodeMediumPhotoEntityType,
	"video":   "node.medium.video-service.video.v1",
}

var mediaOrder = []string{"note", "article", "topic", "photo", "video"}

// A person is somebody here or on another server: a feed kept to posts on
// people's profiles has the posts of people followed on Mastodon and
// Bluesky too.
var actorTypes = map[string][]string{
	"person": {
		constants.NodeActorPersonEntityType,
		constants.NodeActorActivityPubPersonEntityType,
		constants.NodeActorATProtoPersonEntityType,
	},
	"group": {"node.actor.group-service.group.v1"},
	"event": {constants.NodeActorEventEntityType},
}

// Why a definition was refused, as a word the web app has wording for.
type Refusal string

func (r Refusal) Error() string { return string(r) }

const (
	RefusedMalformed     Refusal = "malformed"
	RefusedUnknownField  Refusal = "unknown_field"
	RefusedTooBroad      Refusal = "too_broad"
	RefusedTooManyActors Refusal = "too_many_actors"
	RefusedTooManyTags   Refusal = "too_many_hashtags"
	RefusedTooManyWords  Refusal = "too_many_keywords"
	RefusedTooManyPlaces Refusal = "too_many_locations"
	RefusedKeywordLength Refusal = "keyword_length"
	RefusedHashtag       Refusal = "invalid_hashtag"
	RefusedMediaType     Refusal = "unknown_media_type"
	RefusedActorType     Refusal = "unknown_actor_type"
	RefusedActor         Refusal = "invalid_actor"
	RefusedLocation      Refusal = "invalid_location"
	RefusedLanguage      Refusal = "invalid_language"
	RefusedOrder         Refusal = "unknown_order"
)

// AsRefusal reports the reason in err, when it is one of the above.
func AsRefusal(err error) (Refusal, bool) {
	var refusal Refusal
	if errors.As(err, &refusal) {
		return refusal, true
	}

	return "", false
}

// Parse reads a definition from JSON, tidies it and checks it. A key it does
// not know is refused, so a definition never carries anything this code did
// not read.
func Parse(raw []byte) (*Definition, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, RefusedMalformed
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	def := &Definition{}
	if err := decoder.Decode(def); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return nil, RefusedUnknownField
		}
		return nil, RefusedMalformed
	}
	// One value, and nothing after it.
	if decoder.More() {
		return nil, RefusedMalformed
	}

	if err := def.Check(); err != nil {
		return nil, err
	}

	return def, nil
}

// Check tidies a definition and says whether it may be kept: lists without
// repeats or blanks, hashtags as they are stored, and every limit held.
func (d *Definition) Check() error {
	var err error

	d.Sources.Actors = uniqueIDs(d.Sources.Actors)
	d.Exclude.Actors = uniqueIDs(d.Exclude.Actors)
	d.Filters.Locations = uniqueIDs(d.Filters.Locations)

	for _, id := range append(append([]int64{}, d.Sources.Actors...), d.Exclude.Actors...) {
		if id <= 0 {
			return RefusedActor
		}
	}
	for _, id := range d.Filters.Locations {
		if id <= 0 {
			return RefusedLocation
		}
	}

	if d.Filters.Hashtags, err = cleanHashtags(d.Filters.Hashtags); err != nil {
		return err
	}
	if d.Exclude.Hashtags, err = cleanHashtags(d.Exclude.Hashtags); err != nil {
		return err
	}
	if d.Filters.Keywords, err = cleanKeywords(d.Filters.Keywords); err != nil {
		return err
	}
	if d.Exclude.Keywords, err = cleanKeywords(d.Exclude.Keywords); err != nil {
		return err
	}

	d.Filters.MediaTypes = uniqueWords(d.Filters.MediaTypes)
	for _, kind := range d.Filters.MediaTypes {
		if _, ok := mediaTypes[kind]; !ok {
			return RefusedMediaType
		}
	}
	d.Filters.ActorTypes = uniqueWords(d.Filters.ActorTypes)
	for _, kind := range d.Filters.ActorTypes {
		if _, ok := actorTypes[kind]; !ok {
			return RefusedActorType
		}
	}

	d.Filters.Language = strings.ToLower(strings.TrimSpace(d.Filters.Language))
	if !validLanguage(d.Filters.Language) {
		return RefusedLanguage
	}

	if d.Order == "" {
		d.Order = OrderNewest
	}
	if d.Order != OrderNewest {
		return RefusedOrder
	}

	switch {
	case len(d.Sources.Actors) > MaxActors, len(d.Exclude.Actors) > MaxActors:
		return RefusedTooManyActors
	case len(d.Filters.Hashtags) > MaxHashtags, len(d.Exclude.Hashtags) > MaxHashtags:
		return RefusedTooManyTags
	case len(d.Filters.Keywords) > MaxKeywords, len(d.Exclude.Keywords) > MaxKeywords:
		return RefusedTooManyWords
	case len(d.Filters.Locations) > MaxLocations:
		return RefusedTooManyPlaces
	}

	// The whole site is looked at only through a hashtag, a keyword or a
	// place. A feed of everything, or of every photo, is not one.
	if !d.HasSources() && !d.Narrows() {
		return RefusedTooBroad
	}

	return nil
}

// HasSources reports whether the feed names where its posts come from.
func (d *Definition) HasSources() bool {
	return d.Sources.Following || len(d.Sources.Actors) > 0
}

// Narrows reports whether the feed asks for a hashtag, a keyword or a place.
func (d *Definition) Narrows() bool {
	return len(d.Filters.Hashtags) > 0 || len(d.Filters.Keywords) > 0 || len(d.Filters.Locations) > 0
}

// MediaEntityTypes is the types of post the feed shows: the ones chosen, or
// every kind when none was.
func (d *Definition) MediaEntityTypes() []string {
	kinds := d.Filters.MediaTypes
	if len(kinds) == 0 {
		kinds = mediaOrder
	}

	types := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		types = append(types, mediaTypes[kind])
	}

	return types
}

// ActorEntityTypes is the types of profile the feed is kept to, or nothing
// for any.
func (d *Definition) ActorEntityTypes() []string {
	types := make([]string, 0, len(d.Filters.ActorTypes))
	for _, kind := range d.Filters.ActorTypes {
		types = append(types, actorTypes[kind]...)
	}

	return types
}

// JSON is the definition as it is stored and sent, with every list present.
func (d *Definition) JSON() []byte {
	out := *d
	out.Sources.Actors = orEmptyIDs(out.Sources.Actors)
	out.Exclude.Actors = orEmptyIDs(out.Exclude.Actors)
	out.Filters.Locations = orEmptyIDs(out.Filters.Locations)
	out.Filters.Hashtags = orEmptyWords(out.Filters.Hashtags)
	out.Filters.Keywords = orEmptyWords(out.Filters.Keywords)
	out.Filters.MediaTypes = orEmptyWords(out.Filters.MediaTypes)
	out.Filters.ActorTypes = orEmptyWords(out.Filters.ActorTypes)
	out.Exclude.Hashtags = orEmptyWords(out.Exclude.Hashtags)
	out.Exclude.Keywords = orEmptyWords(out.Exclude.Keywords)

	raw, _ := json.Marshal(out)

	return raw
}

// A hashtag as its node's alias holds it: no "#", lower case. Letters,
// digits and "_" of any script, as the pattern that finds them in a post
// allows.
func cleanHashtags(tags []string) ([]string, error) {
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(tag), "#"))
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > 100 || strings.ContainsAny(tag, " \t\n#,%") {
			return nil, RefusedHashtag
		}
		cleaned = append(cleaned, tag)
	}

	return uniqueWords(cleaned), nil
}

func cleanKeywords(words []string) ([]string, error) {
	cleaned := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.Join(strings.Fields(word), " ")
		if word == "" {
			continue
		}
		if length := utf8.RuneCountInString(word); length < MinKeywordLength || length > MaxKeywordLength {
			return nil, RefusedKeywordLength
		}
		cleaned = append(cleaned, word)
	}

	return uniqueWords(cleaned), nil
}

// "en", "fr", or with a region: "pt-br". Empty for any.
func validLanguage(tag string) bool {
	if tag == "" {
		return true
	}
	if len(tag) > 12 {
		return false
	}
	for _, r := range tag {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}

	return true
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	return unique
}

func uniqueWords(words []string) []string {
	seen := make(map[string]bool, len(words))
	unique := make([]string, 0, len(words))
	for _, word := range words {
		key := strings.ToLower(word)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, word)
		}
	}

	return unique
}

func orEmptyIDs(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}

func orEmptyWords(words []string) []string {
	if words == nil {
		return []string{}
	}
	return words
}
