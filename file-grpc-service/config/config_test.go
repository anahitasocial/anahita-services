package config

import (
	"strings"
	"testing"

	"anahita-libs/constants"
)

// every variable this package reads, cleared before each test so one on the
// developer's machine cannot decide the result.
var storageEnv = []string{
	"ENV",
	"STORAGE_ENDPOINT", "STORAGE_PUBLIC_ENDPOINT", "STORAGE_REGION", "STORAGE_BUCKET",
	"STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY", "STORAGE_PATH_STYLE", "STORAGE_SSE",
	"STORAGE_SEND_ACL", "STORAGE_ALLOWED_TYPES",
	"AWS_REGION", "AWS_DEFAULT_REGION", "AWS_FILES_S3_BUCKET", "AWS_FILES_S3_ASSETS_DIR",
	"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
}

func withEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for _, key := range storageEnv {
		t.Setenv(key, "")
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}

// An installation configured before the STORAGE_* names existed sets only the
// AWS_* ones. It has to come up exactly as it did: on AWS S3, encrypting, and
// sending a private ACL.
func TestTheOldAwsNamesStillConfigureAwsS3(t *testing.T) {
	withEnv(t, map[string]string{
		"AWS_REGION":              "ca-central-1",
		"AWS_FILES_S3_BUCKET":     "staging.example.org",
		"AWS_FILES_S3_ASSETS_DIR": "assets/v2",
		"AWS_ACCESS_KEY_ID":       "old-key",
		"AWS_SECRET_ACCESS_KEY":   "old-secret",
	})

	cfg := NewConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("refused: %v", err)
	}

	s := cfg.Storage
	if s.Region != "ca-central-1" || s.Bucket != "staging.example.org" || s.AccessKey != "old-key" || s.SecretKey != "old-secret" {
		t.Errorf("the AWS_* values were not read: %+v", s)
	}
	if s.Endpoint != "" || s.GetPublicEndpoint() != "" {
		t.Errorf("an endpoint appeared from nowhere: %q, %q", s.Endpoint, s.GetPublicEndpoint())
	}
	if s.SSE != SSEAES256 || !s.SendACL || s.PathStyle {
		t.Errorf("sse=%q acl=%v pathStyle=%v, want AES256, true, false", s.SSE, s.SendACL, s.PathStyle)
	}
	if cfg.Env != constants.ConfigEnvDev {
		t.Errorf("env = %q, want the dev default", cfg.Env)
	}
}

// AWS_DEFAULT_REGION is what docker-compose and most AWS tooling set.
func TestTheRegionFallsBackThroughTheAwsNames(t *testing.T) {
	withEnv(t, map[string]string{"AWS_DEFAULT_REGION": "eu-west-3", "AWS_FILES_S3_BUCKET": "b"})

	if got := NewConfig().Storage.Region; got != "eu-west-3" {
		t.Errorf("region = %q, want eu-west-3", got)
	}
}

func TestTheNeutralNamesWin(t *testing.T) {
	withEnv(t, map[string]string{
		"AWS_REGION": "ca-central-1", "AWS_FILES_S3_BUCKET": "old-bucket",
		"AWS_ACCESS_KEY_ID": "old-key", "AWS_SECRET_ACCESS_KEY": "old-secret",
		"STORAGE_REGION": "garage", "STORAGE_BUCKET": "anahita",
		"STORAGE_ACCESS_KEY": "new-key", "STORAGE_SECRET_KEY": "new-secret",
	})

	s := NewConfig().Storage
	if s.Region != "garage" || s.Bucket != "anahita" || s.AccessKey != "new-key" || s.SecretKey != "new-secret" {
		t.Errorf("the STORAGE_* values did not win: %+v", s)
	}
}

// A store that is not AWS may not implement SSE or ACLs, and one that does
// not refuses the upload. So with an endpoint set, neither is sent unless
// asked for.
func TestAnotherStoreSendsNeitherHeaderByDefault(t *testing.T) {
	withEnv(t, map[string]string{
		"STORAGE_ENDPOINT": "http://garage:3900", "STORAGE_PUBLIC_ENDPOINT": "http://localhost:3900",
		"STORAGE_REGION": "garage", "STORAGE_BUCKET": "anahita", "STORAGE_PATH_STYLE": "true",
	})

	cfg := NewConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("refused: %v", err)
	}

	s := cfg.Storage
	if s.SSE != SSENone || s.SendACL {
		t.Errorf("sse=%q acl=%v, want none and false", s.SSE, s.SendACL)
	}
	if !s.PathStyle {
		t.Error("path style was not read")
	}
	if s.GetPublicEndpoint() != "http://localhost:3900" {
		t.Errorf("public endpoint = %q", s.GetPublicEndpoint())
	}
}

