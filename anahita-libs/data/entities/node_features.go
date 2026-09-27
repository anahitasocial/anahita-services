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

// AddPermissionAccess returns the access level the actor has set for adding
// entity under service, or "" when it has none.
//
// Only as good as the features loaded: the defaults are merged in by the graph
// service's actor read, so an owner taken from a medium or node read — which
// carry the raw column, NULL for most actors — answers "" for everything.
func (a *AnahitaNode) AddPermissionAccess(service string, entity string) string {
	features, err := a.GetFeatures()
	if err != nil {
		return ""
	}
	for _, feature := range features {
		if feature.Service != service {
			continue
		}
		for _, perm := range feature.AddPermissions {
			if perm.Entity == entity {
				return perm.Access
			}
		}
	}
	return ""
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
