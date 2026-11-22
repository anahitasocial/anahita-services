package config

import (
	"anahita-libs/getenv"
	"anahita-libs/settings"
	"errors"
	"log"
	"strings"
)

const (
	ErrMissingActorFeaturesPath = "Actor features path is missing"
)

// DefaultActorFeaturesPath is where the image keeps them: the dockerfile
// copies graph-grpc-defaults to /app/defaults.
const DefaultActorFeaturesPath = "/app/defaults/actor_features"

type Config struct {
	settings.Common

	ActorFeaturesPath string
}

func NewConfig() *Config {
	config := &Config{
		Common:            settings.ReadCommon(),
		ActorFeaturesPath: getenv.String("ACTOR_FEATURES_PATH", DefaultActorFeaturesPath),
	}

	return config
}

func (config *Config) Validate() error {
	missingFields := config.Missing(settings.Database, settings.Amqp)

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

func (config *Config) GetActorFeaturesPath() string {
	return config.ActorFeaturesPath
}
