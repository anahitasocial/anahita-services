package entities

import "encoding/json"

// MetaMixingInterface is the interface that provides the methods
type MetaMixingInterface interface {
	GetMetadata() string
	SetMetadata(string, interface{}) error
}

// SetMetadata sets the metadata of the node
func (a *AnahitaNode) SetMetadata(key string, value interface{}) error {
	jsonMeta := make(map[string]interface{})
	err := json.Unmarshal([]byte(a.Meta), &jsonMeta)
	if err != nil {
		return err
	}

	// add key value to the map
	jsonMeta[key] = value

	// convert the map to json
	jsonMetaByte, err := json.Marshal(jsonMeta)
	if err != nil {
		return err
	}

	a.Meta = jsonMetaByte

	return nil
}

// GetMetadata gets the metadata of the node
func (a *AnahitaNode) GetMetadata(key string) (string, error) {
	jsonMeta := make(map[string]any)
	err := json.Unmarshal([]byte(a.Meta), &jsonMeta)
	if err != nil {
		return "", err
	}

	if key == "" {
		return "", nil
	}

	if jsonMeta[key] == nil {
		return "", nil
	}

	return jsonMeta[key].(string), nil
}
