package features

import (
	"encoding/json"
	"log"

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
	merged = loader.ForSite(merged)

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

// ForSite takes out of a profile's merged features what the installation
// does not offer: video where VIDEO_UPLOADS is not on, sound where
// AUDIO_UPLOADS is off, and what instance.json leaves out.
func (fl *FeatureLoader) ForSite(merged []*entities.AnahitaNodeFeature) []*entities.AnahitaNodeFeature {
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

	// What the instance offers (instance.json, roadmap row 139): every
	// profile has only that, whatever its kind's file or its own choices
	// say, as with video above. A file that cannot be read is said in the
	// log and offers everything, as no file does: a mistake in it does not
	// leave every profile with nothing.
	instance, err := fl.Instance()
	if err != nil {
		log.Printf("instance.json: %v; everything is offered", err)
	}
	merged = instance.Cut(merged)
	return merged
}
