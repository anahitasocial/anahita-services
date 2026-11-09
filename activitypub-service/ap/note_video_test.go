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

// A video or a sound that came with a post is read with its picture, as
// Mastodon sends it (roadmap rows 120 and 137).
func TestAPostsVideoOrSoundIsReadWithItsPicture(t *testing.T) {
	var note ReceivedNote
	raw := `{"id":"https://m.example/notes/1","attachment":[
		{"type":"Document","mediaType":"image/png","url":"https://m.example/a.png"},
		{"type":"Document","mediaType":"video/mp4","url":"https://m.example/v.mp4","name":"A cat","icon":{"type":"Image","url":"https://m.example/v.jpg"}}]}`
	if err := json.Unmarshal([]byte(raw), &note); err != nil {
		t.Fatal(err)
	}
	media := note.Media()
	if media == nil || !media.IsVideo() || media.URL != "https://m.example/v.mp4" || media.Poster() != "https://m.example/v.jpg" {
		t.Fatalf("media: %+v", media)
	}

	_ = json.Unmarshal([]byte(`{"attachment":{"type":"Audio","mediaType":"audio/mpeg","url":"https://m.example/s.mp3","icon":"http://plain.example/c.jpg"}}`), &note)
	if media := note.Media(); media == nil || !media.IsAudio() || media.Poster() != "" {
		t.Errorf("sound: %+v", media)
	}
	_ = json.Unmarshal([]byte(`{"attachment":{"type":"Video","url":"http://plain.example/v.mp4"}}`), &note)
	if note.Media() != nil {
		t.Errorf("a video at a plain address was taken")
	}
}
