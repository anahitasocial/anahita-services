package config

import (
	"anahita-libs/getenv"
	"errors"
	"os"
	"strings"

	"anahita-libs/constants"
)

type Config struct {
	Env              string
	FileServiceAddr  string
	GraphServiceAddr string
	// Where the web app's build is: the folder with index.html in it.
	WebBuildDir string
	// The address people reach the web app at, "https://www.example.org".
	// Written into the tags as each page's own address. Left out, the
	// address the request came to is used.
	PublicWebURL string
}

func NewConfig() *Config {
	return &Config{
		Env:              getenv.String("ENV", constants.ConfigEnvDev),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		WebBuildDir:      getenv.String("WEB_BUILD_DIR", "/app/web"),
		PublicWebURL:     strings.TrimRight(os.Getenv("PUBLIC_WEB_URL"), "/"),
	}
}

func (config *Config) Validate() error {
	var missing []string

	if config.FileServiceAddr == "" {
		missing = append(missing, "FILE_SERVICE_ADDR")
	}
	if config.GraphServiceAddr == "" {
		missing = append(missing, "GRAPH_SERVICE_ADDR")
	}
	if len(missing) > 0 {
		return errors.New("missing: " + strings.Join(missing, ", "))
	}

	return nil
}

func (config *Config) IsDev() bool {
	return config.Env == constants.ConfigEnvDev
}

func (config *Config) GetFileServiceAddr() string {
	return config.FileServiceAddr
}

func (config *Config) GetGraphServiceAddr() string {
	return config.GraphServiceAddr
}
