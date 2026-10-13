package federation

import (
	"encoding/json"
	"testing"
)

func TestWithoutCustomEmoji(t *testing.T) {
	tags := json.RawMessage(`[{"type":"Emoji","name":":v_gqueer:","icon":{"url":"https://vsme.cc/e/1.png"}},{"type":"Hashtag","name":"#garden"},{"type":"Emoji","name":":blobcat:"}]`)

	for text, want := range map[string]string{
		"Boost :v_gqueer:":                     "Boost",
		":blobcat: Ana :v_gqueer: :blobcat:":   "Ana",
		"Beans :blobcat: are up\n\n:v_gqueer:": "Beans are up",
		// Not listed as an emoji: left as written.
		"Open at 12:30:45 :smile:": "Open at 12:30:45 :smile:",
		"Nothing here":             "Nothing here",
	} {
		if got := WithoutCustomEmoji(text, tags); got != want {
			t.Errorf("%q:\n got %q\nwant %q", text, got, want)
		}
	}

	// One thing listed, not a list; nothing listed; something that is not
	// a list at all.
	if got := WithoutCustomEmoji("Hi :wave:", json.RawMessage(`{"type":"Emoji","name":":wave:"}`)); got != "Hi" {
		t.Errorf("one tag: %q", got)
	}
	for _, raw := range []string{``, `null`, `"x"`, `[1,2]`} {
		if got := WithoutCustomEmoji("Hi :wave:", json.RawMessage(raw)); got != "Hi :wave:" {
			t.Errorf("tags %q: %q", raw, got)
		}
	}
}
