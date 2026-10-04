package crypt

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// KMSRegionEnv names the region the KMS key lives in. When it is unset the
// AWS SDK's own resolution applies (AWS_REGION, AWS_DEFAULT_REGION, the
// profile), which is what every other AWS call in this codebase uses.
//
// The region used to be a constant, "ca-central-1", written into this
// library, so a key anywhere else could not be reached whatever the
// environment said.
const KMSRegionEnv = "CRYPT_KMS_REGION"

// kmsTimeout bounds one call to KMS.
const kmsTimeout = 10 * time.Second

type Crypt interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

// NewKmsCrypt builds the KMS client once. It used to be rebuilt, credentials
// and all, on every Encrypt and every Decrypt.
func NewKmsCrypt(config ConfigInterface) (Crypt, error) {
	var options []func(*awsConfig.LoadOptions) error
	if region := os.Getenv(KMSRegionEnv); region != "" {
		options = append(options, awsConfig.WithRegion(region))
	}

	ctx, cancel := context.WithTimeout(context.Background(), kmsTimeout)
	defer cancel()

	cfg, err := awsConfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("crypt: load AWS config: %w", err)
	}

	if cfg.Region == "" {
		return nil, fmt.Errorf("crypt: no AWS region for KMS; set %s or AWS_REGION", KMSRegionEnv)
	}

	return &kmsCrypt{
		awsKmsKeyId: config.GetAwsKmsKeyId(),
		client:      kms.NewFromConfig(cfg),
	}, nil
}

type kmsCrypt struct {
	awsKmsKeyId string
	client      *kms.Client
}

// Encrypt encrypts the given text using AWS KMS
func (crypt *kmsCrypt) Encrypt(text string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), kmsTimeout)
	defer cancel()

	result, err := crypt.client.Encrypt(ctx, &kms.EncryptInput{
		KeyId:     aws.String(crypt.awsKmsKeyId),
		Plaintext: []byte(text),
	})
	if err != nil {
		return "", err
	}

	return b64.StdEncoding.EncodeToString(result.CiphertextBlob), nil
}

// Decrypt decrypts the given base64-encoded ciphertext using AWS KMS
func (crypt *kmsCrypt) Decrypt(text string) (string, error) {
	blob, err := b64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 string: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), kmsTimeout)
	defer cancel()

	result, err := crypt.client.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob: blob,
	})
	if err != nil {
		return "", err
	}

	return string(result.Plaintext), nil
}
