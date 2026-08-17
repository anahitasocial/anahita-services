package images

import (
	"bytes"
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
		err = jpeg.Encode(&buf, resizedImage, nil)
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
		err = jpeg.Encode(&output, img, nil)
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
