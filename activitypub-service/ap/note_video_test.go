package ap

import (
	"encoding/json"
	"testing"
)

func TestAPostWithAVideo(t *testing.T) {
	for attachment, want := range map[string]bool{
		`[{"type":"Document","mediaType":"video/mp4","url":"https://elsewhere.example/a.mp4"}]`: true,
		`{"type":"Document","mediaType":"VIDEO/webm","url":"https://elsewhere.example/a.webm"}`: true,
		`[{"type":"Audio","url":"https://elsewhere.example/a.ogg"}]`:                            true,
		`[{"type":"Document","mediaType":"audio/ogg","url":"https://elsewhere.example/a.ogg"}]`: true,
		`[{"type":"Document","mediaType":"image/png","url":"https://elsewhere.example/a.png"}]`: false,
		`not one`: false,
	} {
		note := ReceivedNote{Attachment: json.RawMessage(attachment)}
		if got := note.HasVideo(); got != want {
			t.Errorf("%s: %v", attachment, got)
		}
	}
	if (&ReceivedNote{}).HasVideo() {
		t.Error("a post with nothing attached was said to have a video")
	}
}

func TestThePageOfAPost(t *testing.T) {
	const id = "https://elsewhere.example/users/data/statuses/1"
	for address, want := range map[string]string{
		`"https://elsewhere.example/@data/1"`: "https://elsewhere.example/@data/1",
		`"https://other.example/@data/1"`:     id,
		`"http://elsewhere.example/@data/1"`:  id,
		`""`:                                  id,
	} {
		note := ReceivedNote{ID: id, URL: json.RawMessage(address)}
		if got := note.Page(); got != want {
			t.Errorf("%s: %s", address, got)
		}
	}
	if got := (&ReceivedNote{ID: id}).Page(); got != id {
		t.Errorf("with no address: %s", got)
	}
}
