package federation

import "testing"

func TestAPostLeadsToItsVideo(t *testing.T) {
	const page = "https://bsky.app/profile/did:plc:a/post/1"
	line := "▶\ufe0e [bsky.app](" + page + ")"

	if got := WithVideoLink("Words", page, "bsky.app"); got != "Words\n\n"+line {
		t.Errorf("after words: %q", got)
	}
	if got := WithVideoLink("", page, "bsky.app"); got != line {
		t.Errorf("alone: %q", got)
	}
	if got := WithVideoLink("Words", "", "bsky.app"); got != "Words" {
		t.Errorf("with no page: %q", got)
	}
	// An address that would end the link and start something else.
	if got := WithVideoLink("Words", "https://elsewhere.example/a)[b](c", "elsewhere.example"); got != "Words" {
		t.Errorf("a bad address: %q", got)
	}
}
