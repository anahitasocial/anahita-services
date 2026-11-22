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
	ErrInvalidChannelsFrom = "CHANNELS_FROM must be one of: "
)

// Config is what syndication-service is told from its environment. The API and
// the worker read the same; each checks what it needs.
type Config struct {
	settings.Common

	DomainName string
	// Who may connect a channel. See GetChannelsFrom.
	ChannelsFrom string

	// The worker's. Where link-preview-service is asked for the card of an
	// item's original; unset, articles have no card.
	LinkPreviewURL string
	// The most posts a channel makes in an hour (CHANNEL_FEED_HOURLY_CAP, 10), and
	// how often a feed is read, in minutes (CHANNEL_FEED_READ_EVERY, 30).
	HourlyCap int
	ReadEvery int
}

func NewConfig() *Config {
	config := &Config{
		Common:     settings.ReadCommon(),
		DomainName: os.Getenv("DOMAIN_NAME"),
		// WEBSITES_FROM, WEBSITE_HOURLY_CAP and WEBSITE_READ_EVERY until
		// roadmap row 147, read for one release.
		ChannelsFrom:   getenv.Renamed("CHANNELS_FROM", "WEBSITES_FROM"),
		LinkPreviewURL: os.Getenv("LINK_PREVIEW_URL"),
		HourlyCap:      getenv.RenamedInt("CHANNEL_FEED_HOURLY_CAP", "WEBSITE_HOURLY_CAP", 10),
		ReadEvery:      getenv.RenamedInt("CHANNEL_FEED_READ_EVERY", "WEBSITE_READ_EVERY", 30),
	}

	return config
}

// Validate is what the API needs.
func (config *Config) Validate() error {
	missing := config.Missing(settings.CookieKey, settings.Database, settings.GraphService,
		settings.OAuthIssuer, settings.OAuthJWKS)

	if len(missing) > 0 {
		return errors.New("missing fields: " + strings.Join(missing, ", "))
	}

	// A value nobody recognises is a typo, and would refuse every channel with
	// no hint as to why.
	if config.ChannelsFrom != "" && !constants.IsRoleLevel(constants.NormalizeRoleLevel(config.ChannelsFrom)) {
		return errors.New(ErrInvalidChannelsFrom + strings.Join(constants.RoleLevelValues, ", "))
	}

	return nil
}

// ValidateWorker is what the worker needs: the database and the graph.
func (config *Config) ValidateWorker() error {
	missing := config.Missing(settings.Database, settings.GraphService)
	if len(missing) > 0 {
		return errors.New("missing fields: " + strings.Join(missing, ", "))
	}
	if config.HourlyCap < 1 || config.ReadEvery < 15 {
		return errors.New("CHANNEL_FEED_HOURLY_CAP must be at least 1, and CHANNEL_FEED_READ_EVERY at least 15 minutes")
	}
	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

func (config *Config) GetDomainName() string {
	return config.DomainName
}

// GetChannelsFrom is the least role that may connect a channel.
//
// Administrators unless an installation says otherwise, as for groups: a
// Channel is a standing profile with a name others will look for, and posts
// to the fediverse in the installation's name.
func (config *Config) GetChannelsFrom() string {
	value := constants.NormalizeRoleLevel(config.ChannelsFrom)
	if value == "" {
		return constants.RoleLevelAdministrators
	}
	return value
}

// MayConnect says whether somebody of this person type meets it. Compared
// by rank, never by equality: see constants.MeetsRoleLevel.
func (config *Config) MayConnect(personType string) bool {
	return constants.MeetsRoleLevel(personType, config.GetChannelsFrom())
}
