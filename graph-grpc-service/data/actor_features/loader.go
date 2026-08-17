package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"anahita-libs/data/entities"
)

type DefaultConfig struct {
	Version           string                                         `json:"version"`
	Services          []*entities.AnahitaNodeFeature                 `json:"services"`
	CommonPermissions map[string][]entities.AnahitaNodeAddPermission `json:"common_permissions"`
}

// FeatureOverride uses pointers to distinguish "not set" from "set to false"
type FeatureOverride struct {
	Service                  string                              `json:"service"`
	NodeTypes                []string                            `json:"node_types,omitempty"`
	Composers                []string                            `json:"composers,omitempty"`
	Optional                 *bool                               `json:"optional,omitempty"`
	Enabled                  *bool                               `json:"enabled,omitempty"`
	AddPermissions           []entities.AnahitaNodeAddPermission `json:"add_permissions,omitempty"`
	IncludeCommonPermissions []string                            `json:"include_common_permissions,omitempty"`
	Ordering                 *int32                              `json:"ordering,omitempty"`
}

type FeatureLoader struct {
	Directory         string
	cache             map[string][]*entities.AnahitaNodeFeature
	commonPermissions map[string][]entities.AnahitaNodeAddPermission
}

func NewFeatureLoader(dir string) *FeatureLoader {
	return &FeatureLoader{
		Directory: dir,
		cache:     make(map[string][]*entities.AnahitaNodeFeature),
	}
}

func (fl *FeatureLoader) Load(nodeType string) ([]*entities.AnahitaNodeFeature, error) {
	filename := nodeTypeToFileName(nodeType)

	if cached, ok := fl.cache[filename]; ok {
		return fl.cloneFeatures(cached), nil
	}

	defaults, err := fl.loadDefaultFile()
	if err != nil {
		return nil, fmt.Errorf("failed to load default features: %w", err)
	}

	if filename == "default.json" {
		fl.cache[filename] = defaults
		return fl.cloneFeatures(defaults), nil
	}

	overrides, err := fl.loadOverrideFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			fl.cache[filename] = defaults
			return fl.cloneFeatures(defaults), nil
		}
		return nil, fmt.Errorf("failed to load features from %s: %w", filename, err)
	}

	merged := fl.Merge(defaults, overrides)
	fl.cache[filename] = merged

	return fl.cloneFeatures(merged), nil
}

func (fl *FeatureLoader) loadDefaultFile() ([]*entities.AnahitaNodeFeature, error) {
	path := filepath.Join(fl.Directory, "default.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config DefaultConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse features from default.json: %w", err)
	}

	fl.commonPermissions = config.CommonPermissions

	return config.Services, nil
}

func (fl *FeatureLoader) loadOverrideFile(filename string) ([]*FeatureOverride, error) {
	path := filepath.Join(fl.Directory, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var overrides []*FeatureOverride
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("failed to parse features from %s: %w", filename, err)
	}

	return overrides, nil
}

func (fl *FeatureLoader) Merge(defaults []*entities.AnahitaNodeFeature, overrides []*FeatureOverride) []*entities.AnahitaNodeFeature {
	overrideMap := make(map[string]*FeatureOverride)
	for _, f := range overrides {
		overrideMap[f.Service] = f
	}

	seen := make(map[string]bool)
	var merged []*entities.AnahitaNodeFeature

	for _, def := range defaults {
		seen[def.Service] = true

		if override, ok := overrideMap[def.Service]; ok {
			merged = append(merged, fl.mergeFeature(def, override))
		} else {
			merged = append(merged, fl.cloneFeature(def))
		}
	}

	for _, override := range overrides {
		if !seen[override.Service] {
			merged = append(merged, fl.overrideToFeature(override))
		}
	}

	return merged
}

