package encode

import (
	"errors"
	"reflect"
	"testing"
)

func TestOnlyAVideoIsTakenForOne(t *testing.T) {
	mp4 := append([]byte{0, 0, 0, 0x20}, []byte("ftypisom\x00\x00\x02\x00")...)
	webm := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81}

	if kind, err := Container(mp4); err != nil || kind != "mov" {
		t.Errorf("an MP4: %q %v", kind, err)
	}
	if kind, err := Container(webm); err != nil || kind != "matroska" {
		t.Errorf("a WebM: %q %v", kind, err)
	}
	for name, head := range map[string][]byte{
		"a page":      []byte("<html><script>alert(1)</script>"),
		"a playlist":  []byte("#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:0\nfile:///etc/passwd"),
		"a concat":    []byte("ffconcat version 1.0\nfile /etc/passwd"),
		"a picture":   {0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F', 0, 1},
		"a zip":       []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00abcd"),
		"a program":   []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00"),
		"nothing":     {},
		"a few bytes": []byte("ftyp"),
	} {
		if kind, err := Container(head); !errors.Is(err, ErrNotAVideo) {
			t.Errorf("%s was taken for %q", name, kind)
		}
	}
}

func TestWhatAFileIsIsReadFromWhatFfprobeSays(t *testing.T) {
	landscape := `{"streams":[
		{"codec_type":"audio"},
		{"codec_type":"video","width":1920,"height":1080}
	],"format":{"duration":"61.5"}}`
	info, err := ParseProbe([]byte(landscape), 600)
	if err != nil || info.Width != 1920 || info.Height != 1080 || info.Duration != 61.5 || !info.HasAudio {
		t.Errorf("got %+v %v", info, err)
	}

	// Filmed on a phone held upright: stored on its side, shown turned.
	for _, upright := range []string{
		`{"streams":[{"codec_type":"video","width":1920,"height":1080,"tags":{"rotate":"90"}}],"format":{"duration":"5"}}`,
		`{"streams":[{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"rotation":-90}]}],"format":{"duration":"5"}}`,
		`{"streams":[{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"rotation":270}]}],"format":{"duration":"5"}}`,
	} {
		info, err := ParseProbe([]byte(upright), 600)
		if err != nil || info.Width != 1080 || info.Height != 1920 || info.HasAudio {
			t.Errorf("%s: got %+v %v", upright, info, err)
		}
	}
	if info, _ := ParseProbe([]byte(`{"streams":[{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"rotation":180}]}],"format":{"duration":"5"}}`), 600); info == nil || info.Width != 1920 {
		t.Errorf("turned upside down changed its shape: %+v", info)
	}

	for name, output := range map[string]string{
		"only sound":               `{"streams":[{"codec_type":"audio"}],"format":{"duration":"61.5"}}`,
		"a song with its cover":    `{"streams":[{"codec_type":"audio"},{"codec_type":"video","width":500,"height":500,"disposition":{"attached_pic":1}}],"format":{"duration":"200"}}`,
		"no length":                `{"streams":[{"codec_type":"video","width":640,"height":360}],"format":{}}`,
		"a length that is not one": `{"streams":[{"codec_type":"video","width":640,"height":360}],"format":{"duration":"N/A"}}`,
		"no size":                  `{"streams":[{"codec_type":"video"}],"format":{"duration":"5"}}`,
		"nothing ffprobe said":     `not json`,
	} {
		if info, err := ParseProbe([]byte(output), 600); !errors.Is(err, ErrNotAVideo) {
			t.Errorf("%s was taken for a video: %+v %v", name, info, err)
		}
	}

	long := `{"streams":[{"codec_type":"video","width":640,"height":360}],"format":{"duration":"660.0"}}`
	if _, err := ParseProbe([]byte(long), 600); !errors.Is(err, ErrTooLong) {
		t.Errorf("eleven minutes: %v", err)
	}
	// The container rounding up is not a longer video.
	nearly := `{"streams":[{"codec_type":"video","width":640,"height":360}],"format":{"duration":"600.4"}}`
	if _, err := ParseProbe([]byte(nearly), 600); err != nil {
		t.Errorf("ten minutes and a fraction: %v", err)
	}
}

func TestAVideoIsNeverMadeLargerThanItIs(t *testing.T) {
	for name, c := range map[string]struct {
		info *Info
		want []Target
	}{
		"full HD":           {&Info{Width: 1920, Height: 1080}, []Target{{720, 1280}, {360, 640}}},
		"exactly 720":       {&Info{Width: 1280, Height: 720}, []Target{{720, 1280}, {360, 640}}},
		"between the two":   {&Info{Width: 854, Height: 480}, []Target{{360, 640}}},
		"smaller than both": {&Info{Width: 320, Height: 240}, []Target{{360, 320}}},
		"upright":           {&Info{Width: 1080, Height: 1920}, []Target{{720, 404}, {360, 202}}},
		"an odd width":      {&Info{Width: 321, Height: 241}, []Target{{360, 320}}},
	} {
		if got := Plan(c.info, []int{360, 720}); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
	if got := Plan(&Info{Width: 1920, Height: 1080}, nil); len(got) != 0 {
		t.Errorf("no sizes asked for: %v", got)
	}
}
