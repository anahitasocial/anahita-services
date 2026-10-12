package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"file/config"
)

// Large files: a video somebody uploads, and what a worker makes of it.
//
// Everything else this service stores passes through it, a few megabytes
// at a time, and is checked for what it really is on the way (see admit).
// A video cannot: it is too big for a request to the API and for a gRPC
// message. So the browser sends it straight to the store, in parts, at
// addresses signed here, and the worker reads it and writes its renditions
// at addresses signed here too.
//
// What keeps that safe is where things may go and what they are stored as:
//
//   - A browser's upload goes under uploads/ and nowhere else, and is
//     stored as application/octet-stream whatever it is. Nothing under
//     uploads/ is ever given to a reader: it is raw material for a worker.
//   - A worker's files go under videos/, as video/mp4 or image/jpeg, which
//     it made itself by encoding. Those are what readers are sent, and they
//     are sent as what their name says they are, whatever they were stored
//     as: the type is part of the address a reader is given (readerRules).

const (
	// UploadsPrefix is where a browser's upload is put, and only there.
	UploadsPrefix = "uploads/"
	// WorkerPrefix is where a worker's files are put.
	WorkerPrefix = "videos/"

	// How long a part's address can be written to. A part of a large file
	// on a slow connection takes a while.
	partExpiry = time.Hour
	// How long a worker has to read or write a file at an address.
	transferExpiry = 2 * time.Hour

	// The most parts asked for at once, and there can be in a file.
	maxPartsAtOnce = 100
	maxPartNumber  = 10000

	rawContentType = "application/octet-stream"
)

// WorkerTypes are what a worker may store: what it encodes.
var WorkerTypes = []string{"video/mp4", "image/jpeg"}

var (
	ErrPathNotAllowed = errors.New("a file of this kind is not put there")
	ErrTooLarge       = errors.New("the file is larger than is allowed")
	ErrNothingSent    = errors.New("no part of the file was sent")
	ErrBadPart        = errors.New("there is no such part")
)

// readerRules is what an address given to a reader is held to, for the
// files that were never checked on their way in.
//
// An upload is not given to a reader at all. A worker's file is sent as
// the type its name ends in and no other: a store sends a file back as
// whatever it was stored as, and not every store holds a write to the type
// its address was signed for, so the type is fixed on the way out instead.
func readerRules(path string, input *s3.GetObjectInput) error {
	if strings.HasPrefix(path, UploadsPrefix) {
		return ErrPathNotAllowed
	}
	if !strings.HasPrefix(path, WorkerPrefix) {
		return nil
	}

	switch {
	case strings.HasSuffix(path, ".mp4"):
		input.ResponseContentType = aws.String("video/mp4")
	case strings.HasSuffix(path, ".jpg"):
		input.ResponseContentType = aws.String("image/jpeg")
	default:
		input.ResponseContentType = aws.String(rawContentType)
		input.ResponseContentDisposition = aws.String("attachment")
	}
	return nil
}

// under says whether a path is inside a prefix, and nowhere it could climb
// out of it from.
func under(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) || len(path) == len(prefix) {
		return false
	}
	if strings.Contains(path, "..") || strings.Contains(path, "//") || strings.ContainsAny(path, "\\\x00") {
		return false
	}
	return true
}

// UploadStart begins a file a browser will send in parts.
func (s *Storage) UploadStart(path string) (string, error) {
	if !under(path, UploadsPrefix) {
		return "", ErrPathNotAllowed
	}

	ctx, cancel := requestContext()
	defer cancel()

	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.config.Bucket),
		Key:         aws.String(path),
		ContentType: aws.String(rawContentType),
	}
	if s.config.SendACL {
		input.ACL = s3types.ObjectCannedACLPrivate
	}
	if s.config.SSE == config.SSEAES256 {
		input.ServerSideEncryption = s3types.ServerSideEncryptionAes256
	}

	out, err := s.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", err
	}

	return aws.ToString(out.UploadId), nil
}

// UploadParts signs the addresses a browser writes parts to, for the
// endpoint a browser can reach.
func (s *Storage) UploadParts(path, uploadID string, numbers []int32) ([]string, error) {
	if !under(path, UploadsPrefix) || uploadID == "" {
		return nil, ErrPathNotAllowed
	}
	if len(numbers) == 0 || len(numbers) > maxPartsAtOnce {
		return nil, ErrBadPart
	}

	ctx, cancel := requestContext()
	defer cancel()

	urls := make([]string, 0, len(numbers))
	for _, number := range numbers {
		if number < 1 || number > maxPartNumber {
			return nil, ErrBadPart
		}

		req, err := s.presigner.PresignUploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(s.config.Bucket),
			Key:        aws.String(path),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(number),
		}, s3.WithPresignExpires(partExpiry))
		if err != nil {
			return nil, err
		}
		urls = append(urls, req.URL)
	}

	return urls, nil
}

