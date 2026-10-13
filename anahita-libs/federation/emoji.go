package federation

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Custom emoji: a small picture a server's administrator added, written in
// text as a word between colons (":blobcat:") and listed beside the text
// with where the picture is.
//
// This installation does not show them yet: that means copying each
// picture into its own storage, as it does an avatar. Until it does, the
// word between colons is taken out, so that somebody called "Boost
// :v_gqueer:" where they are is called "Boost" here and not by a piece of
// markup. Only a word that is listed as an emoji goes: a time written
// 12:30:45, or anything else between colons, is left alone.

// One listed thing beside a text. Only an emoji's name matters here.
type listed struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

var manySpaces = regexp.MustCompile(`[ \t]{2,}`)

// WithoutCustomEmoji takes out of `text` the custom emoji that `tags`
// lists: what a person's description or a post carries as its "tag", one
// thing or a list of them.
func WithoutCustomEmoji(text string, tags json.RawMessage) string {
	if text == "" || len(tags) == 0 || !strings.Contains(text, ":") {
		return text
	}

	var many []listed
	if json.Unmarshal(tags, &many) != nil {
		var one listed
		if json.Unmarshal(tags, &one) != nil {
			return text
		}
		many = []listed{one}
	}

	changed := false
	for _, tag := range many {
		name := strings.TrimSpace(tag.Name)
		if tag.Type != "Emoji" || len(name) < 3 || !strings.HasPrefix(name, ":") || !strings.HasSuffix(name, ":") {
			continue
		}
		if strings.Contains(text, name) {
			text = strings.ReplaceAll(text, name, "")
			changed = true
		}
	}
	if !changed {
		return text
	}

	// The space the word left behind, on each line.
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(manySpaces.ReplaceAllString(line, " "), " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
