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
	"sort"
	"strings"
	"sync"

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
	verbose := flag.Bool("v", false, "name every file as it goes, and not only count them")
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

	m := &mover{client: client, bucket: bucket, apply: *apply, deleteOld: *deleteOld, verbose: *verbose}
	if err := m.run(ctx, prefix); err != nil {
		log.Fatalf("stopped: %v", err)
	}

	fmt.Println()
	if !*apply {
		fmt.Printf("%d files would be copied, %d are already in place, %d are not pictures this knows, %d gave way to another with the same place.\n", m.toCopy, m.inPlace, m.unknown, m.superseded)
		fmt.Println("Nothing was changed. Run again with -apply to copy them.")
		return
	}
	fmt.Printf("%d copied, %d already in place, %d failed, %d not pictures this knows, %d gave way to another with the same place.\n", m.copied, m.inPlace, m.failed, m.unknown, m.superseded)
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

// How many files are copied at once. The store does the copying; this is
// how many requests are in flight.
const workers = 24

type mover struct {
	client    *s3.Client
	bucket    string
	apply     bool
	deleteOld bool
	// Every file moved is named as it goes, and not only counted.
	verbose bool

	mu sync.Mutex
	// What is already in the v1 layout, by key, with its size: read once,
	// so that nothing has to be asked about each file.
	there map[string]int64
	// How many files of each folder there are to move, for the summary.
	folders map[string]int

	toCopy, inPlace, copied, failed, unknown, deleted, superseded int
}

type object struct {
	key  string
	size int64
}

// list reads every file under a prefix.
func (m *mover) list(ctx context.Context, prefix string, each func(object)) error {
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
		for _, entry := range page.Contents {
			each(object{key: aws.ToString(entry.Key), size: aws.ToInt64(entry.Size)})
		}
		if !aws.ToBool(page.IsTruncated) {
			return nil
		}
		token = page.NextContinuationToken
	}
}

func (m *mover) run(ctx context.Context, prefix string) error {
	// What is in place already, in one pass, and then what there is to move.
	m.there = map[string]int64{}
	m.folders = map[string]int{}
	if err := m.list(ctx, assetsPrefix, func(o object) { m.there[o.key] = o.size }); err != nil {
		return err
	}

	var old []object
	if err := m.list(ctx, prefix, func(o object) { old = append(old, o) }); err != nil {
		return err
	}
	fmt.Printf("%d files under %s, %d already under %s\n", len(old), prefix, len(m.there), assetsPrefix)

	// Two old files with one new place: only the one Prefer chooses is
	// copied, so that the choice is the same on every run, and not
	// whichever copy happened to land last.
	chosen := map[string]object{}
	var losers []object
	for _, o := range old {
		newKey, err := NewKey(o.key)
		if err != nil {
			continue
		}
		if prev, ok := chosen[newKey]; ok {
			if Prefer(o.key, prev.key) {
				chosen[newKey], o = o, prev
			}
			losers = append(losers, o)
			continue
		}
		chosen[newKey] = o
	}
	superseded := map[string]bool{}
	for _, o := range losers {
		superseded[o.key] = true
	}
	kept := old[:0:0]
	for _, o := range old {
		if !superseded[o.key] {
			kept = append(kept, o)
		}
	}
	old = kept

	queue := make(chan object)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for o := range queue {
				m.move(ctx, o.key, o.size)
			}
		}()
	}
	for i, o := range old {
		queue <- o
		if m.apply && (i+1)%2000 == 0 {
			fmt.Printf("  %d of %d\n", i+1, len(old))
		}
	}
	close(queue)
	wg.Wait()

	// What another file was chosen over: never copied, and deleted with the
	// old files only once the chosen one's copy is there, whole.
	for _, o := range losers {
		m.superseded++
		winner := chosen[mustNewKey(o.key)]
		if m.verbose || !m.apply {
			m.say("superseded %s\n        by %s\n", o.key, winner.key)
		}
		if !m.apply || !m.deleteOld {
			continue
		}
		if ok, err := m.sameSize(ctx, mustNewKey(o.key), winner.size); err != nil || !ok {
			m.failed++
			m.say("KEPT   %s: the copy it gave way to could not be confirmed\n", o.key)
			continue
		}
		if _, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(m.bucket), Key: aws.String(o.key)}); err != nil {
			m.failed++
			m.say("FAILED deleting %s: %v\n", o.key, err)
			continue
		}
		m.deleted++
	}

	if len(m.folders) > 0 {
		fmt.Println()
		fmt.Println("To move, by where it goes:")
		names := make([]string, 0, len(m.folders))
		for name := range m.folders {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("  %7d  %s\n", m.folders[name], name)
		}
	}
	return nil
}

func (m *mover) count(change func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	change()
}

func (m *mover) say(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fmt.Printf(format, args...)
}

// mustNewKey is NewKey for a key known to have one.
func mustNewKey(oldKey string) string {
	newKey, _ := NewKey(oldKey)
	return newKey
}

// folderOf is the part of a v1 key that says what kind of file it is.
func folderOf(newKey string) string {
	parts := strings.Split(strings.TrimPrefix(newKey, assetsPrefix), "/")
	if len(parts) >= 2 && parts[0] == "identity-image-service" {
		return assetsPrefix + parts[0] + "/" + parts[1] + "/"
	}
	return assetsPrefix + parts[0] + "/"
}

// move copies one file to its new place, unless it is there already, and
// deletes the old one when asked and the copy is certain.
func (m *mover) move(ctx context.Context, oldKey string, size int64) {
	newKey, err := NewKey(oldKey)
	if err != nil {
		// A folder marker, or something that is not one of the pictures.
		if !strings.HasSuffix(oldKey, "/") {
			m.count(func() { m.unknown++ })
			m.say("left   %s (%v)\n", oldKey, err)
		}
		return
	}

	m.mu.Lock()
	have, known := m.there[newKey]
	m.mu.Unlock()
	inPlace := known && have == size

	switch {
	case inPlace:
		m.count(func() { m.inPlace++ })
	case !m.apply:
		m.count(func() {
			m.toCopy++
			m.folders[folderOf(newKey)]++
		})
		if m.verbose {
			m.say("would copy %s\n        to %s\n", oldKey, newKey)
		}
		return
	default:
		// Copied by the store itself: the file does not pass through here.
		_, err := m.client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(m.bucket),
			CopySource: aws.String(m.bucket + "/" + escapeKey(oldKey)),
			Key:        aws.String(newKey),
		})
		if err != nil {
			m.count(func() { m.failed++ })
			m.say("FAILED %s: %v\n", oldKey, err)
			return
		}
		m.count(func() {
			m.copied++
			m.folders[folderOf(newKey)]++
		})
		if m.verbose {
			m.say("copied %s\n    to %s\n", oldKey, newKey)
		}
	}

	if !m.deleteOld {
		return
	}
	// Only once the copy is known to be there, whole. One that was in
	// place when the bucket was read is known already; one copied just now
	// is asked after.
	if !inPlace {
		if ok, err := m.sameSize(ctx, newKey, size); err != nil || !ok {
			m.count(func() { m.failed++ })
			m.say("KEPT   %s: its copy could not be confirmed\n", oldKey)
			return
		}
	}
	if _, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(oldKey),
	}); err != nil {
		m.count(func() { m.failed++ })
		m.say("FAILED deleting %s: %v\n", oldKey, err)
		return
	}
	m.count(func() { m.deleted++ })
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
