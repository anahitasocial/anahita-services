package defaults

import (
	"encoding/json"
	"feed/config"
	"fmt"
	"os"
	"path/filepath"
)

type DefaultsLoader struct {
	dir      string
	fileName string
}

func NewDefaultsLoader(config *config.Config) *DefaultsLoader {
	return &DefaultsLoader{
		dir:      config.GetDefaultsPath(),
		fileName: "defaults.json",
	}
}

func (l *DefaultsLoader) Load() (*FeedDefaults, error) {
	path := filepath.Join(l.dir, l.fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read defaults from %s: %w", path, err)
	}

	defaults := &FeedDefaults{}
	err = json.Unmarshal(data, defaults)
	if err != nil {
		return nil, fmt.Errorf("failed to parse defaults from %s: %w", path, err)
	}

	return defaults, nil
}
