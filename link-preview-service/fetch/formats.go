package fetch

import (
	"bytes"
	"image"
	"image/gif"
	"image/png"
	"net/http"

	"golang.org/x/image/webp"

	images "anahita-libs/utils"
)

// asPNG gives a picture this installation can work on. A page's picture may
// well be WebP or a GIF, and everything after here reads JPEG and PNG, so
// those two are written out again as PNG, which keeps what is see-through. A
// GIF becomes its first picture. Anything else is passed on as it came, for
// images.Accept to judge.
//
// Their size is read from the header first: a small file may say it holds an
// enormous picture, and it is not opened then.
func asPNG(raw []byte, maxPixels int) ([]byte, error) {
	var decode func(*bytes.Reader) (image.Image, error)
	var config func(*bytes.Reader) (image.Config, error)

	switch http.DetectContentType(raw) {
	case "image/webp":
		decode = func(r *bytes.Reader) (image.Image, error) { return webp.Decode(r) }
		config = func(r *bytes.Reader) (image.Config, error) { return webp.DecodeConfig(r) }
	case "image/gif":
		decode = func(r *bytes.Reader) (image.Image, error) { return gif.Decode(r) }
		config = func(r *bytes.Reader) (image.Config, error) { return gif.DecodeConfig(r) }
	default:
		return raw, nil
	}

	declared, err := config(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if declared.Width*declared.Height > maxPixels {
		return nil, images.ErrTooLarge
	}

	picture, err := decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}

	var written bytes.Buffer
	if err := png.Encode(&written, picture); err != nil {
		return nil, err
	}
	return written.Bytes(), nil
}
