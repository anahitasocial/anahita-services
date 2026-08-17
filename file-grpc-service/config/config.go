package config

import (
	"anahita-libs/constants"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrMissingAwsRegion    = "AWS region is missing"
	ErrMissingAwsBucket    = "AWS bucket is missing"
	ErrMissingAwsAssetsDir = "AWS assets dir is missing"
)

type Aws struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Bucket          string
	AssetsDir       string
}

type Config struct {
	Aws      Aws
	AmqpAddr string
	Env      string
}

func NewConfig() *Config {
	config := &Config{
		Env: getEnv("ENV", constants.ConfigEnvDev),
		Aws: Aws{
			AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			Region:          os.Getenv("AWS_REGION"),
			Bucket:          os.Getenv("AWS_FILES_S3_BUCKET"),
			AssetsDir:       os.Getenv("AWS_FILES_S3_ASSETS_DIR"),
		},
	}

	return config
}

func (config *Config) Validate() error {
	var missingFields []string

	if config.Aws.Region == "" {
		missingFields = append(missingFields, ErrMissingAwsRegion)
	}

	if config.Aws.Bucket == "" {
		missingFields = append(missingFields, ErrMissingAwsBucket)
	}

	if config.Aws.AssetsDir == "" {
		missingFields = append(missingFields, ErrMissingAwsAssetsDir)
	}

	if len(missingFields) > 0 {
		return errors.New("missing fields: " + strings.Join(missingFields, ", "))
	}

	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

func (config *Config) IsDev() bool {
	return config.Env == constants.ConfigEnvDev
}

func (config *Config) IsStaging() bool {
	return config.Env == constants.ConfigEnvStaging
}

func (config *Config) IsProd() bool {
	return config.Env == constants.ConfigEnvProd
}

func (config *Config) GetAccessKeyID() string {
	return config.Aws.AccessKeyID
}

func (config *Config) GetSecretAccessKey() string {
	return config.Aws.SecretAccessKey
}

func (config *Config) GetRegion() string {
	return config.Aws.Region
}

func (config *Config) GetBucket() string {
	return config.Aws.Bucket
}

func (config *Config) GetAssetsDir() string {
	return config.Aws.AssetsDir
}
