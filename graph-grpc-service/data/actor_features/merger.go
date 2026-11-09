package features

import (
	"encoding/json"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"anahita-libs/videos"
)

func ApplyFeatures(node *graphpb.Node, loader *FeatureLoader, defaults []*entities.AnahitaNodeFeature) error {
	var overrides []*FeatureOverride

	if node.FeaturesRaw != "" {
		if err := json.Unmarshal([]byte(node.FeaturesRaw), &overrides); err != nil {
			return err
		}
	}

	merged := loader.Merge(defaults, overrides)

	// Video is the installation's to switch on (VIDEO_UPLOADS). Where it is
	// not, no profile has the feature: no composer, no tab, and nothing a
	// profile's own settings can turn back on.
	if !videos.UploadsOn() {
		kept := merged[:0]
		for _, feature := range merged {
			if feature.Service != videos.Service {
				kept = append(kept, feature)
			}
		}
		merged = kept
	}

	// Sound the same (AUDIO_UPLOADS): where it is off, video-service's
	// feature keeps its videos and loses its audio posts, their composer and
	// who may add them.
	if !videos.AudioUploadsOn() {
		for i, feature := range merged {
			if feature.Service == videos.Service {
				withoutAudio(feature)
				merged[i] = feature
			}
		}
	}

	// Convert to protobuf features
	node.Features = make([]*graphpb.NodeFeature, len(merged))
	for i, f := range merged {
		node.Features[i] = &graphpb.NodeFeature{
			Service:        f.Service,
			NodeTypes:      f.NodeTypes,
			Composers:      f.Composers,
			Optional:       f.Optional,
			Enabled:        f.Enabled,
			AddPermissions: toProtoPermissions(f.AddPermissions),
			Ordering:       f.Ordering,
		}
	}

	// Also update the raw JSON if needed for storage
	data, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	node.FeaturesRaw = string(data)

	return nil
}

func toProtoPermissions(perms []entities.AnahitaNodeAddPermission) []*graphpb.NodeAddPermission {
	if perms == nil {
		return []*graphpb.NodeAddPermission{}
	}

	result := make([]*graphpb.NodeAddPermission, len(perms))
	for i, p := range perms {
		result[i] = &graphpb.NodeAddPermission{
			Entity: p.Entity,
			Access: p.Access,
		}
	}
	return result
}

// withoutAudio takes a feature's audio posts out of it.
func withoutAudio(feature *entities.AnahitaNodeFeature) {
	not := func(list []string) []string {
		var kept []string
		for _, each := range list {
			if each != videos.KindAudio {
				kept = append(kept, each)
			}
		}
		return kept
	}
	feature.NodeTypes = not(feature.NodeTypes)
	feature.Composers = not(feature.Composers)
	var permissions []entities.AnahitaNodeAddPermission
	for _, each := range feature.AddPermissions {
		if each.Entity != videos.KindAudio {
			permissions = append(permissions, each)
		}
	}
	feature.AddPermissions = permissions
}
