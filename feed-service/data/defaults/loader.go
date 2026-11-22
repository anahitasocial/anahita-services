package defaults

import (
	"encoding/json"
	"feed/config"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// What was read, by its path: the file is part of the image and does not
// change while the service runs, and it was read and parsed again for every
// feed asked for (roadmap row 170). Read only, by those it is given to.
var loaded sync.Map

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
	if kept, ok := loaded.Load(path); ok {
		return kept.(*FeedDefaults), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read defaults from %s: %w", path, err)
	}

	defaults := &FeedDefaults{}
	err = json.Unmarshal(data, defaults)
	if err != nil {
		return nil, fmt.Errorf("failed to parse defaults from %s: %w", path, err)
	}

	loaded.Store(path, defaults)
	return defaults, nil
}