func TestTheHeadersCanBeTurnedBackOn(t *testing.T) {
	withEnv(t, map[string]string{
		"STORAGE_ENDPOINT": "https://s3.example.com", "STORAGE_REGION": "r", "STORAGE_BUCKET": "b",
		"STORAGE_SSE": "AES256", "STORAGE_SEND_ACL": "true",
	})

	s := NewConfig().Storage
	if s.SSE != SSEAES256 || !s.SendACL {
		t.Errorf("sse=%q acl=%v, want AES256 and true", s.SSE, s.SendACL)
	}
}

// Presigned URLs go to browsers. With no public endpoint they are signed for
// the one this service uses.
func TestThePublicEndpointDefaultsToTheEndpoint(t *testing.T) {
	s := Storage{Endpoint: "https://s3.example.com"}
	if got := s.GetPublicEndpoint(); got != "https://s3.example.com" {
		t.Errorf("public endpoint = %q", got)
	}
}

func TestAllowedTypes(t *testing.T) {
	withEnv(t, nil)
	if got := NewConfig().Storage.AllowedTypes; strings.Join(got, ",") != "image/jpeg,image/png" {
		t.Errorf("default allowed types = %v", got)
	}

	withEnv(t, map[string]string{"STORAGE_ALLOWED_TYPES": " image/jpeg , image/png,video/mp4 ,"})
	if got := NewConfig().Storage.AllowedTypes; strings.Join(got, ",") != "image/jpeg,image/png,video/mp4" {
		t.Errorf("allowed types = %v", got)
	}
}

func TestValidateNamesEveryProblemAtOnce(t *testing.T) {
	valid := func() Storage {
		return Storage{Region: "r", Bucket: "b", SSE: SSENone, AllowedTypes: DefaultAllowedTypes}
	}

	cases := []struct {
		what   string
		change func(*Storage)
		want   string
	}{
		{"no region", func(s *Storage) { s.Region = "" }, "STORAGE_REGION"},
		{"no bucket", func(s *Storage) { s.Bucket = "" }, "STORAGE_BUCKET"},
		{"a key with no secret", func(s *Storage) { s.AccessKey = "k" }, "must be set together"},
		{"a secret with no key", func(s *Storage) { s.SecretKey = "s" }, "must be set together"},
		{"an endpoint with no scheme", func(s *Storage) { s.Endpoint = "garage:3900" }, "STORAGE_ENDPOINT"},
		{"an endpoint that is not http", func(s *Storage) { s.Endpoint = "ftp://example.com" }, "STORAGE_ENDPOINT"},
		{"an endpoint with a path", func(s *Storage) { s.Endpoint = "https://s3.example.com/bucket" }, "must not have a path"},
		{"a public endpoint with a path", func(s *Storage) { s.PublicEndpoint = "https://cdn.example.com/media" }, "STORAGE_PUBLIC_ENDPOINT"},
		{"an unknown encryption", func(s *Storage) { s.SSE = "aws:kms" }, "STORAGE_SSE"},
		{"nothing allowed", func(s *Storage) { s.AllowedTypes = nil }, "STORAGE_ALLOWED_TYPES"},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			s := valid()
			c.change(&s)
			err := (&Config{Storage: s}).Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}

	t.Run("a valid one passes", func(t *testing.T) {
		s := valid()
		s.Endpoint, s.PublicEndpoint = "http://garage:3900", "https://media.example.com/"
		s.AccessKey, s.SecretKey = "k", "s"
		if err := (&Config{Storage: s}).Validate(); err != nil {
			t.Errorf("refused: %v", err)
		}
	})

	t.Run("two problems are both reported", func(t *testing.T) {
		err := (&Config{Storage: Storage{SSE: SSENone, AllowedTypes: DefaultAllowedTypes}}).Validate()
		if err == nil || !strings.Contains(err.Error(), "STORAGE_REGION") || !strings.Contains(err.Error(), "STORAGE_BUCKET") {
			t.Errorf("err = %v, want both the region and the bucket", err)
		}
	})
}

func TestConfig_IsEnv(t *testing.T) {
	cfg := &Config{Env: constants.ConfigEnvDev}
	if !cfg.IsDev() {
		t.Error("expected IsDev true")
	}
	cfg.Env = constants.ConfigEnvStaging
	if !cfg.IsStaging() {
		t.Error("expected IsStaging true")
	}
	cfg.Env = constants.ConfigEnvProd
	if !cfg.IsProd() {
		t.Error("expected IsProd true")
	}
}
