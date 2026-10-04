package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"anahita-libs/proto/filepb"
	"file/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Behaviour against a real S3-compatible store.
//
// Skipped unless STORAGE_TEST_ENDPOINT is set. The store is whatever the
// STORAGE_TEST_* variables point at, so the same test checks Garage, MinIO or
// a provider's bucket:
//
//	STORAGE_TEST_ENDPOINT         http://127.0.0.1:3900
//	STORAGE_TEST_PUBLIC_ENDPOINT  optional; defaults to the endpoint
//	STORAGE_TEST_REGION           garage
//	STORAGE_TEST_BUCKET           anahita-test
//	STORAGE_TEST_ACCESS_KEY, STORAGE_TEST_SECRET_KEY
//	STORAGE_TEST_PATH_STYLE       true for self-hosted stores
//	STORAGE_TEST_SSE              none (default) or AES256
//	STORAGE_TEST_SEND_ACL         false (default) or true
//
// docs/getting-started.md shows how to start the local store.
func integrationConfig(t *testing.T) *config.Config {
	t.Helper()

	endpoint := os.Getenv("STORAGE_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set STORAGE_TEST_ENDPOINT to run storage tests against a real store")
	}

	env := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	return &config.Config{Storage: config.Storage{
		Endpoint:       endpoint,
		PublicEndpoint: os.Getenv("STORAGE_TEST_PUBLIC_ENDPOINT"),
		Region:         env("STORAGE_TEST_REGION", "us-east-1"),
		Bucket:         env("STORAGE_TEST_BUCKET", "anahita-test"),
		AccessKey:      os.Getenv("STORAGE_TEST_ACCESS_KEY"),
		SecretKey:      os.Getenv("STORAGE_TEST_SECRET_KEY"),
		PathStyle:      env("STORAGE_TEST_PATH_STYLE", "true") == "true",
		SSE:            env("STORAGE_TEST_SSE", config.SSENone),
		SendACL:        env("STORAGE_TEST_SEND_ACL", "false") == "true",
		AllowedTypes:   config.DefaultAllowedTypes,
	}}
}

func integrationStorage(t *testing.T) (*Storage, *config.Config) {
	t.Helper()
	cfg := integrationConfig(t)

	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	// Create the bucket when the store lets a client do that (MinIO does;
	// Garage wants it made by its own tool, and then this just fails quietly).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	storage.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(cfg.Storage.Bucket)})

	return storage, cfg
}

func TestStorageRoundTripAgainstARealStore(t *testing.T) {
	storage, cfg := integrationStorage(t)

	path := "test/" + time.Now().Format("20060102-150405.000000000") + "/avatar.png"
	content := pngBytes(t)
	t.Cleanup(func() { storage.Delete(path) })

	t.Run("a file that is not there does not exist", func(t *testing.T) {
		exists, err := storage.Exists(path)
		if err != nil || exists {
			t.Fatalf("exists = %v, err = %v; want false and no error", exists, err)
		}
	})

	t.Run("editing a file that is not there is refused", func(t *testing.T) {
		err := storage.Edit(&filepb.File{Path: path, Content: content})
		if !errors.Is(err, ErrFileMissing) {
			t.Errorf("err = %v, want ErrFileMissing", err)
		}
	})

	t.Run("a script is refused and nothing is stored", func(t *testing.T) {
		err := storage.Add(&filepb.File{Path: path, Content: []byte("<html><script>alert(1)</script></html>"), Mimetype: "image/png"})
		if !errors.Is(err, ErrTypeNotAllowed) {
			t.Fatalf("err = %v, want ErrTypeNotAllowed", err)
		}
		if exists, _ := storage.Exists(path); exists {
			t.Error("the refused file was stored anyway")
		}
	})

	t.Run("an image is stored", func(t *testing.T) {
		// The declared type is wrong on purpose: it must not be believed.
		if err := storage.Add(&filepb.File{Path: path, Content: content, Mimetype: "text/html"}); err != nil {
			t.Fatalf("add: %v", err)
		}
		exists, err := storage.Exists(path)
		if err != nil || !exists {
			t.Fatalf("exists = %v, err = %v; want true", exists, err)
		}
	})

	t.Run("adding it again is refused", func(t *testing.T) {
		if err := storage.Add(&filepb.File{Path: path, Content: content}); !errors.Is(err, ErrFileExists) {
			t.Errorf("err = %v, want ErrFileExists", err)
		}
	})

	t.Run("it reads back byte for byte", func(t *testing.T) {
		file, err := storage.Read(path)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(file.Content) != string(content) || file.Size != int64(len(content)) {
			t.Errorf("read %d bytes, want the %d that were stored", len(file.Content), len(content))
		}
	})

	t.Run("a presigned URL opens it, at the public endpoint, as an image", func(t *testing.T) {
		url, err := storage.Presign(path)
		if err != nil {
			t.Fatalf("presign: %v", err)
		}
		if want := strings.TrimRight(cfg.Storage.GetPublicEndpoint(), "/"); !strings.HasPrefix(url, want) {
			t.Fatalf("url %q is not at the public endpoint %q", url, want)
		}

		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, body %s", resp.StatusCode, body)
		}
		if string(body) != string(content) {
			t.Error("the presigned URL returned different bytes")
		}
		// The type sniffed from the bytes, not the text/html it was declared as.
		if got := resp.Header.Get("Content-Type"); got != "image/png" {
			t.Errorf("content type = %q, want image/png", got)
		}
	})

	t.Run("the URL is useless without its signature", func(t *testing.T) {
		url, _ := storage.Presign(path)
		bare, _, _ := strings.Cut(url, "?")

		resp, err := http.Get(bare)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Error("the object is readable without a signature: the bucket is public")
		}
	})

	t.Run("a batch keeps its order", func(t *testing.T) {
		paths := []string{path, "test/other.png", path}
		urls, err := storage.BatchPresign(paths)
		if err != nil || len(urls) != 3 {
			t.Fatalf("urls = %d, err = %v", len(urls), err)
		}
		if !strings.Contains(urls[0], "avatar.png") || !strings.Contains(urls[1], "other.png") || !strings.Contains(urls[2], "avatar.png") {
			t.Errorf("order was not kept: %v", urls)
		}
	})

	t.Run("it can be replaced", func(t *testing.T) {
		replacement := jpegBytes(t)
		if err := storage.Edit(&filepb.File{Path: path, Content: replacement}); err != nil {
			t.Fatalf("edit: %v", err)
		}
		file, err := storage.Read(path)
		if err != nil || string(file.Content) != string(replacement) {
			t.Errorf("after edit: err = %v, %d bytes", err, len(file.Content))
		}
	})

	t.Run("it can be deleted, twice", func(t *testing.T) {
		if err := storage.Delete(path); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if exists, err := storage.Exists(path); err != nil || exists {
			t.Errorf("after delete: exists = %v, err = %v", exists, err)
		}
		if err := storage.Delete(path); err != nil {
			t.Errorf("deleting a file that is not there: %v", err)
		}
	})
}

// "Could not find out" must not read as "not there". With wrong credentials
// Exists used to answer false, and Add then went on to fail somewhere less
// clear.
func TestExistsReportsAFailureAsAFailure(t *testing.T) {
	cfg := integrationConfig(t)
	cfg.Storage.AccessKey, cfg.Storage.SecretKey = "not-a-real-key", "not-a-real-secret"

	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	exists, err := storage.Exists("test/anything.png")
	if err == nil {
		t.Errorf("exists = %v with no error, using credentials the store does not know", exists)
	}
}
