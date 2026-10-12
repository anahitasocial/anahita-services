package main

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Against a real S3-compatible store, in a bucket for tests. Skipped unless
// STORAGE_TEST_ENDPOINT is set; the other STORAGE_TEST_* variables are the
// ones file-grpc-service's storage tests use.
//
// THE RULE THIS GUARDS: nothing is changed without -apply, nothing is
// deleted without -delete-old, and an old file is deleted only once its
// copy is in place, whole.
func testMover(t *testing.T) *mover {
	t.Helper()

	endpoint := os.Getenv("STORAGE_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set STORAGE_TEST_ENDPOINT to run against a real store")
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(os.Getenv("STORAGE_TEST_REGION")),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("STORAGE_TEST_ACCESS_KEY"), os.Getenv("STORAGE_TEST_SECRET_KEY"), "")))
	if err != nil {
		t.Fatal(err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	return &mover{client: client, bucket: os.Getenv("STORAGE_TEST_BUCKET")}
}

func (m *mover) has(t *testing.T, key string) bool {
	t.Helper()
	_, err := m.client.HeadObject(context.Background(), &s3.HeadObjectInput{Bucket: aws.String(m.bucket), Key: aws.String(key)})
	return err == nil
}

func TestABucketIsMovedInThreeSteps(t *testing.T) {
	m := testMover(t)
	ctx := context.Background()

	// A person nobody else's test uses, so the listing is this test's own.
	who := time.Now().Format("150405000000")
	old := map[string]string{
		"assets/v2/avatar-service/person/" + who + "/a b+c_large.jpg": "an avatar",
		"assets/v2/cover-service/person/" + who + "/cover_large.jpg":  "a cover",
		"assets/v2/photo-service/photo/" + who + "/p_original.jpg":    "a photo, whole",
	}
	moved := map[string]string{}
	for key, body := range old {
		if _, err := m.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(m.bucket), Key: aws.String(key), Body: bytes.NewReader([]byte(body)),
		}); err != nil {
			t.Fatalf("seeding %s: %v", key, err)
		}
		newKey, err := NewKey(key)
		if err != nil {
			t.Fatal(err)
		}
		moved[key] = newKey
	}
	t.Cleanup(func() {
		for key, newKey := range moved {
			for _, each := range []string{key, newKey} {
				_, _ = m.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(m.bucket), Key: aws.String(each)})
			}
		}
	})

	step := func(apply, deleteOld bool) *mover {
		run := &mover{client: m.client, bucket: m.bucket, apply: apply, deleteOld: deleteOld}
		// The whole of it, as the program runs it: what is in place is read
		// first, then this test's own files are moved.
		if err := run.run(ctx, "assets/v2/avatar-service/person/"+who+"/"); err != nil {
			t.Fatal(err)
		}
		for _, prefix := range []string{"assets/v2/cover-service/person/" + who + "/", "assets/v2/photo-service/photo/" + who + "/"} {
			if err := run.list(ctx, prefix, func(o object) { run.move(ctx, o.key, o.size) }); err != nil {
				t.Fatal(err)
			}
		}
		return run
	}

	// 1. Without -apply: listed, and nothing changed.
	dry := step(false, false)
	if dry.toCopy != 3 || dry.copied != 0 {
		t.Fatalf("the dry run: %+v", dry)
	}
	for key, newKey := range moved {
		if !m.has(t, key) || m.has(t, newKey) {
			t.Fatalf("the dry run changed the bucket at %s", key)
		}
	}

	// 2. With -apply: copied, the old ones still there.
	copied := step(true, false)
	if copied.copied != 3 || copied.failed != 0 || copied.deleted != 0 {
		t.Fatalf("the copy: %+v", copied)
	}
	for key, newKey := range moved {
		if !m.has(t, key) || !m.has(t, newKey) {
			t.Fatalf("after the copy, %s or its copy is missing", key)
		}
	}
	// Run again: nothing to do.
	if again := step(true, false); again.copied != 0 || again.inPlace != 3 {
		t.Errorf("a second copy: %+v", again)
	}

	// 3. With -delete-old: the old ones go, the new ones stay.
	removed := step(true, true)
	if removed.deleted != 3 || removed.failed != 0 {
		t.Fatalf("the removal: %+v", removed)
	}
	for key, newKey := range moved {
		if m.has(t, key) || !m.has(t, newKey) {
			t.Errorf("after the removal, %s is still there or its copy is gone", key)
		}
	}
}
