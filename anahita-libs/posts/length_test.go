package posts

import (
	"strings"
	"testing"
)

// A link costs 23 however long its address, and a Markdown link its words
// (roadmap row 144); everything else, each character once.
func TestAPostsLengthCountsALinkAsTwentyThree(t *testing.T) {
	cases := []struct {
		words string
		want  int
	}{
		{"Tycho Station", 13},
		{"Ceres é 東京", 10},
		{"See https://example.org/" + strings.Repeat("a", 200) + "?utm_source=x", 4 + LinkLength},
		{"Read [the log](https://example.org/a/very/long/path) now", 5 + 7 + 4},
		{"https://a.example https://b.example", LinkLength + 1 + LinkLength},
		{"**bold** marks count", 20},
	}
	for _, each := range cases {
		if got := Length(each.words); got != each.want {
			t.Errorf("Length(%q) = %d, want %d", each.words, got, each.want)
		}
	}
	if !Fits(strings.Repeat("a", MaxLength)) || Fits(strings.Repeat("a", MaxLength+1)) {
		t.Error("the limit is not 500")
	}
	if !Fits(strings.Repeat("a", MaxLength-LinkLength-1) + " https://example.org/" + strings.Repeat("b", 400)) {
		t.Error("a long address took more than 23")
	}
}
