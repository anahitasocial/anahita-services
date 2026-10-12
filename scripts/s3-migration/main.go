// Moves a bucket's pictures to the v1 layout: from the PHP application's
// (assets/public, "v0") or from the one this platform used while it was
// being built (assets/v2). See README.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// firstEnv returns the first of the named variables that is set.
func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func main() {
	from := flag.String("from", "", `the layout the bucket is in now: "v0" (the PHP application's, assets/public) or "v2" (assets/v2)`)
	apply := flag.Bool("apply", false, "copy the files. Without it, only what would be copied is listed")
	deleteOld := flag.Bool("delete-old", false, "delete each old file whose copy is in place. Only with -apply, and only after checking the site")
	flag.Parse()

	prefix, ok := sources[*from]
	if !ok {
		log.Fatal(`say which layout the bucket is in now: -from v0 or -from v2`)
	}
	if *deleteOld && !*apply {
		log.Fatal("-delete-old changes the bucket, and so needs -apply as well")
	}

	// The same settings file-grpc-service reads, so the script follows the
	// installation to whichever S3-compatible store it uses.
	bucket := firstEnv("STORAGE_BUCKET", "AWS_FILES_S3_BUCKET")
	if bucket == "" {
		log.Fatal("set STORAGE_BUCKET to the bucket whose files should be moved")
	}

	ctx := context.Background()

	var options []func(*config.LoadOptions) error
	if region := firstEnv("STORAGE_REGION", "AWS_REGION", "AWS_DEFAULT_REGION"); region != "" {
		options = append(options, config.WithRegion(region))
	}
	if key, secret := os.Getenv("STORAGE_ACCESS_KEY"), os.Getenv("STORAGE_SECRET_KEY"); key != "" && secret != "" {
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")))
	}

	cfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint := os.Getenv("STORAGE_ENDPOINT"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = os.Getenv("STORAGE_PATH_STYLE") == "true"
	})

	m := &mover{client: client, bucket: bucket, apply: *apply, deleteOld: *deleteOld}
	if err := m.run(ctx, prefix); err != nil {
		log.Fatalf("stopped: %v", err)
	}

	fmt.Println()
	if !*apply {
		fmt.Printf("%d files would be copied, %d are already in place, %d are not pictures this knows.\n", m.toCopy, m.inPlace, m.unknown)
		fmt.Println("Nothing was changed. Run again with -apply to copy them.")
		return
	}
	fmt.Printf("%d copied, %d already in place, %d failed, %d not pictures this knows.\n", m.copied, m.inPlace, m.failed, m.unknown)
	if *deleteOld {
		fmt.Printf("%d old files deleted.\n", m.deleted)
	} else {
		fmt.Println("The old files are still there. When the site shows its pictures from the new places,")
		fmt.Println("run again with -apply -delete-old to remove them.")
	}
	if m.failed > 0 {
		os.Exit(1)
	}
}

// Where each layout keeps its files.
var sources = map[string]string{
	"v0": "assets/public/n",
	"v2": "assets/v2/",
}

type mover struct {
	client    *s3.Client
	bucket    string
	apply     bool
	deleteOld bool

	toCopy, inPlace, copied, failed, unknown, deleted int
}

func (m *mover) run(ctx context.Context, prefix string) error {
	var token *string

	for {
		page, err := m.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(m.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return fmt.Errorf("listing %s: %w", prefix, err)
		}

		for _, object := range page.Contents {
			m.move(ctx, aws.ToString(object.Key), aws.ToInt64(object.Size))
		}

		if !aws.ToBool(page.IsTruncated) {
			return nil
		}
		token = page.NextContinuationToken
	}
}

// move copies one file to its new place, unless it is there already, and
// deletes the old one when asked and the copy is certain.
func (m *mover) move(ctx context.Context, oldKey string, size int64) {
	newKey, err := NewKey(oldKey)
	if err != nil {
		// A folder marker, or something that is not one of the pictures.
		if !strings.HasSuffix(oldKey, "/") {
			m.unknown++
			fmt.Printf("left   %s (%v)\n", oldKey, err)
		}
		return
	}

	there, err := m.sameSize(ctx, newKey, size)
	if err != nil {
		m.failed++
		fmt.Printf("FAILED %s: %v\n", oldKey, err)
		return
	}

	switch {
	case there:
		m.inPlace++
	case !m.apply:
		m.toCopy++
		fmt.Printf("would copy %s\n        to %s\n", oldKey, newKey)
		return
	default:
		// Copied by the store itself: the file does not pass through here.
		_, err := m.client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(m.bucket),
			CopySource: aws.String(m.bucket + "/" + escapeKey(oldKey)),
			Key:        aws.String(newKey),
		})
		if err != nil {
			m.failed++
			fmt.Printf("FAILED %s: %v\n", oldKey, err)
			return
		}
		m.copied++
		fmt.Printf("copied %s\n    to %s\n", oldKey, newKey)
	}

	if !m.deleteOld {
		return
	}
	// Only once the copy is known to be there, whole.
	if ok, err := m.sameSize(ctx, newKey, size); err != nil || !ok {
		m.failed++
		fmt.Printf("KEPT   %s: its copy could not be confirmed\n", oldKey)
		return
	}
	if _, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(oldKey),
	}); err != nil {
		m.failed++
		fmt.Printf("FAILED deleting %s: %v\n", oldKey, err)
		return
	}
	m.deleted++
}

// sameSize says whether a file is at a key and is this many bytes.
func (m *mover) sameSize(ctx context.Context, key string, size int64) (bool, error) {
	head, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var responseErr interface{ HTTPStatusCode() int }
		if errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return aws.ToInt64(head.ContentLength) == size, nil
}

// escapeKey writes a key as a copy source wants it: each part escaped, the
// slashes between them left.
func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		// A plus is read as a space in a copy source unless it is written out.
		parts[i] = strings.ReplaceAll(url.PathEscape(part), "+", "%2B")
	}
	return strings.Join(parts, "/")
}
