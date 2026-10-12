package handlers

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

type fakePictures struct {
	asked   []string
	content []byte
	err     error
}

func (f *fakePictures) Bytes(_ context.Context, address string, _ int64) ([]byte, string, error) {
	f.asked = append(f.asked, address)
	return f.content, "image/png", f.err
}

type fakeFiles struct {
	services.FilesGRPCInterface
	stored  map[string]int
	deleted []string
}

func (f *fakeFiles) Add(req *services.FileAddRequest) (*entities.File, error) {
	f.stored[req.Path] = len(req.Content)
	return &entities.File{}, nil
}

func (f *fakeFiles) Delete(req *services.FileDeleteRequest) error {
	delete(f.stored, req.Path)
	f.deleted = append(f.deleted, req.Path)
	return nil
}

func aPicture(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 600, 400))
	for x := 0; x < 600; x++ {
		for y := 0; y < 400; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// THE RULE THIS GUARDS: the picture of somebody on another server is shown
// from this installation's own storage, as a picture this installation made
// of what it was sent, and never from where it came from.
func TestTheirPictureIsCopiedHere(t *testing.T) {
	w := newInboxWorld()
	pictures := &fakePictures{content: aPicture(t)}
	files := &fakeFiles{stored: map[string]int{}}
	w.h.Pictures, w.h.Files = pictures, files

	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)

	alice, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == aliceID })
	if alice == nil || alice.AvatarFilename == "" {
		t.Fatalf("she has no picture: %+v", alice)
	}
	if strings.Join(pictures.asked, " ") != "https://mastodon.example/a.png https://mastodon.example/header.png" || alice.CoverFilename == "" {
		t.Errorf("asked %v, cover %q", pictures.asked, alice.CoverFilename)
	}

	// Every size an avatar has, under this installation's own folders, and
	// under her node.
	avatars, _ := alice.GetAvatars()
	covers, _ := alice.GetCovers()
	sizes := map[string]bool{}
	for _, size := range avatars {
		sizes[size.Url] = true
	}
	for _, size := range covers {
		sizes[size.Url] = true
	}
	if len(files.stored) != len(sizes) || len(avatars) == 0 || len(covers) == 0 {
		t.Fatalf("stored %d of %d sizes: %v", len(files.stored), len(sizes), files.stored)
	}
	for path := range files.stored {
		if !strings.HasPrefix(path, entities.IdentityImagesPrefix) || strings.Contains(path, "mastodon.example") {
			t.Errorf("stored at %q", path)
		}
	}
	first := alice.AvatarFilename

	// Looked up again with the same picture: not fetched again.
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	if len(pictures.asked) != 2 {
		t.Errorf("fetched again: %v", pictures.asked)
	}

	// Forgotten, her picture goes from storage with her.
	for _, kept := range w.actors.kept {
		if err := w.h.forget(kept); err != nil {
			t.Fatal(err)
		}
	}
	if len(files.stored) != 0 || len(files.deleted) != len(sizes) {
		t.Errorf("left in storage: %v (named %s)", files.stored, first)
	}
}

func TestAPictureThatIsNotOneIsNotKept(t *testing.T) {
	for name, pictures := range map[string]*fakePictures{
		"a page":            {content: []byte("<html><script>alert(1)</script></html>")},
		"a script":          {content: []byte("#!/bin/sh\nrm -rf /")},
		"nothing":           {content: nil},
		"could not be read": {err: errors.New("status 500")},
	} {
		w := newInboxWorld()
		files := &fakeFiles{stored: map[string]int{}}
		w.h.Pictures, w.h.Files = pictures, files

		rec := w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
		alice, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == aliceID })

		// She is found all the same, without a picture, and nothing is stored.
		if rec.Code != http.StatusOK || alice == nil || alice.AvatarFilename != "" || len(files.stored) != 0 {
			t.Errorf("%s: %d, avatar %q, stored %v", name, rec.Code, alice.AvatarFilename, files.stored)
		}
	}

	// A server that is refused is not asked for a picture.
	w := newInboxWorld()
	pictures := &fakePictures{content: aPicture(t)}
	w.h.Pictures, w.h.Files = pictures, &fakeFiles{stored: map[string]int{}}
	w.blocks.domains["mastodon.example"] = ""
	w.h.copyAvatar(1, "https://mastodon.example/a.png")
	if len(pictures.asked) != 0 {
		t.Errorf("asked %v", pictures.asked)
	}
}
