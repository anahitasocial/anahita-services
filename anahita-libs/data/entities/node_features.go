package entities

import "encoding/json"

// GetFeatures returns the features of the actor
func (a *AnahitaNode) GetFeatures() ([]*AnahitaNodeFeature, error) {
	if len(a.FeaturesRaw) != 0 {
		var features []*AnahitaNodeFeature
		err := json.Unmarshal([]byte(a.FeaturesRaw), &features)
		if err != nil {
			return nil, err
		}
		a.Features = features
	} else {
		a.Features = nil
	}

	return a.Features, nil
}

// SetFeatures sets the features of the actor
func (a *AnahitaNode) SetFeatures(features []*AnahitaNodeFeature) error {
	if features == nil {
		// nil, not []byte{}. json.RawMessage marshals nil as "null" but an
		// empty non-nil slice as nothing, which breaks any response carrying
		// the node — see rawJSON in data/services/graph_node_helper.go.
		a.FeaturesRaw = nil
		return nil
	}

	data, err := json.Marshal(features)
	if err != nil {
		return err
	}

	a.FeaturesRaw = data
	a.Features = features

	return nil
}

// HasAddPermission checks if the actor has add permission for a specific service, entity, and access
func (a *AnahitaNode) HasAddPermission(service string, entity string, access string) bool {
	features, err := a.GetFeatures()
	if err != nil {
		return false
	}
	for _, feature := range features {
		if feature.Service == service {
			for _, perm := range feature.AddPermissions {
				if perm.Entity == entity && perm.Access == access {
					return true
				}
			}
		}
	}
	return false
}
