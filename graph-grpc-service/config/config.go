package config

import (
	"anahita-libs/constants"
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

const DefaultActorFeaturesPath = "/defaults/actor_features"

type Config struct {
	Env               string
	DSN               string
	AmqpAddr          string
	ActorFeaturesPath string
}

func NewConfig() *Config {
	config := &Config{
		Env:               getEnv("ENV", constants.ConfigEnvDev),
		DSN:               os.Getenv("DSN"),
		AmqpAddr:          os.Getenv("AMQP_ADDR"),
		ActorFeaturesPath: getEnv("ACTOR_FEATURES_PATH", "/defaults/actor_features"),
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
