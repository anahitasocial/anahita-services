package crypt

// ConfigInterface is the subset of a service's Config that the crypt
// factory reads. Implemented by each service's own Config type.
type ConfigInterface interface {
	// GetCryptBackend reports which implementation NewCrypt should
	// build — constants.CryptBackendLocal or constants.CryptBackendKMS.
	GetCryptBackend() string
	// GetCryptLocalKey returns the 32-byte AES-256 key used by the
	// local backend. Only read when the backend is "local".
	GetCryptLocalKey() string
	// GetAwsKmsKeyId returns the AWS KMS key id used by the kms
	// backend. Only read when the backend is "kms".
	GetAwsKmsKeyId() string
}
