package features

import "anahita-libs/data/entities"

type FeatureService interface {
	Load(nodeType string) ([]*entities.AnahitaNodeFeature, error)
	Merge(defaults, overrides []*entities.AnahitaNodeFeature) []*entities.AnahitaNodeFeature
}
