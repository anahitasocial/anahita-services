package storage

import "time"

const (
	AwsS3AclPrivate        = "private"
	AwsS3AclPublic         = "public-read"
	AwsS3PresignExpireTime = 15 * time.Minute
)
