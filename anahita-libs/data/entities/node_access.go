package entities

import (
	"anahita-libs/constants"
	"encoding/json"
)

// GetAccess returns the access of the node
func (a *AnahitaNode) GetAccess() string {
	if a.Access == "" {
		return constants.AnahitaAccessPublic
	}

	return a.Access
}

// SetAccess sets the access of the node
func (a *AnahitaNode) SetAccess(access string) {
	a.Access = access
}

// GetPermission returns the permission for a given key and value
// if the key doesn't exist, it returns the default value
func (a *AnahitaNode) GetPermission(key string, defaultValue string) string {
	if key == "access" {
		return a.Access
	}

	permissions, err := a.GetPermissions()
	if err != nil {
		return defaultValue
	}

	_, ok := permissions[key]
	if !ok {
		return defaultValue
	}

	return permissions[key]
}

// GetPermissions returns the permissions of the node
func (a *AnahitaNode) GetPermissions() (map[string]string, error) {
	if a.Permissions == nil {
		jsonData := make(map[string]string)
		err := json.Unmarshal([]byte(a.PermissionsRaw), &jsonData)
		if err != nil {
			return nil, err
		}

		a.Permissions = jsonData
	}

	return a.Permissions, nil
}
