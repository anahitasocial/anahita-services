package storage

type ConfigInterface interface {
	GetAccessKeyID() string
	GetSecretAccessKey() string
	GetRegion() string
	GetBucket() string
	GetAssetsDir() string
}
