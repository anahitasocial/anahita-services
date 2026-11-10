package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// What the instance offers (roadmap row 139, docs/instance-features.md):
// instance.json, beside the files of each kind of profile and in their
// form. An instance is one big actor, and every profile has only what it
// has (the user, 2026-11-09).

// InstanceFile is the file's name in the features directory.
const InstanceFile = "instance.json"

// The service whose kinds are the kinds of profile that may be made, and
// the one kind always offered: a person is an account.
const (
	profilesService = "actor-service"
	personKind      = "person"
)

// Instance is what instance.json says. Nil is everything: no file, as
// before it was read.
type Instance struct {
	services map[string]*FeatureOverride
}

// LoadInstance reads instance.json from the directory, or nil when there
// is none.
func LoadInstance(dir string) (*Instance, error) {
	data, err := os.ReadFile(filepath.Join(dir, InstanceFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var listed []*FeatureOverride
	if err := json.Unmarshal(data, &listed); err != nil {
		return nil, fmt.Errorf("failed to parse features from %s: %w", InstanceFile, err)
	}
	instance := &Instance{services: map[string]*FeatureOverride{}}
	for _, each := range listed {
		if each != nil && each.Service != "" {
			instance.services[each.Service] = each
		}
	}
	return instance, nil
}

// Instance is what the loader's directory says the instance offers, read
// once.
func (fl *FeatureLoader) Instance() (*Instance, error) {
	if fl.instanceRead {
		return fl.instance, nil
	}
	instance, err := LoadInstance(fl.Directory)
	if err != nil {
		return nil, err
	}
	fl.instance, fl.instanceRead = instance, true
	return instance, nil
}

// OffersService is whether a service is offered: not listed, or listed and
// not switched off.
func (in *Instance) OffersService(service string) bool {
	if in == nil {
		return true
	}
	listed, ok := in.services[service]
	return !ok || listed.Enabled == nil || *listed.Enabled
}

// Offers is whether a kind of a service is offered: the service is, and
// its kinds are not listed or the kind is among them.
func (in *Instance) Offers(service, kind string) bool {
	if in == nil {
		return true
	}
	if !in.OffersService(service) {
		return false
	}
	listed, ok := in.services[service]
	return !ok || len(listed.NodeTypes) == 0 || slices.Contains(listed.NodeTypes, kind)
}

// OffersProfile is whether a kind of profile may be made: a person always.
func (in *Instance) OffersProfile(kind string) bool {
	return kind == personKind || in.Offers(profilesService, kind)
}

// Cut takes out of a profile's merged features what the instance does not
// offer: a service it does not offer, and of one it offers, the kinds it
// does not, with their composers and who may add them. Who may comment,
// like or mention is not a kind, and stays.
func (in *Instance) Cut(merged []*entities.AnahitaNodeFeature) []*entities.AnahitaNodeFeature {
	if in == nil {
		return merged
	}
	kept := merged[:0]
	for _, feature := range merged {
		if !in.OffersService(feature.Service) {
			continue
		}
		kinds := feature.NodeTypes
		offered := func(kind string) bool {
			return !slices.Contains(kinds, kind) || in.Offers(feature.Service, kind)
		}
		feature.NodeTypes = keepIf(feature.NodeTypes, func(kind string) bool { return in.Offers(feature.Service, kind) })
		feature.Composers = keepIf(feature.Composers, func(kind string) bool { return in.Offers(feature.Service, kind) })
		var permissions []entities.AnahitaNodeAddPermission
		for _, each := range feature.AddPermissions {
			if offered(each.Entity) {
				permissions = append(permissions, each)
			}
		}
		feature.AddPermissions = permissions
		kept = append(kept, feature)
	}
	return kept
}

func keepIf(list []string, keep func(string) bool) []string {
	var kept []string
	for _, each := range list {
		if keep(each) {
			kept = append(kept, each)
		}
	}
	return kept
}

// The kinds of profile, by their short names and their types, in the order
// they are said.
var profileKinds = []struct{ Name, Type string }{
	{"person", constants.NodeActorPersonEntityType},
	{"group", constants.NodeActorGroupEntityType},
	{"event", constants.NodeActorEventEntityType},
	{"website", constants.NodeActorWebsiteEntityType},
}

// Offered is what the installation offers, for ReadInstance: the kinds of
// profile that may be made, and every kind of post one of them may write,
// as each kind's features say once cut to the site (ForSite).
func (fl *FeatureLoader) Offered() (profiles, posts []string, err error) {
	instance, err := fl.Instance()
	if err != nil {
		return nil, nil, err
	}
	for _, kind := range profileKinds {
		if !instance.OffersProfile(kind.Name) {
			continue
		}
		profiles = append(profiles, kind.Name)
		features, err := fl.Load(kind.Type)
		if err != nil {
			return nil, nil, err
		}
		for _, feature := range fl.ForSite(features) {
			for _, post := range feature.NodeTypes {
				if !slices.Contains(posts, post) {
					posts = append(posts, post)
				}
			}
		}
	}
	return profiles, posts, nil
}
