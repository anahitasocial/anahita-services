package handlers

import (
	"testing"

	"anahita-libs/data/entities"
)

func TestWhatADownloadedFileIsCalled(t *testing.T) {
	post := entities.NewAnahitaNode()
	post.ID = 170203

	for title, want := range map[string]string{
		"West End, Vancouver from the Burrard Bridge ": "west-end-vancouver-from-the-burrard-bridge.jpg",
		"":                    "photo-170203.jpg",
		"غروب":                "photo-170203.jpg",
		`a"b\r\nc/../d`:       "a-b-r-nc-d.jpg",
		"Shine on you (1975)": "shine-on-you-1975.jpg",
	} {
		post.Name = title
		if got := DownloadName(post, "photo", "", "photos/5/abc_original.jpg"); got != want {
			t.Errorf("%q: got %q, want %q", title, got, want)
		}
	}

	post.Name = "Sunset"
	if got := DownloadName(post, "photo", "2", "photos/5/abc_original.png"); got != "sunset-2.png" {
		t.Errorf("the second of several: %q", got)
	}
	if got := DownloadName(post, "video", "720p", "videos/5/x/720.mp4"); got != "sunset-720p.mp4" {
		t.Errorf("a size of a video: %q", got)
	}

	// What a post starts with: what was said, or what its profile says.
	yes, no := true, false
	group := entities.NewAnahitaNode()
	group.AllowDownload = true
	if !DefaultAllowDownload(nil, group) || DefaultAllowDownload(&no, group) || !DefaultAllowDownload(&yes, nil) || DefaultAllowDownload(nil, nil) {
		t.Errorf("the default is not what was said, then what the profile says")
	}
}
