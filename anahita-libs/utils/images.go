package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
)

// ALLOWED_MIMETYPES is a list of allowed mimetypes
var ALLOWED_MIMETYPES = []string{
	"image/jpeg",
	"image/jpg",
	"image/png",
}

func IsValidMimeType(mimeType string) bool {
	for _, allowed := range ALLOWED_MIMETYPES {
		if mimeType == allowed {
			return true
		}
	}

	return false
}

func GenerateUniqueFilename(contentType string) (string, error) {
	name := uuid.New().String()

	if contentType == "image/jpeg" {
		return name + ".jpg", nil
	} else if contentType == "image/png" {
		return name + ".png", nil
	}

	return "", fmt.Errorf("unsupported content type: %s", contentType)
}

// JPEGQuality is how every JPEG is written: 85, the usual balance of size
// and detail for photographs, as most networks keep them. Go's own default
// is 75, which showed. (The user, 2026-10-16.)
const JPEGQuality = 85

var jpegOptions = &jpeg.Options{Quality: JPEGQuality}

// ResizeImage resizes the image based on the provided width and height
func ResizeImage(input []byte, width, height int) ([]byte, error) {
	// Detect image format
	contentType := http.DetectContentType(input)
	var img image.Image
	var err error

	// Decode image based on format
	if contentType == "image/jpeg" {
		img, err = jpeg.Decode(bytes.NewReader(input))
		if err != nil {
			return nil, fmt.Errorf("failed to decode jpeg: %w", err)
		}
	} else if contentType == "image/png" {
		img, err = png.Decode(bytes.NewReader(input))
		if err != nil {
			return nil, fmt.Errorf("failed to decode png: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported image format: %s", contentType)
	}

	// If height is 0, calculate the height to maintain aspect ratio
	if height == 0 {
		aspectRatio := float64(img.Bounds().Dy()) / float64(img.Bounds().Dx())
		height = int(float64(width) * aspectRatio)
	}

	// Resize the image
	resizedImage := imaging.Resize(img, width, height, imaging.Lanczos)

	// Buffer to store the resized image
	var buf bytes.Buffer

	// Encode the resized image back to the original format
	if contentType == "image/jpeg" {
		err = jpeg.Encode(&buf, resizedImage, jpegOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to encode jpeg: %w", err)
		}
	} else if contentType == "image/png" {
		err = png.Encode(&buf, resizedImage)
		if err != nil {
			return nil, fmt.Errorf("failed to encode png: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// CorrectOrientation corrects the image orientation based on EXIF metadata from []byte input
func CorrectOrientation(input []byte) ([]byte, error) {
	// Detect the content type of the image (JPEG, PNG, etc.)
	contentType := http.DetectContentType(input)

	// Decode the image based on the detected format
	var img image.Image
	var err error
	switch contentType {
	case "image/jpeg":
		img, err = jpeg.Decode(bytes.NewReader(input))
		if err != nil {
			return nil, fmt.Errorf("failed to decode jpeg: %w", err)
		}
	case "image/png":
		img, err = png.Decode(bytes.NewReader(input))
		if err != nil {
			return nil, fmt.Errorf("failed to decode png: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported image format: %s", contentType)
	}

	// Decode EXIF metadata from the input byte slice
	exifData, err := exif.Decode(bytes.NewReader(input))
	if err != nil {
		// If there's no EXIF metadata, return the original image bytes
		fmt.Println("No EXIF metadata found, returning original image")
		return input, nil
	}

	// Get the Orientation tag from EXIF data
	orientationTag, err := exifData.Get(exif.Orientation)
	if err != nil {
		// If no orientation tag is found, return the original image bytes
		fmt.Println("No Orientation tag found in EXIF, returning original image")
		return input, nil
	}

	orientation, err := orientationTag.Int(0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse orientation tag: %w", err)
	}

	// Apply the correct transformation based on the orientation value
	switch orientation {
	case 3:
		// Rotate 180 degrees
		img = imaging.Rotate180(img)
	case 6:
		// Rotate 90 degrees clockwise
		img = imaging.Rotate270(img)
	case 8:
		// Rotate 90 degrees counter-clockwise
		img = imaging.Rotate90(img)
	default:
		// If no rotation is needed, return the original image
		return input, nil
	}

	// Encode the corrected image back to the original format
	var output bytes.Buffer
	switch contentType {
	case "image/jpeg":
		err = jpeg.Encode(&output, img, jpegOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to encode jpeg: %w", err)
		}
	case "image/png":
		err = png.Encode(&output, img)
		if err != nil {
			return nil, fmt.Errorf("failed to encode png: %w", err)
		}
	}

	return output.Bytes(), nil
}

// decodeImage reads a JPEG or PNG and says which it was.
func decodeImage(input []byte) (image.Image, string, error) {
	contentType := http.DetectContentType(input)

	switch contentType {
	case "image/jpeg":
		img, err := jpeg.Decode(bytes.NewReader(input))
		if err != nil {
			return nil, "", fmt.Errorf("failed to decode jpeg: %w", err)
		}
		return img, contentType, nil
	case "image/png":
		img, err := png.Decode(bytes.NewReader(input))
		if err != nil {
			return nil, "", fmt.Errorf("failed to decode png: %w", err)
		}
		return img, contentType, nil
	}

	return nil, "", fmt.Errorf("unsupported image format: %s", contentType)
}

// encodeImage writes an image out in the format it came in.
func encodeImage(img image.Image, contentType string) ([]byte, error) {
	var buf bytes.Buffer

	switch contentType {
	case "image/jpeg":
		if err := jpeg.Encode(&buf, img, jpegOptions); err != nil {
			return nil, fmt.Errorf("failed to encode jpeg: %w", err)
		}
	case "image/png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("failed to encode png: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported image format: %s", contentType)
	}

	return buf.Bytes(), nil
}

// FitImage writes the image out again no wider than maxWidth, keeping its
// shape.
//
// An image already narrower is written out at its own size, never enlarged:
// enlarging adds bytes and no detail. Either way the result is a fresh
// encoding of the picture alone, so nothing else a file carried survives.
func FitImage(input []byte, maxWidth int) ([]byte, error) {
	img, contentType, err := decodeImage(input)
	if err != nil {
		return nil, err
	}

	if maxWidth > 0 && img.Bounds().Dx() > maxWidth {
		// Height 0 keeps the shape.
		img = imaging.Resize(img, maxWidth, 0, imaging.Lanczos)
	}

	return encodeImage(img, contentType)
}

// FitLongest writes the image out again with its longest side no longer
// than maxSide, keeping its shape: a photo standing up is measured by its
// height, one lying down by its width, as Bluesky and Pixelfed measure
// them. Never enlarged.
func FitLongest(input []byte, maxSide int) ([]byte, error) {
	img, contentType, err := decodeImage(input)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	if maxSide > 0 && (bounds.Dx() > maxSide || bounds.Dy() > maxSide) {
		if bounds.Dx() >= bounds.Dy() {
			img = imaging.Resize(img, maxSide, 0, imaging.Lanczos)
		} else {
			img = imaging.Resize(img, 0, maxSide, imaging.Lanczos)
		}
	}

	return encodeImage(img, contentType)
}

// AsPhoto is a picture as a photo is kept: a PNG with nothing see-through
// in it written as a JPEG, which is a fifth of its size or less and what
// every network takes; a PNG that is see-through somewhere, and a JPEG,
// as they are. Says what it is now.
func AsPhoto(input []byte, contentType string) ([]byte, string, error) {
	if contentType != "image/png" {
		return input, contentType, nil
	}
	img, err := png.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if opaque, ok := img.(interface{ Opaque() bool }); !ok || !opaque.Opaque() {
		return input, contentType, nil
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, jpegOptions); err != nil {
		return nil, "", fmt.Errorf("failed to encode jpeg: %w", err)
	}
	return buf.Bytes(), "image/jpeg", nil
}

// SquareImage writes out the middle of the image as a square of this side.
//
// Cut, not squeezed: resizing to a square without cutting stretches
// everything that was not square to begin with.
func SquareImage(input []byte, side int) ([]byte, error) {
	img, contentType, err := decodeImage(input)
	if err != nil {
		return nil, err
	}

	// No larger than the image's own shorter side, for the reason FitImage
	// does not enlarge.
	if shorter := min(img.Bounds().Dx(), img.Bounds().Dy()); shorter < side {
		side = shorter
	}

	return encodeImage(imaging.Fill(img, side, side, imaging.Center, imaging.Lanczos), contentType)
}

// Why an uploaded file was not accepted as an image.
var (
	// Not a JPEG or a PNG, judged from its bytes.
	ErrUnsupported = errors.New("not a supported image")
	// More bytes, or more pixels, than is accepted.
	ErrTooLarge = errors.New("image is too large")
	// It claims to be an image and cannot be read as one.
	ErrUnreadable = errors.New("image cannot be read")
)

// Accept checks an uploaded file and returns it turned the right way up,
// with what kind of image it is.
//
// The one way in for anything a person uploads as an image, whatever it is
// for: a photo, an avatar, a cover. So the rules are the same for all of
// them, and are these:
//
//   - What a file is, is decided from its bytes. A page of HTML named
//     photo.jpg is refused.
//   - Its size is checked twice, in bytes and in pixels, and the pixels are
//     read from the header before any of the picture is opened. A small file
//     can describe an enormous picture, and opening one takes memory by the
//     pixel.
//   - It comes back upright, so every size made from it is too.
//
// What is returned is still what was uploaded, and is never to be stored as
// it is: every stored size is written out again by FitImage or SquareImage,
// so nothing a file carries besides its picture reaches storage.
func Accept(content []byte, maxBytes, maxPixels int) ([]byte, string, error) {
	if !IsValidMimeType(http.DetectContentType(content)) {
		return nil, "", ErrUnsupported
	}

	if len(content) > maxBytes {
		return nil, "", ErrTooLarge
	}

	declared, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if declared.Width*declared.Height > maxPixels {
		return nil, "", ErrTooLarge
	}

	upright, err := CorrectOrientation(content)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnreadable, err)
	}

	mimeType := http.DetectContentType(upright)
	if !IsValidMimeType(mimeType) {
		return nil, "", ErrUnsupported
	}

	return upright, mimeType, nil
}

// Dimensions is the width and height of an image, read from its header.
func Dimensions(content []byte) (int, int, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return config.Width, config.Height, nil
}