// UploadComplete makes one file of the parts that were sent, and answers
// with its size.
//
// Which parts there are, and how big, is asked of the store and not of the
// browser: it is the store that knows what arrived. A file past `maxSize`
// is thrown away, parts and all, so that saying a file is small and sending
// a large one gains nothing.
func (s *Storage) UploadComplete(path, uploadID string, maxSize int64) (int64, error) {
	if !under(path, UploadsPrefix) || uploadID == "" {
		return 0, ErrPathNotAllowed
	}

	ctx, cancel := requestContext()
	defer cancel()

	var (
		parts  []s3types.CompletedPart
		total  int64
		marker *string
	)
	for {
		page, err := s.client.ListParts(ctx, &s3.ListPartsInput{
			Bucket:           aws.String(s.config.Bucket),
			Key:              aws.String(path),
			UploadId:         aws.String(uploadID),
			PartNumberMarker: marker,
		})
		if err != nil {
			return 0, err
		}
		for _, part := range page.Parts {
			total += aws.ToInt64(part.Size)
			parts = append(parts, s3types.CompletedPart{
				ETag:       part.ETag,
				PartNumber: part.PartNumber,
			})
		}
		if !aws.ToBool(page.IsTruncated) || page.NextPartNumberMarker == nil {
			break
		}
		marker = page.NextPartNumberMarker
	}

	if len(parts) == 0 {
		return 0, ErrNothingSent
	}
	if maxSize > 0 && total > maxSize {
		_ = s.UploadAbort(path, uploadID)
		return 0, fmt.Errorf("%w: %d bytes", ErrTooLarge, total)
	}

	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(s.config.Bucket),
		Key:             aws.String(path),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &s3types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		return 0, err
	}

	return total, nil
}

// UploadAbort throws away an upload that was begun, and the parts of it
// the store was keeping. One that is already gone is not an error.
func (s *Storage) UploadAbort(path, uploadID string) error {
	if !under(path, UploadsPrefix) || uploadID == "" {
		return ErrPathNotAllowed
	}

	ctx, cancel := requestContext()
	defer cancel()

	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.config.Bucket),
		Key:      aws.String(path),
		UploadId: aws.String(uploadID),
	})
	if err != nil && !isNotFound(err) {
		var noUpload *s3types.NoSuchUpload
		if errors.As(err, &noUpload) {
			return nil
		}
		return err
	}
	return nil
}

// Stat says whether a file is there, how big it is and what it is kept as.
func (s *Storage) Stat(path string) (exists bool, size int64, contentType string, err error) {
	ctx, cancel := requestContext()
	defer cancel()

	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		if isNotFound(err) {
			return false, 0, "", nil
		}
		return false, 0, "", err
	}

	return true, aws.ToInt64(out.ContentLength), aws.ToString(out.ContentType), nil
}

// FetchURL is an address another service can read a file at, by the
// endpoint the services reach the store by. For a worker reading an upload,
// which is too big to hand over in a message.
func (s *Storage) FetchURL(path string) (string, error) {
	if !under(path, UploadsPrefix) && !under(path, WorkerPrefix) {
		return "", ErrPathNotAllowed
	}

	ctx, cancel := requestContext()
	defer cancel()

	req, err := s.internal.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	}, s3.WithPresignExpires(transferExpiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// StoreURL is an address a worker can write one of its files to, and only
// under videos/ and as one of the types a worker makes. What a reader is
// sent it as does not rest on that: see readerRules.
func (s *Storage) StoreURL(path, contentType string) (string, error) {
	if !under(path, WorkerPrefix) {
		return "", ErrPathNotAllowed
	}
	if !slices.Contains(WorkerTypes, contentType) {
		return "", fmt.Errorf("%w: %s", ErrTypeNotAllowed, contentType)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.config.Bucket),
		Key:         aws.String(path),
		ContentType: aws.String(contentType),
	}
	req, err := s.internal.PresignPutObject(ctx, input, s3.WithPresignExpires(transferExpiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
