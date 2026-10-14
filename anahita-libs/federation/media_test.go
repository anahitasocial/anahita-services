package federation

import (
	"strings"
	"testing"
	"time"
)

func TestWhatAPostSaysOfPicturesThatWereRemoved(t *testing.T) {
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

	// Beside whatever else the post keeps there, which is left.
	meta := []byte(`{"link_preview":{"url":"https://example.com","title":"T"}}`)
	marked := WithMediaRemoved(meta, &RemovedMedia{At: at, Alts: []string{"A row of beans", ""}})
	if !strings.Contains(string(marked), `"link_preview"`) {
		t.Fatalf("what else it kept was lost: %s", marked)
	}
	removed := MediaRemovedFrom(marked)
	if removed == nil || len(removed.Alts) != 2 || removed.Alts[0] != "A row of beans" || removed.Gone || !removed.At.Equal(at) {
		t.Fatalf("read back as %+v", removed)
	}

	// There again: it says nothing of them, and the rest is as it was.
	back := WithMediaRemoved(marked, nil)
	if MediaRemovedFrom(back) != nil || !strings.Contains(string(back), `"link_preview"`) {
		t.Errorf("after they were copied again: %s", back)
	}
	// And a post that kept nothing else keeps nothing.
	if only := WithMediaRemoved(WithMediaRemoved(nil, &RemovedMedia{At: at}), nil); only != nil {
		t.Errorf("left with %s", only)
	}

	for _, nothing := range []string{``, `{}`, `not json`, `{"media_removed":"words"}`} {
		if got := MediaRemovedFrom([]byte(nothing)); got != nil {
			t.Errorf("%q read as %+v", nothing, got)
		}
	}
}

func TestHowLongPicturesAreKept(t *testing.T) {
	for said, want := range map[string]int{"": 30, "7": 7, "0": 0, "-3": 30, "soon": 30} {
		t.Setenv("REMOTE_MEDIA_DAYS", said)
		if got := MediaDays(); got != want {
			t.Errorf("%q: %d days, want %d", said, got, want)
		}
	}
}