func (fl *FeatureLoader) mergeFeature(base *entities.AnahitaNodeFeature, override *FeatureOverride) *entities.AnahitaNodeFeature {
	merged := fl.cloneFeature(base)

	if len(override.NodeTypes) > 0 {
		merged.NodeTypes = override.NodeTypes
	}

	if len(override.Composers) > 0 {
		merged.Composers = override.Composers
	}

	if override.Optional != nil {
		merged.Optional = *override.Optional
	}

	if override.Enabled != nil {
		merged.Enabled = *override.Enabled
	}

	if len(override.AddPermissions) > 0 {
		merged.AddPermissions = override.AddPermissions
	}

	if len(override.IncludeCommonPermissions) > 0 && fl.commonPermissions != nil {
		for _, key := range override.IncludeCommonPermissions {
			if perms, ok := fl.commonPermissions[key]; ok {
				merged.AddPermissions = append(merged.AddPermissions, perms...)
			}
		}
	}

	if override.Ordering != nil && *override.Ordering > 0 {
		merged.Ordering = *override.Ordering
	}

	return merged
}

func (fl *FeatureLoader) overrideToFeature(override *FeatureOverride) *entities.AnahitaNodeFeature {
	f := &entities.AnahitaNodeFeature{
		Service:   override.Service,
		NodeTypes: override.NodeTypes,
		Composers: override.Composers,
	}

	if override.Optional != nil {
		f.Optional = *override.Optional
	}

	if override.Enabled != nil {
		f.Enabled = *override.Enabled
	}

	if override.Ordering != nil {
		f.Ordering = *override.Ordering
	}

	f.AddPermissions = override.AddPermissions

	if len(override.IncludeCommonPermissions) > 0 && fl.commonPermissions != nil {
		for _, key := range override.IncludeCommonPermissions {
			if perms, ok := fl.commonPermissions[key]; ok {
				f.AddPermissions = append(f.AddPermissions, perms...)
			}
		}
	}

	if f.NodeTypes == nil {
		f.NodeTypes = []string{}
	}
	if f.Composers == nil {
		f.Composers = []string{}
	}
	if f.AddPermissions == nil {
		f.AddPermissions = []entities.AnahitaNodeAddPermission{}
	}

	return f
}

func (fl *FeatureLoader) cloneFeature(f *entities.AnahitaNodeFeature) *entities.AnahitaNodeFeature {
	clone := &entities.AnahitaNodeFeature{
		Service:  f.Service,
		Optional: f.Optional,
		Enabled:  f.Enabled,
		Ordering: f.Ordering,
	}

	if f.NodeTypes != nil {
		clone.NodeTypes = make([]string, len(f.NodeTypes))
		copy(clone.NodeTypes, f.NodeTypes)
	} else {
		clone.NodeTypes = []string{}
	}

	if f.Composers != nil {
		clone.Composers = make([]string, len(f.Composers))
		copy(clone.Composers, f.Composers)
	} else {
		clone.Composers = []string{}
	}

	if f.AddPermissions != nil {
		clone.AddPermissions = make([]entities.AnahitaNodeAddPermission, len(f.AddPermissions))
		copy(clone.AddPermissions, f.AddPermissions)
	} else {
		clone.AddPermissions = []entities.AnahitaNodeAddPermission{}
	}

	if f.IncludeCommonPermissions != nil {
		clone.IncludeCommonPermissions = make([]string, len(f.IncludeCommonPermissions))
		copy(clone.IncludeCommonPermissions, f.IncludeCommonPermissions)
	}

	return clone
}

func (fl *FeatureLoader) cloneFeatures(features []*entities.AnahitaNodeFeature) []*entities.AnahitaNodeFeature {
	cloned := make([]*entities.AnahitaNodeFeature, len(features))
	for i, f := range features {
		cloned[i] = fl.cloneFeature(f)
	}
	return cloned
}

func nodeTypeToFileName(nodeType string) string {
	parts := strings.Split(nodeType, ".")
	if len(parts) >= 4 {
		return fmt.Sprintf("%s.json", parts[3])
	}
	return "default.json"
}
