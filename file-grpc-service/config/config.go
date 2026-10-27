package config

import (
	"anahita-libs/constants"
	"anahita-libs/getenv"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	// SSENone sends no server-side-encryption header. SSEAES256 asks the
	// store to encrypt each object at rest with a key it manages.
	SSENone   = "none"
	SSEAES256 = "AES256"
)

// DefaultAllowedTypes is what may be stored when STORAGE_ALLOWED_TYPES is
// unset: the two image types every upload path re-encodes to.
var DefaultAllowedTypes = []string{"image/jpeg", "image/png"}

// Storage describes an S3-compatible object store: AWS S3, or anything that
// speaks its API (Garage, MinIO, Ceph, OVHcloud, Scaleway, Hetzner, …).
//
// Every setting has a neutral STORAGE_* name. The AWS_* names this service
// used to read are still honoured as fallbacks, so an installation configured
// before the rename keeps working untouched.
type Storage struct {
	// Endpoint is where this service reaches the store. Empty means AWS S3,
	// whose address the SDK derives from the region.
	Endpoint string

	// PublicEndpoint is the address written into presigned URLs, which
	// browsers load directly. It differs from Endpoint whenever the store is
	// reached by an internal name: a URL signed for http://garage:3900 is one
	// no browser can open. Empty means the same as Endpoint.
	PublicEndpoint string

	Region string
	Bucket string

	// AccessKey and SecretKey are static credentials. Both empty means the
	// SDK's default chain, which is how an AWS role or profile is picked up.
	AccessKey string
	SecretKey string

	// PathStyle addresses objects as <endpoint>/<bucket>/<key> instead of
	// <bucket>.<endpoint>/<key>. Self-hosted stores usually need it, and so
	// does any bucket whose name contains a dot when served over HTTPS.
	PathStyle bool

	// SSE is the server-side-encryption header sent with each upload.
	SSE string

	// SendACL sends `x-amz-acl: private` with each upload. Some stores reject
	// the header outright; buckets are private by default everywhere anyway.
	SendACL bool

	// AllowedTypes are the content types this service will store, judged by
	// the bytes themselves and not by what the caller declares.
	AllowedTypes []string
}

type Config struct {
	Storage  Storage
	AmqpAddr string
	Env      string
}

func NewConfig() *Config {
	endpoint := os.Getenv("STORAGE_ENDPOINT")

	// The defaults follow the endpoint. With none, this is AWS S3 and behaves
	// exactly as it did before these settings existed: encrypted, with a
	// private ACL. With one, it is some other store, and the conservative
	// choice is to send neither header, because a store that does not
	// implement one refuses the whole upload.
	onAws := endpoint == ""

	sse := SSENone
	if onAws {
		sse = SSEAES256
	}

	return &Config{
		Env: getenv.String("ENV", constants.ConfigEnvDev),
		Storage: Storage{
			Endpoint:       endpoint,
			PublicEndpoint: os.Getenv("STORAGE_PUBLIC_ENDPOINT"),
			Region:         firstEnv("STORAGE_REGION", "AWS_REGION", "AWS_DEFAULT_REGION"),
			Bucket:         firstEnv("STORAGE_BUCKET", "AWS_FILES_S3_BUCKET"),
			AccessKey:      firstEnv("STORAGE_ACCESS_KEY", "AWS_ACCESS_KEY_ID"),
			SecretKey:      firstEnv("STORAGE_SECRET_KEY", "AWS_SECRET_ACCESS_KEY"),
			PathStyle:      boolEnv("STORAGE_PATH_STYLE", false),
			SSE:            getenv.String("STORAGE_SSE", sse),
			SendACL:        boolEnv("STORAGE_SEND_ACL", onAws),
			AllowedTypes:   listEnv("STORAGE_ALLOWED_TYPES", DefaultAllowedTypes),
		},
	}
}

// Validate reports everything wrong at once, so a misconfigured installation
// is fixed in one pass rather than one restart per mistake.
func (config *Config) Validate() error {
	var problems []string
	s := config.Storage

	if s.Region == "" {
		problems = append(problems, "STORAGE_REGION is not set (any value will do for a store that has no regions, e.g. \"garage\")")
	}

	if s.Bucket == "" {
		problems = append(problems, "STORAGE_BUCKET is not set")
	}

	// One without the other is a half-configured credential, and falling
	// back to the default chain there would pick up whatever identity the
	// machine happens to have.
	if (s.AccessKey == "") != (s.SecretKey == "") {
		problems = append(problems, "STORAGE_ACCESS_KEY and STORAGE_SECRET_KEY must be set together")
	}

	if err := validEndpoint("STORAGE_ENDPOINT", s.Endpoint); err != nil {
		problems = append(problems, err.Error())
	}

	if err := validEndpoint("STORAGE_PUBLIC_ENDPOINT", s.PublicEndpoint); err != nil {
		problems = append(problems, err.Error())
	}

	if s.SSE != SSENone && s.SSE != SSEAES256 {
		problems = append(problems, fmt.Sprintf("STORAGE_SSE must be %q or %q, not %q", SSENone, SSEAES256, s.SSE))
	}

	if len(s.AllowedTypes) == 0 {
		problems = append(problems, "STORAGE_ALLOWED_TYPES is empty, so nothing could be stored")
	}

	if len(problems) > 0 {
		return errors.New("storage configuration: " + strings.Join(problems, "; "))
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

// GetPublicEndpoint is the address presigned URLs are written for.
func (s Storage) GetPublicEndpoint() string {
	if s.PublicEndpoint != "" {
		return s.PublicEndpoint
	}
	return s.Endpoint
}

// validEndpoint accepts an empty value, or an absolute http(s) URL with no
// path: the bucket and key are added by the client, and a path here would be
// silently dropped or doubled depending on the addressing style.
func validEndpoint(name, value string) error {
	if value == "" {
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an http(s) URL such as https://s3.example.com, not %q", name, value)
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("%s must not have a path (%q): the bucket is added for you", name, parsed.Path)
	}

	return nil
}

// firstEnv returns the first of the named variables that is set.
func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

// boolEnv reads a boolean. An unparsable value is fatal rather than read as
// the fallback: "STORAGE_PATH_STYLE=yes" quietly meaning false is the kind of
// mistake that costs an afternoon.
func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("%s must be true or false, not %q", key, value)
	}
	return parsed
}

func listEnv(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
