// Package ranking holds the weights search orders "Most relevant" by.
//
// The order is in three parts, one after the other (see
// data/repositories/search.go):
//
//  1. how well a result matches what was searched for, by where the words
//     are: the handle, the name, a hashtag, the body;
//  2. within the same match, its standing: whether the searcher follows it,
//     and how useful people have found it, by its likes, saves, replies,
//     reposts, quotes and followers;
//  3. its id, so that pages never overlap.
//
// Old and useful counts as much as new. What fades, if anything, is
// interest: each like, save, reply or follow counts by when it happened,
// halved every HalfLifeDays, so that a guide from years ago that people
// still save stands as high as a post from this week. A newsroom may set a
// month; a library, a school or a research group a year, or 0 for never.
//
// The weights are the operator's, in a JSON file (SEARCH_RANKING_PATH,
// graph-grpc-defaults/search_ranking.json in the image). It is read again
// when it changes, without a restart. A missing or broken file, or a value
// out of bounds, leaves the weights as they come.
package ranking

import (
	"anahita-libs/constants"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// DefaultPath is where the image keeps the file.
const DefaultPath = "/app/defaults/search_ranking.json"

// Match is the points for where the words were found. Only their order
// matters: a result is ranked by its match first, and only then by its
// standing.
type Match struct {
	Handle       float64 `json:"handle"`
	Name         float64 `json:"name"`
	NameStarts   float64 `json:"name_starts"`
	WordStarts   float64 `json:"word_starts"`
	NameContains float64 `json:"name_contains"`
	Hashtag      float64 `json:"hashtag"`
	BodyWord     float64 `json:"body_word"`
	BodyContains float64 `json:"body_contains"`
}

// Useful is how much each sign that somebody found something useful
// counts, and the scale of the whole once it is put on a logarithm.
type Useful struct {
	Scale  float64 `json:"scale"`
	Like   float64 `json:"like"`
	Save   float64 `json:"save"`
	Reply  float64 `json:"reply"`
	Follow float64 `json:"follow"`
	// Cite is each person who cites it, a post or a profile: shown it
	// within an article or a topic of their own (nodes.citation_count).
	// More than a like or a save, since citing takes deliberate effort; a
	// starting value, to be tuned once there are citations to look at (the
	// user, 2026-10-16). Counted as it stands, not halved with time: a
	// citation is lasting, and useful knowledge stays useful.
	Cite float64 `json:"cite"`
}

// Weights is the whole of it.
type Weights struct {
	Match Match `json:"match"`
	// Following is added to the standing of what the searcher follows, or
	// what is on a profile they follow.
	Following float64 `json:"following"`
	// PicturedWithFollowed is added to the standing of a photo or a video
	// with somebody the searcher follows in it, and of a person in public
	// photos or videos with them (roadmap row 104). Approved tags only.
	PicturedWithFollowed float64 `json:"pictured_with_followed"`
	Useful               Useful  `json:"useful"`
	// HalfLifeDays halves the weight of each sign of interest every so
	// many days after it happened. 0 for never.
	HalfLifeDays float64 `json:"half_life_days"`
}

// Default is the weights as they come. Interest a year old counts half:
// slow enough that what is still used stays high, fast enough that what
// nobody has touched in years gives way.
func Default() Weights {
	return Weights{
		Match: Match{
			Handle: 100, Name: 90, NameStarts: 70, WordStarts: 60,
			NameContains: 50, Hashtag: 40, BodyWord: 20, BodyContains: 10,
		},
		Following:            15,
		PicturedWithFollowed: 8,
		Useful:               Useful{Scale: 5, Like: 1, Save: 2, Reply: 2, Follow: 1, Cite: 3},
		HalfLifeDays:         365,
	}
}

// The largest a weight may be, and the longest half-life in days, near
// three years; longer is as good as never, which is 0. A file with a value
// outside 0 to this is not taken.
const most = 1000

func (w Weights) valid() bool {
	values := []float64{
		w.Match.Handle, w.Match.Name, w.Match.NameStarts, w.Match.WordStarts,
		w.Match.NameContains, w.Match.Hashtag, w.Match.BodyWord, w.Match.BodyContains,
		w.Following, w.PicturedWithFollowed, w.Useful.Scale, w.Useful.Like, w.Useful.Save, w.Useful.Reply,
		w.Useful.Follow, w.Useful.Cite, w.HalfLifeDays,
	}
	for _, value := range values {
		if value < 0 || value > most {
			return false
		}
	}
	return true
}

// Parse reads weights from JSON. What it does not say is as it comes.
func Parse(raw []byte) (Weights, bool) {
	weights := Default()
	if err := json.Unmarshal(raw, &weights); err != nil || !weights.valid() {
		return Default(), false
	}
	return weights, true
}

// Source reads the file, again when it has changed.
type Source struct {
	path string

	mu       sync.Mutex
	modified time.Time
	weights  Weights
}

// NewSource is weights read from `path`.
func NewSource(path string) *Source {
	return &Source{path: path, weights: Default()}
}

// FromEnv is weights read from SEARCH_RANKING_PATH, or the image's file.
func FromEnv() *Source {
	path := os.Getenv("SEARCH_RANKING_PATH")
	if path == "" {
		path = DefaultPath
	}
	return NewSource(path)
}

// Weights is what the file says now.
func (s *Source) Weights() Weights {
	if s == nil {
		return Default()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		// No file: as they come.
		s.weights, s.modified = Default(), time.Time{}
		return s.weights
	}
	if info.ModTime().Equal(s.modified) {
		return s.weights
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return s.weights
	}
	weights, ok := Parse(raw)
	if !ok {
		log.Printf("search ranking: %s is not understood; the weights are as they come", s.path)
	}
	s.weights, s.modified = weights, info.ModTime()
	return s.weights
}

// What somebody may adjust for themselves, and how far (the user,
// 2026-10-16: as they adjust their recommendations). Each is a step that
// multiplies the installation's own weight: Off, Less, As is, More, Much
// more. So what the operator tunes still holds for everybody, and each
// person leans it their way; what words match where is not theirs to move.
var Adjustable = constants.SearchAdjustable

// Steps is what each may be set to. 1 is as the installation has it.
var Steps = constants.SearchSteps

// IsStep says whether a value is one of Steps.
func IsStep(value float64) bool {
	for _, step := range Steps {
		if value == step {
			return true
		}
	}
	return false
}

// Adjusted is the weights with somebody's own steps applied. A name or a
// value that is not one is left as the installation has it.
func (w Weights) Adjusted(steps map[string]float64) Weights {
	for name, step := range steps {
		if !IsStep(step) {
			continue
		}
		switch name {
		case "like":
			w.Useful.Like *= step
		case "save":
			w.Useful.Save *= step
		case "reply":
			w.Useful.Reply *= step
		case "follow":
			w.Useful.Follow *= step
		case "cite":
			w.Useful.Cite *= step
		case "following":
			// Being pictured with them is a kind of following them.
			w.Following *= step
			w.PicturedWithFollowed *= step
		}
	}
	return w
}
