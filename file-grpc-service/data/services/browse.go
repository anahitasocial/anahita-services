package services

import (
	"errors"

	"anahita-libs/proto/filepb"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// AssetsPrefix is the one folder Browse lists under: what the services
// store. Nothing else in the bucket is theirs to sweep.
const AssetsPrefix = "assets/v1/"

// BrowseMax is the most one page lists, S3's own most.
const BrowseMax = 1000

// ErrOutsideAssets is a folder that is not under assets/v1/.
var ErrOutsideAssets = errors.New("only what is under assets/v1/ is listed")

// Browse lists what is stored under prefix, a folder of assets/v1/, a page
// at a time in the order of the keys, from the key after `after`. It says
// whether there is more. The file sweep reads the bucket through this, so
// that it holds no keys to the bucket itself (docs/plans/done/file-sweep.md).
func (s *Storage) Browse(prefix, after string, limit int) ([]*filepb.BrowsedFile, bool, error) {
	if prefix != AssetsPrefix && !under(prefix, AssetsPrefix) {
		return nil, false, ErrOutsideAssets
	}
	if after != "" && !under(after, prefix) {
		return nil, false, ErrOutsideAssets
	}
	if limit <= 0 || limit > BrowseMax {
		limit = BrowseMax
	}

	ctx, cancel := requestContext()
	defer cancel()

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.config.Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(limit)),
	}
	if after != "" {
		input.StartAfter = aws.String(after)
	}
	page, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, false, err
	}

	files := make([]*filepb.BrowsedFile, 0, len(page.Contents))
	for _, object := range page.Contents {
		file := &filepb.BrowsedFile{Path: aws.ToString(object.Key), Size: aws.ToInt64(object.Size)}
		if object.LastModified != nil {
			file.ModifiedAt = object.LastModified.Unix()
		}
		files = append(files, file)
	}
	return files, aws.ToBool(page.IsTruncated), nil
}
