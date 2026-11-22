package config

import (
	"anahita-libs/getenv"
	"anahita-libs/settings"
	"errors"
	"os"
	"strings"

	"anahita-libs/constants"
)

// Where Bluesky is read from, unless the installation says otherwise.
const (
	// The public reader of the network: who somebody is, and what they
	// posted. It asks for no account.
	defaultAppView = "https://public.api.bsky.app"
	// Where what is posted is heard as it happens: every one Bluesky runs,
	// listened to in turn, since any one of them can fall behind.
	defaultJetstream = "wss://jetstream1.us-east.bsky.network/subscribe," +
		"wss://jetstream2.us-west.bsky.network/subscribe," +
		"wss://jetstream2.us-east.bsky.network/subscribe," +
		"wss://jetstream1.us-west.bsky.network/subscribe"
	// Where pictures are fetched from, to be copied here.
	defaultCDN = "https://cdn.bsky.app"
	// Where the server that holds an account is looked up.
	defaultDirectory = "https://plc.directory"
)

type Config struct {
	settings.Common

	// Where link-preview-service is, for the cards of links in posts from
	// Bluesky. Empty for none.
	LinkPreviewURL string

	AppViewURL string
	// One or more, with commas between.
	JetstreamURLs []string
	CDNURL        string
	DirectoryURL  string
	// How the token of each connected account is kept: encrypted, by the
	// same means auth-service keeps its secrets. Without a key, nobody
	// connects an account and nothing is written to Bluesky.
	CryptBackend  string
	CryptLocalKey string
	AwsKmsKeyId   string
}

func NewConfig() *Config {
	config := &Config{
		Common:         settings.ReadCommon(),
		LinkPreviewURL: os.Getenv("LINK_PREVIEW_URL"),
		AppViewURL:     strings.TrimRight(getenv.String("ATPROTO_APPVIEW_URL", defaultAppView), "/"),
		JetstreamURLs:  list(getenv.String("ATPROTO_JETSTREAM_URL", defaultJetstream)),
		CDNURL:         strings.TrimRight(getenv.String("ATPROTO_CDN_URL", defaultCDN), "/"),
		DirectoryURL:   strings.TrimRight(getenv.String("ATPROTO_PLC_URL", defaultDirectory), "/"),
		CryptBackend:   getenv.String("CRYPT_BACKEND", constants.CryptBackendLocal),
		CryptLocalKey:  os.Getenv("CRYPT_LOCAL_KEY"),
		AwsKmsKeyId:    os.Getenv("AWS_KMS_KEY_ID"),
	}

	return config
}

func (config *Config) Validate() error {
	if len(config.SecureCookieKey) != 32 {
		return errors.New("SECURE_COOKIE_KEY must be 32 bytes long")
	}
	var names []string
	for name, value := range map[string]string{
		"DSN":                config.DSN,
		"GRAPH_SERVICE_ADDR": config.GraphServiceAddr,
		"FILE_SERVICE_ADDR":  config.FileServiceAddr,
		"AMQP_ADDR":          config.AmqpAddr,
		"OAUTH_ISSUER":       config.OAuthIssuer,
		"OAUTH_JWKS_URL":     config.OAuthJWKSURL,
	} {
		if value == "" {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		return errors.New("missing: " + strings.Join(names, ", "))
	}
	if !strings.HasPrefix(config.AppViewURL, "https://") || !strings.HasPrefix(config.CDNURL, "https://") || !strings.HasPrefix(config.DirectoryURL, "https://") {
		return errors.New("ATPROTO_APPVIEW_URL, ATPROTO_CDN_URL and ATPROTO_PLC_URL must be https addresses")
	}
	if len(config.JetstreamURLs) == 0 {
		return errors.New("ATPROTO_JETSTREAM_URL names no address")
	}
	for _, address := range config.JetstreamURLs {
		if !strings.HasPrefix(address, "wss://") {
			return errors.New("ATPROTO_JETSTREAM_URL must be wss addresses, with commas between")
		}
	}
	return nil
}

func (config *Config) GetDomainName() string    { return os.Getenv("DOMAIN_NAME") }
func (config *Config) GetCryptBackend() string  { return config.CryptBackend }
func (config *Config) GetCryptLocalKey() string { return config.CryptLocalKey }
func (config *Config) GetAwsKmsKeyId() string   { return config.AwsKmsKeyId }

// CanKeepSecrets says whether there is a key to keep tokens under.
func (config *Config) CanKeepSecrets() bool {
	return config.CryptBackend != constants.CryptBackendLocal || config.CryptLocalKey != ""
}

// list is what was written with commas between, without what is empty.
func list(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
