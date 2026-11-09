package ap

import (
	"encoding/json"
	"strings"
	"testing"

	"anahita-libs/data/entities"
)

// A post's content warning goes out as the web app shows it (roadmap row
// 122): pictures blurred and words shown, or words hidden behind it.
func TestAWarningGoesOutWithAPost(t *testing.T) {
	words := &Note{Type: "Note", Content: "<p>Fresh coffee</p>"}
	words.WithWarning(&entities.AnahitaNode{Warning: entities.WarningOther, WarningText: " Hot coffee "})
	if !words.Sensitive || words.Summary != "Hot coffee" {
		t.Errorf("words alone: sensitive %v, summary %q", words.Sensitive, words.Summary)
	}

	kind := &Note{Type: "Note"}
	kind.WithWarning(&entities.AnahitaNode{Warning: entities.WarningGraphic})
	if kind.Summary != "Graphic media" {
		t.Errorf("a kind is said by its name: %q", kind.Summary)
	}

	// Its words are shown: only what it carries is blurred.
	picture := &Note{Type: "Note", Attachment: []SentPicture{{Type: "Image"}}}
	picture.WithWarning(&entities.AnahitaNode{Warning: entities.WarningNudity})
	if !picture.Sensitive || picture.Summary != "" {
		t.Errorf("with a picture: sensitive %v, summary %q", picture.Sensitive, picture.Summary)
	}

	// An article keeps its opening as its summary.
	article := &Note{Type: "Article", Summary: "Its opening"}
	article.WithWarning(&entities.AnahitaNode{Warning: entities.WarningAdult})
	if !article.Sensitive || article.Summary != "Its opening" {
		t.Errorf("an article: sensitive %v, summary %q", article.Sensitive, article.Summary)
	}

	plain := &Note{Type: "Note"}
	plain.WithWarning(&entities.AnahitaNode{})
	sent, _ := json.Marshal(plain.WithContext())
	if plain.Sensitive || plain.Summary != "" || strings.Contains(string(sent), `"sensitive":true`) {
		t.Errorf("no warning: %s", sent)
	}
	// What sensitive means is said, as Mastodon says it.
	if !strings.Contains(string(sent), `"sensitive":"as:sensitive"`) {
		t.Errorf("sensitive is not declared: %s", sent)
	}
}
