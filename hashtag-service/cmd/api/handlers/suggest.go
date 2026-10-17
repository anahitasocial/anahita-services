package handlers

import (
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// Hashtags in use, offered to somebody writing (roadmap row 78): as they
// type a hashtag, those that begin so; for long content, those that are
// words of what they wrote. The graph chooses and orders them
// (HashtagSuggestionsRepo): one they follow first, then the most used.

const (
	// Offered at once, unless fewer are asked for.
	suggestLimit = 8
	// The longest text looked at, in bytes: an article's worth.
	maxSuggestText = 100_000
	// The words of a text sent to the graph, the most used first.
	maxSuggestWords = 200
)

// SuggestRequest is a text to offer hashtags for, and what it is called.
type SuggestRequest struct {
	// The title, if it has one.
	Title string `json:"title"`
	// What is written, as Markdown.
	Body string `json:"body"`
	// How many to offer, 1 to 20; 8 when not given.
	Limit uint32 `json:"limit"`
}

// Suggestion is a hashtag in use, offered.
type Suggestion struct {
	ID int64 `json:"id"`
	// In lower case: what it is found by, and what to write after the #.
	Alias string `json:"alias"`
	// As it is shown.
	Name string `json:"name"`
	// How many posts it is on that anybody may see, and the viewer's own.
	Uses int64 `json:"uses"`
	// In one of the viewer's custom feeds.
	Followed bool `json:"followed"`
}

// SuggestResponse is the hashtags offered, the best first.
type SuggestResponse struct {
	Data []Suggestion `json:"data"`
}

// SuggestQuery is the start of a hashtag being typed.
type SuggestQuery struct {
	// What follows the #, at least one letter.
	Q string `query:"q"`
	// How many to offer, 1 to 20; 8 when not given.
	Limit uint32 `query:"limit"`
}

// Begun is the hashtags that begin with what was typed after a #.
func (h *hashtagHandler) Begun(c echo.Context) error {
	req := &SuggestQuery{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	prefix := strings.TrimPrefix(strings.TrimSpace(req.Q), "#")
	if prefix == "" || !wordLetters.MatchString(prefix) {
		return c.JSON(http.StatusOK, SuggestResponse{Data: []Suggestion{}})
	}
	return h.suggest(c, &services.HashtagSuggestRequest{Prefix: prefix, Limit: limitOf(req.Limit)})
}

// ForText is the hashtags in use that are words of a text, leaving out
// those it has already.
func (h *hashtagHandler) ForText(c echo.Context) error {
	req := &SuggestRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	text := req.Title + "\n\n" + req.Body
	if len(text) > maxSuggestText {
		text = text[:maxSuggestText]
	}

	written := &entities.AnahitaNode{Body: text}
	return h.suggest(c, &services.HashtagSuggestRequest{
		Words:   wordsOf(text, maxSuggestWords),
		Exclude: written.GetHashtags(),
		Limit:   limitOf(req.Limit),
	})
}

func (h *hashtagHandler) suggest(c echo.Context, req *services.HashtagSuggestRequest) error {
	found, err := h.Srv.HashtagGRPC.Suggest(req, getViewer(c))
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp := SuggestResponse{Data: []Suggestion{}}
	for _, each := range found {
		resp.Data = append(resp.Data, Suggestion{
			ID:       each.ID,
			Alias:    each.Alias,
			Name:     each.Name,
			Uses:     each.Uses,
			Followed: each.Followed,
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func limitOf(asked uint32) uint32 {
	if asked == 0 || asked > 20 {
		return suggestLimit
	}
	return asked
}

var (
	// The letters a hashtag is made of (entities.HashtagRegexPattern), and
	// nothing else.
	wordLetters = regexp.MustCompile(`^[\p{L}\p{N}_]+$`)
	// A word: letters and digits, apostrophes inside it dropped.
	aWord = regexp.MustCompile(`[\p{L}\p{N}_]+(?:['’][\p{L}\p{N}_]+)*`)
	// What is not words of the text: addresses, Markdown's link targets,
	// code, and hashtags and mentions as they are written.
	notWords = regexp.MustCompile("(?s)```.*?```|`[^`]*`|\\]\\([^)]*\\)|https?://\\S+|[#@][\\p{L}\\p{N}_@.]+")
)

// wordsOf is the words of a text, and each two that follow one another
// run together (open science, #openscience), in lower case, the most used
// first and then as they come; words of fewer than three letters left out.
func wordsOf(text string, most int) []string {
	text = notWords.ReplaceAllString(text, " ")

	count := map[string]int{}
	first := map[string]int{}
	add := func(word string) {
		if _, seen := count[word]; !seen {
			first[word] = len(first)
		}
		count[word]++
	}

	previous := ""
	for _, word := range aWord.FindAllString(strings.ToLower(text), -1) {
		word = strings.NewReplacer("'", "", "’", "").Replace(word)
		if len([]rune(word)) < 3 || len(word) > 64 {
			previous = ""
			continue
		}
		add(word)
		if previous != "" && len(previous)+len(word) <= 64 {
			add(previous + word)
		}
		previous = word
	}

	words := make([]string, 0, len(count))
	for word := range count {
		words = append(words, word)
	}
	sort.Slice(words, func(i, j int) bool {
		if count[words[i]] != count[words[j]] {
			return count[words[i]] > count[words[j]]
		}
		return first[words[i]] < first[words[j]]
	})
	if len(words) > most {
		words = words[:most]
	}
	return words
}
