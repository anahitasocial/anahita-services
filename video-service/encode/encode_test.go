package encode

import (
	"bytes"
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
		"a GIF89a": []byte("GIF89a\x40\x01\xf0\x00"),
		"a GIF87a": []byte("GIF87a\x40\x01\xf0\x00"),
	} {
		if kind, err := Container(head); err != nil || kind != GIF {
			t.Errorf("%s: %q %v", name, kind, err)
		}
	}
	if kind, err := Container(append(append([]byte{}, pngSignature...), 0, 0, 0, 13)); err != nil || kind != APNG {
		t.Errorf("a PNG: %q %v", kind, err)
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
		// ffmpeg cannot read the frames of one yet.
		"an animated WebP": []byte("RIFF\x00\x01\x00\x00WEBPVP8X"),
		"GIF in name only": []byte("GIF8"),
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

// chunk is one chunk of a PNG: its length, its name, what it holds and a
// checksum, which is not read.
func chunk(name string, data []byte) []byte {
	n := len(data)
	out := []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	out = append(out, name...)
	out = append(out, data...)
	return append(out, 0, 0, 0, 0)
}

func TestAPNGThatMovesIsToldFromAStillOne(t *testing.T) {
	header := chunk("IHDR", make([]byte, 13))
	moving := bytes.Join([][]byte{header, chunk("acTL", make([]byte, 8)), chunk("IDAT", []byte{1, 2})}, nil)
	still := bytes.Join([][]byte{header, chunk("tEXt", []byte("Comment\x00hello")), chunk("IDAT", []byte{1, 2})}, nil)
	// Said to move only once its picture has begun: not an animated PNG.
	late := bytes.Join([][]byte{header, chunk("IDAT", []byte{1, 2}), chunk("acTL", make([]byte, 8))}, nil)

	if moves, err := PNGMoves(bytes.NewReader(moving)); err != nil || !moves {
		t.Errorf("an animated PNG: %v %v", moves, err)
	}
	for name, data := range map[string][]byte{"a still PNG": still, "acTL after the picture": late} {
		if moves, err := PNGMoves(bytes.NewReader(data)); err != nil || moves {
			t.Errorf("%s: %v %v", name, moves, err)
		}
	}
	// Cut short, or a chunk that says it is longer than the file.
	for name, data := range map[string][]byte{
		"cut short":      header[:6],
		"a lying length": append(chunk("IHDR", make([]byte, 13))[:4:4], []byte("tEXt\xff")...),
	} {
		if _, err := PNGMoves(bytes.NewReader(data)); !errors.Is(err, ErrNotAVideo) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestAMovingPictureIsReadFromItsPictures(t *testing.T) {
	gif := `{"packets":[{"pts_time":"0","duration_time":"0.1"},{"pts_time":"0.1","duration_time":"0.1"},{"pts_time":"0.2","duration_time":"0.5"}],
		"streams":[{"codec_type":"video","width":321,"height":241}],"format":{"duration":"0.7"}}`
	if info, err := ParseAnimation([]byte(gif), 30); err != nil || !info.Animated || info.Frames != 3 || info.Duration != 0.7 || info.Width != 321 {
		t.Errorf("a GIF: %+v %v", info, err)
	}
	// An animated PNG says no length: it is where its last picture ends.
	apng := `{"packets":[{"pts_time":"0","duration_time":"0.1"},{"pts_time":"1.8","duration_time":"0.2"}],
		"streams":[{"codec_type":"video","width":200,"height":150}],"format":{"duration":"N/A"}}`
	if info, err := ParseAnimation([]byte(apng), 30); err != nil || info.Duration != 2.0 {
		t.Errorf("an animated PNG: %+v %v", info, err)
	}

	for name, c := range map[string]struct {
		output string
		want   error
	}{
		"one picture": {`{"packets":[{"pts_time":"0","duration_time":"0.04"}],"streams":[{"codec_type":"video","width":64,"height":48}],"format":{"duration":"0.04"}}`, ErrStill},
		"too long":    {`{"packets":[{},{}],"streams":[{"codec_type":"video","width":64,"height":48}],"format":{"duration":"45"}}`, ErrTooLong},
		"tiny":        {`{"packets":[{},{}],"streams":[{"codec_type":"video","width":8,"height":8}],"format":{"duration":"1"}}`, ErrNotAVideo},
		"no length":   {`{"packets":[{},{}],"streams":[{"codec_type":"video","width":64,"height":48}],"format":{}}`, ErrNotAVideo},
		"no picture":  {`{"packets":[{},{}],"streams":[],"format":{"duration":"1"}}`, ErrNotAVideo},
		"not json":    {`what`, ErrNotAVideo},
	} {
		if info, err := ParseAnimation([]byte(c.output), 30); !errors.Is(err, c.want) {
			t.Errorf("%s: %+v %v, wanted %v", name, info, err, c.want)
		}
	}
}

func TestAMovingPictureIsMadeOnceAtItsOwnSize(t *testing.T) {
	for name, c := range map[string]struct {
		info *Info
		want Target
	}{
		"small and odd":   {&Info{Width: 321, Height: 241}, Target{Height: 240, Width: 320}},
		"taller than 720": {&Info{Width: 1000, Height: 1000}, Target{Height: 720, Width: 720}},
		"wide and tall":   {&Info{Width: 1920, Height: 1080}, Target{Height: 720, Width: 1280}},
	} {
		if got := PlanAnimation(c.info); len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: %v, wanted %v", name, got, c.want)
		}
	}
}
