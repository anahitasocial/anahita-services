package embeds

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func ours(address string) bool {
	return strings.HasPrefix(address, "https://example.org/")
}

func TestFind(t *testing.T) {
	text := strings.Join([]string{
		"An article about https://example.org/photos/3 in a sentence.",
		"",
		"https://example.org/photos/12",
		"  <https://example.org/articles/40-a-title>  ",
		"https://example.org/photos/12",
		"https://example.org/people/5",
		"https://Elsewhere.com/page#part",
		"https://elsewhere.com/page",
		"ftp://elsewhere.com/file",
		"see https://elsewhere.com/other",
		"[https://example.org/notes/7](https://example.org/notes/7)",
		"[https://example.org/notes/a\\_b](https://example.org/notes/a_b)",
		"[words](https://example.org/notes/8)",
	}, "\n")

	got := Find(text, ours)

	wantPosts := []Post{{Kind: "photo", ID: 12}, {Kind: "article", ID: 40}, {Kind: "note", ID: 7}}
	if !reflect.DeepEqual(got.Posts, wantPosts) {
		t.Errorf("posts: got %v, want %v", got.Posts, wantPosts)
	}
	wantPages := []string{"https://elsewhere.com/page"}
	if !reflect.DeepEqual(got.Pages, wantPages) {
		t.Errorf("pages: got %v, want %v", got.Pages, wantPages)
	}
}

func TestFindLimits(t *testing.T) {
	lines := []string{}
	for i := 1; i <= MaxPosts+5; i++ {
		lines = append(lines, "https://example.org/notes/"+strconv.Itoa(i))
	}
	got := Find(strings.Join(lines, "\n"), ours)
	if len(got.Posts) != MaxPosts {
		t.Errorf("got %d posts, want %d", len(got.Posts), MaxPosts)
	}
}

func TestFindNothing(t *testing.T) {
	got := Find("Words only.\nAnd more words.", ours)
	if len(got.Posts) != 0 || len(got.Pages) != 0 {
		t.Errorf("got %v, want nothing", got)
	}
}
