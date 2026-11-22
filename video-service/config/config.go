package config

import (
	"anahita-libs/getenv"
	"anahita-libs/settings"
	"anahita-libs/videos"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	settings.Common

	// The largest video taken, in bytes, and the longest, in seconds.
	VideoMaxBytes   int64
	VideoMaxSeconds int
	// A GIF's or an animated PNG's.
	AnimationMaxBytes   int64
	AnimationMaxSeconds int
	// Audio posts' limits (roadmap row 137).
	AudioMaxBytes   int64
	AudioMaxSeconds int
	// How many videos one person may send in a day.
	VideoDailyUploads int
}

func NewConfig() *Config {
	config := &Config{
		Common:              settings.ReadCommon(),
		VideoMaxBytes:       videos.MaxBytes(),
		VideoMaxSeconds:     videos.MaxSeconds(),
		AnimationMaxBytes:   videos.AnimationMaxBytes(),
		AnimationMaxSeconds: videos.AnimationMaxSeconds(),
		AudioMaxBytes:       videos.AudioMaxBytes(),
		AudioMaxSeconds:     videos.AudioMaxSeconds(),
		VideoDailyUploads:   getenv.Int("VIDEO_DAILY_UPLOADS", 20),
	}

	return config
}

func (config *Config) Validate() error {
	missingFields := config.Missing(settings.CookieKey, settings.Database, settings.FileService,
		settings.GraphService, settings.OAuthIssuer, settings.OAuthJWKS)

	if len(missingFields) > 0 {
		return errors.New("missing fields: " + strings.Join(missingFields, ", "))
	}

	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

// ValidateWorker is what the worker needs, which is less than the API: it
// answers nobody and reads no tokens.
func (config *Config) ValidateWorker() error {
	missing := config.Missing(settings.Database, settings.FileService, settings.GraphService)
	if len(missing) > 0 {
		return errors.New("missing fields: " + strings.Join(missing, ", "))
	}
	return nil
}

// Heights reads VIDEO_RENDITIONS, "1080,720,360": the sizes a video is made
// in, by height, up to 2160 (4K). A video is never made larger than it is.
func Heights() []int {
	var heights []int
	for _, part := range strings.Split(getenv.String("VIDEO_RENDITIONS", "1080,720,360"), ",") {
		height, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && height >= 144 && height <= 2160 {
			heights = append(heights, height)
		}
	}
	if len(heights) == 0 {
		heights = []int{1080, 720, 360}
	}
	return heights
}

// GetDomainName is the site's domain, which the session store is built
// with. Only its counter is used here, to limit downloads.
func (config *Config) GetDomainName() string {
	return os.Getenv("DOMAIN_NAME")
}
