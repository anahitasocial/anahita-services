package services

import (
	"anahita-libs/proto/filepb"
	"bytes"
	"context"
	"errors"
	"file/config"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	// presignExpiry is how long a presigned URL can be opened for.
	presignExpiry = 15 * time.Minute

	// requestTimeout bounds one call to the store. Without it a store that
	// stopped answering held the gRPC call, and whoever was waiting on it,
	// open for as long as the connection lasted.
	requestTimeout = 30 * time.Second
)

var (
	ErrFileExists     = errors.New("file already exists")
	ErrFileMissing    = errors.New("file does not exist")
	ErrTypeNotAllowed = errors.New("this type of file is not stored")
)

// StorageInterface is the object store as the rest of this service sees it.
type StorageInterface interface {
	Read(path string) (*filepb.File, error)
	Edit(file *filepb.File) error
	Add(file *filepb.File) error
	Delete(path string) error
	Exists(path string) (bool, error)
	Presign(path string) (string, error)
	// PresignDownload is Presign for a file to keep: the address makes a
	// browser save it, under the name given, and not show it.
	PresignDownload(path, name string) (string, error)
	BatchPresign(paths []string) ([]string, error)

	// Large files, sent to the store directly. See uploads.go.
	UploadStart(path string) (string, error)
	UploadParts(path, uploadID string, numbers []int32) ([]string, error)
	UploadComplete(path, uploadID string, maxSize int64) (int64, error)
	UploadAbort(path, uploadID string) error
	Stat(path string) (exists bool, size int64, contentType string, err error)
	FetchURL(path string) (string, error)
	StoreURL(path, contentType string) (string, error)
}

// Storage talks to any S3-compatible store. See config.Storage for what each
// setting means.
//
// It holds two clients. `client` is for the work this service does itself,
// through the endpoint it can reach. `presigner` signs the URLs browsers
// open, for the endpoint THEY can reach, and `internal` the ones another
// service reads and writes large files at, for the endpoint services reach. A URL signed for one host is not
// valid on another, so the public endpoint cannot be swapped in afterwards.
type Storage struct {
	config    config.Storage
	client    *s3.Client
	presigner *s3.PresignClient
	internal  *s3.PresignClient
}

// NewStorage builds the clients once. The old code built a new client on
// every call, which re-resolved credentials each time.
func NewStorage(cfg *config.Config) (*Storage, error) {
	storage := cfg.Storage

	options := []func(*awsConfig.LoadOptions) error{
		awsConfig.WithRegion(storage.Region),
	}

	// Static keys when given. Otherwise the SDK's own chain, which is how a
	// role, a profile or an instance identity is picked up on AWS.
	if storage.AccessKey != "" {
		options = append(options, awsConfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(storage.AccessKey, storage.SecretKey, ""),
		))
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("storage: load configuration: %w", err)
	}

	clientFor := func(endpoint string) *s3.Client {
		return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			if endpoint != "" {
				o.BaseEndpoint = aws.String(endpoint)
			}
			o.UsePathStyle = storage.PathStyle
		})
	}

	return &Storage{
		config:    storage,
		client:    clientFor(storage.Endpoint),
		presigner: s3.NewPresignClient(clientFor(storage.GetPublicEndpoint())),
		internal:  s3.NewPresignClient(clientFor(storage.Endpoint)),
	}, nil
}

func requestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}

// Read reads a file from the store.
func (s *Storage) Read(path string) (*filepb.File, error) {
	ctx, cancel := requestContext()
	defer cancel()

	buffer := manager.NewWriteAtBuffer([]byte{})

	_, err := manager.NewDownloader(s.client).Download(ctx, buffer, &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, err
	}

	content := buffer.Bytes()

	return &filepb.File{
		Content:  content,
		Size:     int64(len(content)),
		Path:     path,
		Mimetype: "application/octet-stream",
	}, nil
}

// Edit replaces a file that is already there.
func (s *Storage) Edit(file *filepb.File) error {
	exists, err := s.Exists(file.Path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}
	if !exists {
		return ErrFileMissing
	}

	return s.put(file)
}

// Add stores a new file, and refuses to overwrite one.
func (s *Storage) Add(file *filepb.File) error {
	exists, err := s.Exists(file.Path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}
	if exists {
		return ErrFileExists
	}

	return s.put(file)
}

