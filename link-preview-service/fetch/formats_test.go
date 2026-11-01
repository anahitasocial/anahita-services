package fetch

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"net/http"
	"testing"

	images "anahita-libs/utils"
)

// The smallest WebP there is: one pixel.
const onePixelWebP = "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA"

func aGIF(t *testing.T) []byte {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, 4, 2), color.Palette{color.White, color.Black})
	var out bytes.Buffer
	if err := gif.Encode(&out, frame, nil); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestAPictureAnyPageMayUseBecomesOneWeRead(t *testing.T) {
	webp, err := base64.StdEncoding.DecodeString(onePixelWebP)
	if err != nil {
		t.Fatal(err)
	}

	for name, raw := range map[string][]byte{"WebP": webp, "GIF": aGIF(t)} {
		t.Run(name, func(t *testing.T) {
			readable, err := asPNG(raw, MaxImagePixels)
			if err != nil {
				t.Fatalf("not read: %v", err)
			}
			if kind := http.DetectContentType(readable); kind != "image/png" {
				t.Errorf("became %s", kind)
			}
			if _, _, err := images.Accept(readable, MaxImageBytes, MaxImagePixels); err != nil {
				t.Errorf("not accepted after: %v", err)
			}
		})
	}
}

func TestAJPEGOrPNGIsLeftAsItCame(t *testing.T) {
	raw := []byte("\xff\xd8\xff\xe0 not WebP or GIF")
	readable, err := asPNG(raw, MaxImagePixels)
	if err != nil || !bytes.Equal(readable, raw) {
		t.Errorf("changed: %q, %v", readable, err)
	}
}

func TestAPictureSayingItIsEnormousIsNotOpened(t *testing.T) {
	if _, err := asPNG(aGIF(t), 1); !errors.Is(err, images.ErrTooLarge) {
		t.Errorf("err = %v, want too large", err)
	}
}
