package services

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"file/config"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatalf("jpeg: %v", err)
	}
	return buf.Bytes()
}

// What is stored is decided by the bytes. An object store serves back
// whatever content type it was given and a browser acts on it, so a script
// stored under an image's name is a script on whichever origin serves it.
func TestOnlyWhatTheBytesSayIsAnAllowedTypeIsStored(t *testing.T) {
	storage := &Storage{config: config.Storage{AllowedTypes: config.DefaultAllowedTypes}}

	accepted := []struct {
		what    string
		content []byte
		want    string
	}{
		{"a PNG", pngBytes(t), "image/png"},
		{"a JPEG", jpegBytes(t), "image/jpeg"},
	}
	for _, c := range accepted {
		t.Run(c.what+" is stored as what it is", func(t *testing.T) {
			got, err := storage.admit(c.content)
			if err != nil || got != c.want {
				t.Errorf("type = %q, err = %v; want %q", got, err, c.want)
			}
		})
	}

	refused := []struct {
		what    string
		content []byte
	}{
		{"an HTML page", []byte("<!DOCTYPE html><html><body><script>alert(1)</script></body></html>")},
		{"an SVG with a script", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)},
		{"plain text", []byte("just some words")},
		{"a shell script", []byte("#!/bin/sh\nrm -rf /\n")},
		{"a zip archive", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")},
		{"a GIF, which is an image but not on the list", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")},
		{"a Windows executable", []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff")},
		{"nothing at all", nil},
	}
	for _, c := range refused {
		t.Run(c.what+" is refused", func(t *testing.T) {
			if got, err := storage.admit(c.content); !errors.Is(err, ErrTypeNotAllowed) {
				t.Errorf("type = %q, err = %v; want ErrTypeNotAllowed", got, err)
			}
		})
	}
}

// The list is configuration, so a later service (video) can extend it.
func TestTheAllowedTypesAreConfigurable(t *testing.T) {
	gif := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")

	storage := &Storage{config: config.Storage{AllowedTypes: []string{"image/gif"}}}
	if got, err := storage.admit(gif); err != nil || got != "image/gif" {
		t.Errorf("a GIF with image/gif allowed: type = %q, err = %v", got, err)
	}
	if _, err := storage.admit(pngBytes(t)); !errors.Is(err, ErrTypeNotAllowed) {
		t.Errorf("a PNG with only image/gif allowed: err = %v", err)
	}
}
