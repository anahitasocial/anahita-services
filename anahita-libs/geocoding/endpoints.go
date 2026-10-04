package geocoding

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// The two geocoders, and where each is reached. Both are open-source and can
// be run by an installation itself, so their addresses are settings. The
// defaults are the public instances, which are shared services with strict
// usage limits and which see every address a member types.
//
//	GEOCODER_PHOTON_URL     default https://photon.komoot.io
//	GEOCODER_NOMINATIM_URL  default https://nominatim.openstreetmap.org
//	GEOCODER_CONTACT        an email address or URL, sent in the User-Agent
//
// "off" in place of an address leaves that geocoder out, for an installation
// that runs one of them and does not want the other consulted. At least one
// has to stay: a place cannot be saved without coordinates.
const (
	PhotonURLEnv    = "GEOCODER_PHOTON_URL"
	NominatimURLEnv = "GEOCODER_NOMINATIM_URL"
	ContactEnv      = "GEOCODER_CONTACT"

	geocoderOff = "off"

	// defaultContact identified every installation as the Anahita project
	// itself. Nominatim's usage policy asks for a contact for whoever is
	// actually sending the requests, so an installation should set its own.
	defaultContact = "contact@anahita.io"
)

// geocoderURL resolves one geocoder's base URL. The bool is false when the
// geocoder is switched off.
func geocoderURL(env, fallback string) (string, bool, error) {
	value := strings.TrimSpace(os.Getenv(env))

	switch {
	case value == "":
		return fallback, true, nil
	case strings.EqualFold(value, geocoderOff):
		return "", false, nil
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false, fmt.Errorf("%s must be an http(s) URL or %q, not %q", env, geocoderOff, value)
	}

	return strings.TrimRight(value, "/"), true, nil
}

// geocoderUserAgent is what the geocoders are told about who is asking.
func geocoderUserAgent() string {
	contact := strings.TrimSpace(os.Getenv(ContactEnv))
	if contact == "" {
		contact = defaultContact
	}
	return fmt.Sprintf("Anahita-Services/1.0 (%s)", contact)
}
