package config

import (
	"anahita-libs/constants"
	"anahita-libs/getenv"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrMissingDSN               = "DSN is missing"
	ErrMissingAmqpAddr          = "AMQP address is missing"
	ErrMissingActorFeaturesPath = "Actor features path is missing"
)

// DefaultActorFeaturesPath is where the image keeps them: the dockerfile
// copies graph-grpc-defaults to /app/defaults.
const DefaultActorFeaturesPath = "/app/defaults/actor_features"

type Config struct {
	Env               string
	DSN               string
	AmqpAddr          string
	ActorFeaturesPath string

	// FileServiceAddr lets the account sweeper delete uploaded blobs when it
	// purges a profile.
	//
	// Optional, and unset by the gRPC server, which has no use for it. Without
	// it the sweeper still purges the database and simply leaves the objects in
	// storage — which is the behaviour everything had before, so a deployment
	// that has not set it is no worse off than it was.
	FileServiceAddr string
}

func NewConfig() *Config {
	config := &Config{
		Env:               getenv.String("ENV", constants.ConfigEnvDev),
		DSN:               os.Getenv("DSN"),
		AmqpAddr:          os.Getenv("AMQP_ADDR"),
		ActorFeaturesPath: getenv.String("ACTOR_FEATURES_PATH", DefaultActorFeaturesPath),
		FileServiceAddr:   os.Getenv("FILE_SERVICE_ADDR"),
	}

	return config
}

func (config *Config) Validate() error {
	var missingFields []string

	if config.DSN == "" {
		missingFields = append(missingFields, ErrMissingDSN)
	}

	if config.AmqpAddr == "" {
		missingFields = append(missingFields, ErrMissingAmqpAddr)
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

func (config *Config) GetDSN() string {
	return config.DSN
}

func (config *Config) GetAmqpAddr() string {
	return config.AmqpAddr
}

func (config *Config) GetActorFeaturesPath() string {
	return config.ActorFeaturesPath
}

// GetFileServiceAddr returns the file service address, satisfying
// services.FileGRPCConfigInterface. Empty when unset — the sweeper checks for
// that rather than dialling nothing.
func (config *Config) GetFileServiceAddr() string {
	return config.FileServiceAddr
}
