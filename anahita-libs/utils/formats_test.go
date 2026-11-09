package images

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

// The pictures in testdata are small: a 64×48 test card as WebP, HEIC
// and AVIF, and a half see-through red square as WebP. Made with ffmpeg
// and libheif's heif-enc.

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func TestEachKindIsKnownByItsBytes(t *testing.T) {
	for name, want := range map[string]string{
		"opaque.webp": kindWebP,
		"alpha.webp":  kindWebP,
		"small.heic":  kindHEIF,
		"small.avif":  kindHEIF,
	} {
		if got := kindOf(fixture(t, name)); got != want {
			t.Errorf("%s is %q, want %q", name, got, want)
		}
	}
	// A video is an ISO media file too, of brands that are not an image's.
	mp4 := append([]byte{0, 0, 0, 24}, []byte("ftypisom\x00\x00\x02\x00isomiso2")...)
	if kindOf(mp4) != "" {
		t.Error("an MP4 is taken for an image")
	}
}

func TestAHEIFSaysItsSizeBeforeItIsOpened(t *testing.T) {
	for _, name := range []string{"small.heic", "small.avif"} {
		// At least the picture, and no more than the coded size it is cut
		// from: a HEIC's is padded to whole blocks, 64×64 here.
		pixels, ok := heifPixels(fixture(t, name))
		if !ok || pixels < 64*48 || pixels > 64*64 {
			t.Errorf("%s: %d pixels, %v", name, pixels, ok)
		}
	}
	if _, ok := heifPixels([]byte("no boxes here")); ok {
		t.Error("a size found in nothing")
	}
}

func TestAWebPIsKeptAsAJPEGOrAPNG(t *testing.T) {
	content, kind, err := Accept(fixture(t, "opaque.webp"), 1<<20, 1<<20)
	if err != nil || kind != kindJPEG {
		t.Fatalf("an opaque WebP: %q, %v", kind, err)
	}
	if width, height, err := Dimensions(content); err != nil || width != 64 || height != 48 {
		t.Errorf("kept at %d×%d, %v", width, height, err)
	}

	// What is see-through stays so.
	if _, kind, err := Accept(fixture(t, "alpha.webp"), 1<<20, 1<<20); err != nil || kind != kindPNG {
		t.Errorf("a see-through WebP: %q, %v", kind, err)
	}

	// Held to the pixels allowed, from its header.
	if _, _, err := Accept(fixture(t, "opaque.webp"), 1<<20, 1000); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a WebP too large: %v", err)
	}
}

func TestAHEICOrAnAVIFIsReadWhereHeifConvertIs(t *testing.T) {
	_, err := exec.LookPath(HEIFConverter)
	for _, name := range []string{"small.heic", "small.avif"} {
		// Held to the pixels allowed before it is opened, either way.
		if _, _, err := Accept(fixture(t, name), 1<<20, 1000); !errors.Is(err, ErrTooLarge) {
			t.Errorf("%s too large: %v", name, err)
		}

		content, kind, acceptErr := Accept(fixture(t, name), 1<<20, 1<<20)
		if err != nil {
			// No heif-convert here: refused, as before.
			if !errors.Is(acceptErr, ErrUnsupported) {
				t.Errorf("%s without heif-convert: %v", name, acceptErr)
			}
			continue
		}
		if acceptErr != nil || kind != kindJPEG {
			t.Fatalf("%s: %q, %v", name, kind, acceptErr)
		}
		if width, height, _ := Dimensions(content); width != 64 || height != 48 {
			t.Errorf("%s kept at %d×%d", name, width, height)
		}
	}
}
