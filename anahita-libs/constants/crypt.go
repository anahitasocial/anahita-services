package constants

const (
	CryptKMSRegion = "ca-central-1"
)

const (
	// CryptBackendLocal encrypts and decrypts with an AES-256 key held
	// in the service's own configuration. No AWS dependency — this is
	// the default so a deployment can run without KMS.
	CryptBackendLocal = "local"
	// CryptBackendKMS delegates encryption to AWS KMS.
	CryptBackendKMS = "kms"
)
