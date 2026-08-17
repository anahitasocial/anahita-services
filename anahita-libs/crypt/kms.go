package crypt

import (
	"context"
	b64 "encoding/base64"
	"fmt"

	"anahita-libs/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

type Crypt interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

func NewKmsCrypt(config ConfigInterface) Crypt {
	return &kmsCrypt{
		awsKmsKeyId: config.GetAwsKmsKeyId(),
	}
}

type kmsCrypt struct {
	awsKmsKeyId string
}

// Encrypt encrypts the given text using AWS KMS
func (crypt *kmsCrypt) Encrypt(text string) (string, error) {
	// Load AWS default config
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(constants.CryptKMSRegion))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create KMS service client
	svc := kms.NewFromConfig(cfg)

	// Encrypt the data
	result, err := svc.Encrypt(context.TODO(), &kms.EncryptInput{
		KeyId:     aws.String(crypt.awsKmsKeyId),
		Plaintext: []byte(text),
	})

	if err != nil {
		return "", err
	}

	// Encode the ciphertext blob into base64
	blobString := b64.StdEncoding.EncodeToString(result.CiphertextBlob)

	return blobString, nil
}

// Decrypt decrypts the given base64-encoded ciphertext using AWS KMS
func (crypt *kmsCrypt) Decrypt(text string) (string, error) {
	// Load AWS default config
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(constants.CryptKMSRegion))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create KMS service client
	svc := kms.NewFromConfig(cfg)

	// Decode the base64-encoded ciphertext
	blob, err := b64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 string: %w", err)
	}

	// Decrypt the data
	result, err := svc.Decrypt(context.TODO(), &kms.DecryptInput{
		CiphertextBlob: blob,
	})

	if err != nil {
		fmt.Println("Got error decrypting data: ", err)
		return "", err
	}

	// Return the decrypted plaintext
	return string(result.Plaintext), nil
}
