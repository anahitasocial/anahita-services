package services

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"file/config"
)

func TestWhereLargeFilesMayGo(t *testing.T) {
	for path, want := range map[string]bool{
		"uploads/5/abc/source":       true,
		"uploads/":                   false,
		"uploads":                    false,
		"avatars/5/x.jpg":            false,
		"uploads/../avatars/5/x.jpg": false,
		"uploads//x":                 false,
		"uploads/a\\b":               false,
		"/uploads/x":                 false,
		"videos/uploads/x":           false,
		"":                           false,
	} {
		if got := under(path, UploadsPrefix); got != want {
			t.Errorf("under(%q) = %v, want %v", path, got, want)
		}
	}

	// Refused before the store is asked anything: a Storage with no client
	// would panic if it got that far.
	s := &Storage{config: config.Storage{Bucket: "b"}}
	if _, err := s.UploadStart("avatars/5/x.jpg"); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("an upload outside uploads/: %v", err)
	}
	if _, err := s.UploadParts("photos/x", "id", []int32{1}); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("parts outside uploads/: %v", err)
	}
	if _, err := s.UploadComplete("photos/x", "id", 0); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("completing outside uploads/: %v", err)
	}
	if _, err := s.StoreURL("uploads/5/x", "video/mp4"); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("a worker's file outside videos/: %v", err)
	}
	if _, err := s.StoreURL("videos/5/x.html", "text/html"); !errors.Is(err, ErrTypeNotAllowed) {
		t.Errorf("a worker's file that is a page: %v", err)
	}
	if _, err := s.FetchURL("avatars/5/x.jpg"); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("reading what is not a large file: %v", err)
	}
	for _, numbers := range [][]int32{nil, {0}, {10001}, make([]int32, 101)} {
		if _, err := s.UploadParts("uploads/x", "id", numbers); !errors.Is(err, ErrBadPart) {
			t.Errorf("parts %v: %v", numbers, err)
		}
	}
}

func put(t *testing.T, url string, body []byte, contentType string) int {
	t.Helper()

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

func TestALargeFileSentInPartsAgainstARealStore(t *testing.T) {
	storage, _ := integrationStorage(t)

	stamp := time.Now().Format("20060102-150405.000000000")
	path := UploadsPrefix + "test/" + stamp + "/source"
	t.Cleanup(func() { storage.Delete(path) })

	// Two parts: every part but the last has to be at least 5 MB.
	first := bytes.Repeat([]byte("a"), 5<<20)
	second := []byte("the end")

	id, err := storage.UploadStart(path)
	if err != nil || id == "" {
		t.Fatalf("start: %q %v", id, err)
	}

	// Nothing sent yet: there is nothing to make a file of.
	if _, err := storage.UploadComplete(path, id, 0); !errors.Is(err, ErrNothingSent) {
		t.Fatalf("completing an empty upload: %v", err)
	}

	urls, err := storage.UploadParts(path, id, []int32{1, 2})
	if err != nil || len(urls) != 2 {
		t.Fatalf("parts: %v %v", urls, err)
	}
	for i, body := range [][]byte{first, second} {
		if status := put(t, urls[i], body, ""); status != http.StatusOK {
			t.Fatalf("part %d answered %d", i+1, status)
		}
	}

	size, err := storage.UploadComplete(path, id, 10<<20)
	if err != nil || size != int64(len(first)+len(second)) {
		t.Fatalf("complete: %d %v", size, err)
	}

	exists, stored, contentType, err := storage.Stat(path)
	if err != nil || !exists || stored != size {
		t.Fatalf("stat: %v %d %v", exists, stored, err)
	}
	// Whatever it is, it is kept as nothing a browser would act on.
	if contentType != rawContentType {
		t.Errorf("kept as %q", contentType)
	}

	// Another service reads it at an address signed for it.
	address, err := storage.FetchURL(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	read, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || int64(len(read)) != size {
		t.Errorf("read back %d bytes, status %d", len(read), res.StatusCode)
	}
}

func TestAFileLargerThanAllowedIsThrownAway(t *testing.T) {
	storage, _ := integrationStorage(t)

	path := UploadsPrefix + "test/" + time.Now().Format("20060102-150405.000000000") + "/big"
	t.Cleanup(func() { storage.Delete(path) })

	id, err := storage.UploadStart(path)
	if err != nil {
		t.Fatal(err)
	}
	urls, err := storage.UploadParts(path, id, []int32{1})
	if err != nil {
		t.Fatal(err)
	}
	put(t, urls[0], bytes.Repeat([]byte("a"), 2048), "")

	if _, err := storage.UploadComplete(path, id, 1024); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("a file past the limit: %v", err)
	}
	if exists, _, _, _ := storage.Stat(path); exists {
		t.Error("it was kept anyway")
	}
	// Thrown away with its parts: there is nothing left to complete.
	if _, err := storage.UploadComplete(path, id, 0); err == nil {
		t.Error("the upload could still be completed")
	}
	// Throwing it away twice is not an error.
	if err := storage.UploadAbort(path, id); err != nil {
		t.Errorf("aborting what is gone: %v", err)
	}
}

func TestAWorkerWritesItsOwnFiles(t *testing.T) {
	storage, _ := integrationStorage(t)

	path := WorkerPrefix + "test/" + time.Now().Format("20060102-150405.000000000") + "/720.mp4"
	t.Cleanup(func() { storage.Delete(path) })

	address, err := storage.StoreURL(path, "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	// Written as a page, which not every store refuses.
	if status := put(t, address, []byte("<html>"), "text/html"); status != http.StatusOK {
		t.Fatalf("the write answered %d", status)
	}
	if exists, size, _, _ := storage.Stat(path); !exists || size != 6 {
		t.Errorf("kept: %v %d", exists, size)
	}

	// A reader is sent it as a video all the same.
	reader, err := storage.Presign(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(reader)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if got := res.Header.Get("Content-Type"); res.StatusCode != http.StatusOK || got != "video/mp4" {
		t.Errorf("a reader was sent it as %q, status %d", got, res.StatusCode)
	}

	// And is never given an upload.
	if _, err := storage.Presign(UploadsPrefix + "5/abc/source"); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("an upload was signed for a reader: %v", err)
	}
}
