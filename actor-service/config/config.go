package config

import (
	"anahita-libs/constants"
	"anahita-libs/getenv"
	"anahita-libs/settings"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrInvalidGroupsFrom   = "GROUPS_FROM must be one of: "
	ErrInvalidEventsFrom   = "EVENTS_FROM must be one of: "
	ErrInvalidChannelsFrom = "CHANNELS_FROM must be one of: "
)

// Config is what actor-service is told from its environment.
//
// There is no database address here, on purpose. The service reads and
// writes profiles through graph-grpc-service and owns no tables, so it is
// given no database credentials to lose.
type Config struct {
	settings.Common

	GroupsFrom string
	// Who may make an event of their own. See GetEventsFrom.
	EventsFrom string
	// Who may make a channel (roadmap row 147). See GetChannelsFrom.
	ChannelsFrom string
	// The web app's address, for links that leave the site: the one in a
	// calendar file. Optional; without it the link is left out.
	WebOrigin string
}

func NewConfig() *Config {
	config := &Config{
		Common:     settings.ReadCommon(),
		GroupsFrom: os.Getenv("GROUPS_FROM"),
		EventsFrom: os.Getenv("EVENTS_FROM"),
		// WEBSITES_FROM, its old name, still read for one release.
		ChannelsFrom: getenv.Renamed("CHANNELS_FROM", "WEBSITES_FROM"),
		WebOrigin:    strings.TrimRight(os.Getenv("WEB_ORIGIN"), "/"),
	}

	return config
}

func (config *Config) Validate() error {
	missingFields := config.Missing(settings.CookieKey, settings.Amqp, settings.FileService,
		settings.GraphService, settings.OAuthIssuer,
		settings.OAuthJWKS)

	if len(missingFields) > 0 {
		return errors.New("missing fields: " + strings.Join(missingFields, ", "))
	}

	// Checked against the raw value, not the resolved one: unset is legal and
	// means the default, but a value nobody recognises is a typo, and quietly
	// falling back to administrators would hide it. Unknown levels rank above
	// everything, so booting on one would refuse every group creation with no
	// hint as to why.
	if config.GroupsFrom != "" && !constants.IsRoleLevel(constants.NormalizeRoleLevel(config.GroupsFrom)) {
		return errors.New(ErrInvalidGroupsFrom + strings.Join(constants.RoleLevelValues, ", "))
	}

	// And for channels (roadmap row 147).
	if config.ChannelsFrom != "" && !constants.IsRoleLevel(constants.NormalizeRoleLevel(config.ChannelsFrom)) {
		return errors.New(ErrInvalidChannelsFrom + strings.Join(constants.RoleLevelValues, ", "))
	}

	// The same for events: a value nobody recognises is a typo, and would
	// refuse every event with no hint as to why.
	if config.EventsFrom != "" && !constants.IsRoleLevel(constants.NormalizeRoleLevel(config.EventsFrom)) {
		return errors.New(ErrInvalidEventsFrom + strings.Join(constants.RoleLevelValues, ", "))
	}

	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

// GetWebOrigin returns the web app's address with no slash at its end, or
// "" when it has not been set.
func (config *Config) GetWebOrigin() string {
	return config.WebOrigin
}

// GetGroupsFrom returns the minimum role that may create a group.
//
// Defaults to administrators, which is what happened before this setting
// existed — so no installation changes behaviour by upgrading into it.
func (config *Config) GetGroupsFrom() string {
	value := constants.NormalizeRoleLevel(config.GroupsFrom)
	if value == "" {
		return constants.RoleLevelAdministrators
	}

	return value
}

// GetChannelsFrom returns the minimum role that may make a channel, as
// syndication-service and auth-service read it: administrators unless the
// installation says otherwise.
func (config *Config) GetChannelsFrom() string {
	value := constants.NormalizeRoleLevel(config.ChannelsFrom)
	if value == "" {
		return constants.RoleLevelAdministrators
	}
	return value
}

// GetEventsFrom returns the minimum role that may make an event of their
// own.
//
// Defaults to registered: anybody signed in. A group is a standing place
// with a name others will look for, which is why making one is kept to
// administrators until an installation says otherwise. An event is an
// evening, and an installation where only administrators can hold one would
// have none. One that wants them kept sets EVENTS_FROM.
//
// An event made for a group is not this question: that is for the group's
// administrators, whatever is set here.
func (config *Config) GetEventsFrom() string {
	value := constants.NormalizeRoleLevel(config.EventsFrom)
	if value == "" {
		return constants.RoleLevelRegistered
	}

	return value
}

// MayCreateGroup reports whether somebody of this person type meets the
// configured minimum. Compared by rank, never by equality — see
// constants.MeetsRoleLevel.
func (config *Config) MayCreateGroup(personType string) bool {
	return constants.MeetsRoleLevel(personType, config.GetGroupsFrom())
}
