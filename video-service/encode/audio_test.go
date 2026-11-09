package encode

import (
	"errors"
	"testing"
)

// The kinds of sound file taken are told by their first bytes; anything
// else is not a sound (roadmap row 137).
func TestASoundFileIsToldByItsFirstBytes(t *testing.T) {
	for want, head := range map[string][]byte{
		"wav":      []byte("RIFF\x00\x00\x00\x00WAVEfmt "),
		"flac":     []byte("fLaC\x00\x00\x00\x22"),
		"ogg":      []byte("OggS\x00\x02"),
		"mp3":      []byte("ID3\x04\x00\x00"),
		"aac":      {0xFF, 0xF1, 0x50, 0x80},
		"mov":      []byte("\x00\x00\x00\x20ftypM4A "),
		"matroska": {0x1A, 0x45, 0xDF, 0xA3, 0x01},
	} {
		if got := AudioContainer(head); got != want {
			t.Errorf("%s: %q", want, got)
		}
	}
	if got := AudioContainer([]byte{0xFF, 0xFB, 0x90}); got != "mp3" {
		t.Errorf("an MP3 frame: %q", got)
	}
	if got := AudioContainer([]byte("%PDF-1.7")); got != "" {
		t.Errorf("a PDF: %q", got)
	}
}

func TestASoundIsReadWithItsCover(t *testing.T) {
	info, err := ParseSoundProbe([]byte(`{"streams":[{"codec_type":"audio"},{"codec_type":"video","disposition":{"attached_pic":1}}],"format":{"duration":"183.4"}}`), 3600)
	if err != nil || !info.HasCover || info.Duration != 183.4 {
		t.Fatalf("%+v %v", info, err)
	}
	if _, err := ParseSoundProbe([]byte(`{"streams":[{"codec_type":"video"}],"format":{"duration":"5"}}`), 3600); !errors.Is(err, ErrNotASound) {
		t.Errorf("a video with no sound: %v", err)
	}
	if _, err := ParseSoundProbe([]byte(`{"streams":[{"codec_type":"audio"}],"format":{"duration":"4000"}}`), 3600); !errors.Is(err, ErrTooLong) {
		t.Errorf("too long: %v", err)
	}
}
