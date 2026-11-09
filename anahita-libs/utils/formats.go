package images

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/image/webp"
)

// WebP, HEIC and AVIF (roadmap row 135): what phones and websites take
// photos in now, taken in and kept as a photo is, as JPEG, or PNG where
// something is see-through. They are made into one of those first, by
// Accept, so that nothing after it changes: every size is written out
// again from that, and nothing a file carries besides its picture is kept.

// HEIFConverter is the program HEIC and AVIF are read with: heif-convert,
// from libheif (Alpine's libheif-tools), in the images of the services that
// take pictures. Where it is not there, they are refused as before.
var HEIFConverter = "heif-convert"

// ConvertTimeout is the longest a HEIC or an AVIF may take to be read.
var ConvertTimeout = 60 * time.Second

// The kinds of image a file can be, judged from its bytes.
const (
	kindJPEG = "image/jpeg"
	kindPNG  = "image/png"
	kindWebP = "image/webp"
	kindHEIF = "image/heif"
)

// HEIF's brands: the image ones of HEIC and AVIF, not the image sequences
// (a moving HEIF), which are a video's.
var heifBrands = map[string]bool{
	"heic": true, "heix": true, "heim": true, "heis": true,
	"mif1": true, "mif2": true,
	"avif": true,
}

// kindOf is what an image is, from its bytes: JPEG, PNG, WebP or HEIF (a
// HEIC or an AVIF), or "" for anything else.
func kindOf(content []byte) string {
	switch detected := http.DetectContentType(content); detected {
	case kindJPEG, kindPNG, kindWebP:
		return detected
	}
	if isHEIF(content) {
		return kindHEIF
	}
	return ""
}

// isHEIF is whether a file is a HEIF image: an ISO media file whose `ftyp`
// box names an image brand, as its major brand or among those it is
// compatible with.
func isHEIF(content []byte) bool {
	if len(content) < 16 || string(content[4:8]) != "ftyp" {
		return false
	}
	size := int(binary.BigEndian.Uint32(content[0:4]))
	if size < 16 || size > len(content) || size > 4096 {
		return false
	}
	if heifBrands[string(content[8:12])] {
		return true
	}
	for at := 16; at+4 <= size; at += 4 {
		if heifBrands[string(content[at:at+4])] {
			return true
		}
	}
	return false
}

// heifPixels is the largest picture a HEIF file says it holds, from its
// `ispe` boxes (a photo from a phone is tiles of a larger one, which says
// its size too), read before any of it is opened. ok is false when it
// says none.
func heifPixels(content []byte) (int, bool) {
	largest, found := 0, false
	marker := []byte("ispe")
	for from := 0; ; {
		at := bytes.Index(content[from:], marker)
		if at < 0 {
			break
		}
		at += from
		// The box's version and flags, then its width and height.
		if at+16 <= len(content) {
			width := int(binary.BigEndian.Uint32(content[at+8 : at+12]))
			height := int(binary.BigEndian.Uint32(content[at+12 : at+16]))
			if width > 0 && height > 0 {
				found = true
				largest = max(largest, width*height)
			}
		}
		from = at + len(marker)
	}
	return largest, found
}

// readable makes a WebP, a HEIC or an AVIF a PNG, which AsPhoto makes a
// JPEG when nothing in it is see-through. A JPEG or a PNG comes back as
// it is. Its pixels are checked from its header first, as Accept checks a
// JPEG's.
func readable(content []byte, kind string, maxPixels int) ([]byte, error) {
	switch kind {
	case kindWebP:
		declared, err := webp.DecodeConfig(bytes.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
		if declared.Width*declared.Height > maxPixels {
			return nil, ErrTooLarge
		}
		picture, err := webp.Decode(bytes.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
		return asPNG(picture)
	case kindHEIF:
		pixels, ok := heifPixels(content)
		if !ok {
			return nil, fmt.Errorf("%w: it says no size", ErrUnreadable)
		}
		if pixels > maxPixels {
			return nil, ErrTooLarge
		}
		converted, err := convertHEIF(content)
		if err != nil {
			return nil, err
		}
		// What came out is held to the same, whatever the file said.
		declared, _, err := image.DecodeConfig(bytes.NewReader(converted))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
		if declared.Width*declared.Height > maxPixels {
			return nil, ErrTooLarge
		}
		return converted, nil
	}
	return content, nil
}

func asPNG(picture image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, picture); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return buf.Bytes(), nil
}

// convertHEIF has heif-convert write a HEIC or an AVIF out as a PNG, in a
// folder of its own that is removed after, within ConvertTimeout. libheif
// turns it the right way up as it does. A file it cannot read, or no
// heif-convert here, is not a supported image.
func convertHEIF(content []byte) ([]byte, error) {
	program, err := exec.LookPath(HEIFConverter)
	if HEIFConverter == "" || err != nil {
		return nil, ErrUnsupported
	}

	dir, err := os.MkdirTemp("", "heif-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	in := filepath.Join(dir, "in.heif")
	out := filepath.Join(dir, "out.png")
	if err := os.WriteFile(in, content, 0o600); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), ConvertTimeout)
	defer cancel()
	// Its primary image only: a HEIF can hold others, a depth map or the
	// frames of a burst, which are not the photo.
	cmd := exec.CommandContext(ctx, program, in, out)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%w: heif-convert: %v: %s", ErrUnreadable, err, bytes.TrimSpace(output))
	}

	converted, err := os.ReadFile(out)
	if err != nil {
		// Some versions number what they write when there is more than one.
		matches, _ := filepath.Glob(filepath.Join(dir, "out*.png"))
		if len(matches) == 0 {
			return nil, fmt.Errorf("%w: heif-convert wrote nothing", ErrUnreadable)
		}
		if converted, err = os.ReadFile(matches[0]); err != nil {
			return nil, err
		}
	}
	return converted, nil
}