// put uploads a file after checking what it actually is.
func (s *Storage) put(file *filepb.File) error {
	contentType, err := s.admit(file.Content)
	if err != nil {
		return err
	}

	ctx, cancel := requestContext()
	defer cancel()

	size := int64(len(file.Content))
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.config.Bucket),
		Key:           aws.String(file.Path),
		Body:          bytes.NewReader(file.Content),
		ContentType:   aws.String(contentType),
		ContentLength: &size,
	}

	// Each header is optional because a store that does not implement one
	// refuses the upload rather than ignoring it.
	if s.config.SendACL {
		input.ACL = s3types.ObjectCannedACLPrivate
	}
	if s.config.SSE == config.SSEAES256 {
		input.ServerSideEncryption = s3types.ServerSideEncryptionAes256
	}

	_, err = s.client.PutObject(ctx, input)
	return err
}

// admit decides whether content may be stored, and under what content type.
//
// The type comes from the bytes, never from the caller. An object store does
// not execute what it holds, but it does serve it back with whatever content
// type it was given, and a browser acts on that: an HTML or SVG file stored
// as "image/png" by a careless caller, or as text/html by a hostile one, runs
// script on whichever origin serves it. Only what is on the list is stored,
// and it is stored as what it is.
func (s *Storage) admit(content []byte) (string, error) {
	// DetectContentType may append parameters ("text/plain; charset=utf-8").
	detected, _, _ := strings.Cut(http.DetectContentType(content), ";")
	detected = strings.TrimSpace(detected)

	if !slices.Contains(s.config.AllowedTypes, detected) {
		return "", fmt.Errorf("%w: %s", ErrTypeNotAllowed, detected)
	}

	return detected, nil
}

// Delete removes a file. Deleting one that is not there is not an error.
//
// It used to wait, for up to five minutes, until the store confirmed the
// object was gone. S3 and the stores compatible with it answer a delete only
// once it has happened, so the wait added a second round trip to every
// delete and, against a store whose HEAD behaves differently, a five-minute
// hang.
func (s *Storage) Delete(path string) error {
	ctx, cancel := requestContext()
	defer cancel()

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	})
	return err
}

// Exists reports whether a file is in the store.
//
// "Not there" and "could not find out" are different answers. This used to
// return false for both, so with the store unreachable or the credentials
// wrong, Add saw "does not exist" and went on to fail later with a less
// useful error, and Edit reported a missing file that was not missing.
func (s *Storage) Exists(path string) (bool, error) {
	ctx, cancel := requestContext()
	defer cancel()

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	})
	if err == nil {
		return true, nil
	}

	if isNotFound(err) {
		return false, nil
	}

	return false, err
}

// isNotFound recognises a missing object. A HEAD response has no body, so the
// SDK cannot always build its typed NotFound error and stores differ in the
// code they send; the status code is the part every one of them agrees on.
func isNotFound(err error) bool {
	var notFound *s3types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var noSuchKey *s3types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return true
		}
	}

	var responseErr interface{ HTTPStatusCode() int }
	if errors.As(err, &responseErr) {
		return responseErr.HTTPStatusCode() == http.StatusNotFound
	}

	return false
}

// Presign returns a URL a browser can open for a short while.
func (s *Storage) Presign(path string) (string, error) {
	return s.presign(path, "")
}

// PresignDownload returns a URL that makes a browser save the file, under
// this name, and not show it.
func (s *Storage) PresignDownload(path, name string) (string, error) {
	name = DownloadName(name)
	if name == "" {
		return "", ErrPathNotAllowed
	}
	return s.presign(path, name)
}

// DownloadName makes a name safe to put in a header: letters, digits, dots,
// dashes and underscores, and nothing else. The caller builds it from a
// post's title, which is somebody's text.
func DownloadName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	clean := strings.Trim(b.String(), ".")
	if len(clean) > 120 {
		clean = clean[len(clean)-120:]
	}
	return clean
}

func (s *Storage) presign(path, downloadName string) (string, error) {
	ctx, cancel := requestContext()
	defer cancel()

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	}
	// Large files did not pass through admit; see uploads.go.
	if err := readerRules(path, input); err != nil {
		return "", err
	}
	// The store sends this header back with the file, because the address
	// was signed asking it to.
	if downloadName != "" {
		input.ResponseContentDisposition = aws.String(`attachment; filename="` + downloadName + `"`)
	}

	req, err := s.presigner.PresignGetObject(ctx, input, s3.WithPresignExpires(presignExpiry))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

// BatchPresign presigns many paths, keeping their order. Signing is local
// arithmetic with no request to the store, so the goroutines only spread it
// over the available cores.
func (s *Storage) BatchPresign(paths []string) ([]string, error) {
	results := make([]string, len(paths))
	errs := make(chan error, len(paths))

	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()

			url, err := s.Presign(path)
			if err != nil {
				errs <- err
				return
			}
			results[i] = url
		}(i, path)
	}

	wg.Wait()
	close(errs)

	if err := <-errs; err != nil {
		return nil, err
	}

	return results, nil
}
