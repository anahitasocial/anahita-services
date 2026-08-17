package config

import (
	"os"
	"testing"

	"anahita-libs/constants"
)

func setEnv(key, value string) func() {
	orig := os.Getenv(key)
	os.Setenv(key, value)
	return func() { os.Setenv(key, orig) }
}

func TestNewConfig_Defaults(t *testing.T) {
	unsetEnv := []func(){}
	unsetEnv = append(unsetEnv, setEnv("ENV", ""))
	unsetEnv = append(unsetEnv, setEnv("AWS_ACCESS_KEY_ID", "test-access-key"))
	unsetEnv = append(unsetEnv, setEnv("AWS_SECRET_ACCESS_KEY", "test-secret"))
	unsetEnv = append(unsetEnv, setEnv("AWS_REGION", "us-west-1"))
	unsetEnv = append(unsetEnv, setEnv("AWS_FILES_S3_BUCKET", "test-bucket"))
	unsetEnv = append(unsetEnv, setEnv("AWS_FILES_S3_ASSETS_DIR", "assets"))
	defer func() {
		for _, f := range unsetEnv {
			f()
		}
	}()

	cfg := NewConfig()
	if cfg.Env != constants.ConfigEnvDev {
		t.Errorf("expected default env %s, got %s", constants.ConfigEnvDev, cfg.Env)
	}
	if cfg.Aws.AccessKeyID != "test-access-key" {
		t.Errorf("expected access key, got %s", cfg.Aws.AccessKeyID)
	}
	if cfg.Aws.SecretAccessKey != "test-secret" {
		t.Errorf("expected secret, got %s", cfg.Aws.SecretAccessKey)
	}
	if cfg.Aws.Region != "us-west-1" {
		t.Errorf("expected region, got %s", cfg.Aws.Region)
	}
	if cfg.Aws.Bucket != "test-bucket" {
		t.Errorf("expected bucket, got %s", cfg.Aws.Bucket)
	}
	if cfg.Aws.AssetsDir != "assets" {
		t.Errorf("expected assets dir, got %s", cfg.Aws.AssetsDir)
	}
}

func TestConfig_Validate_AllFieldsPresent(t *testing.T) {
	cfg := &Config{
		Aws: Aws{
			Region:    "us-west-1",
			Bucket:    "bucket",
			AssetsDir: "assets",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestConfig_Validate_MissingFields(t *testing.T) {
	cfg := &Config{
		Aws: Aws{},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
	msg := err.Error()
	if !contains(msg, ErrMissingAwsRegion) || !contains(msg, ErrMissingAwsBucket) || !contains(msg, ErrMissingAwsAssetsDir) {
		t.Errorf("unexpected error message: %s", msg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
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

func TestConfig_Getters(t *testing.T) {
	cfg := &Config{
		Aws: Aws{
			AccessKeyID:     "id",
			SecretAccessKey: "secret",
			Region:          "region",
			Bucket:          "bucket",
			AssetsDir:       "assets",
		},
	}
	if cfg.GetAccessKeyID() != "id" {
		t.Error("GetAccessKeyID failed")
	}
	if cfg.GetSecretAccessKey() != "secret" {
		t.Error("GetSecretAccessKey failed")
	}
	if cfg.GetRegion() != "region" {
		t.Error("GetRegion failed")
	}
	if cfg.GetBucket() != "bucket" {
		t.Error("GetBucket failed")
	}
	if cfg.GetAssetsDir() != "assets" {
		t.Error("GetAssetsDir failed")
	}
}
