package ap

import "testing"

// A post that quotes ends with a line leading to what it quotes (roadmap
// row 156), in every language its words are in.
func TestANoteEndsWithWhatItQuotes(t *testing.T) {
	n := &Note{Content: "<p>Look</p>", ContentMap: map[string]string{"en": "<p>Look</p>"}}
	n.EndWithQuote("https://banu.io/photos/185877")
	want := `<p>Look</p><p>↪ <a href="https://banu.io/photos/185877">banu.io/photos/185877</a></p>`
	if n.Content != want || n.ContentMap["en"] != want {
		t.Errorf("content %q, in English %q", n.Content, n.ContentMap["en"])
	}
}
