package geocoding

import (
	"strings"
	"testing"
)

func TestGeocoderURL(t *testing.T) {
	const fallback = "https://public.example.org"

	cases := []struct {
		what    string
		value   string
		want    string
		enabled bool
		wantErr bool
	}{
		{"unset uses the public instance", "", fallback, true, false},
		{"an installation's own instance", "https://photon.internal.example.com", "https://photon.internal.example.com", true, false},
		{"plain http inside a private network", "http://photon:2322", "http://photon:2322", true, false},
		{"a trailing slash is dropped", "https://geo.example.com/", "https://geo.example.com", true, false},
		{"off leaves the geocoder out", "off", "", false, false},
		{"OFF too", "OFF", "", false, false},
		{"a host with no scheme", "photon.example.com", "", false, true},
		{"another scheme", "ftp://photon.example.com", "", false, true},
		{"a word that is not off", "none", "", false, true},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			t.Setenv(PhotonURLEnv, c.value)

			got, enabled, err := geocoderURL(PhotonURLEnv, fallback)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, want an error: %v", err, c.wantErr)
			}
			if got != c.want || enabled != c.enabled {
				t.Errorf("url = %q, enabled = %v; want %q, %v", got, enabled, c.want, c.enabled)
			}
		})
	}
}

// The public Nominatim asks every caller to say who they are. An installation
// that does not set a contact is identified as the Anahita project, which is
// not who is sending the requests.
func TestTheUserAgentNamesTheConfiguredContact(t *testing.T) {
	t.Setenv(ContactEnv, "")
	if got := geocoderUserAgent(); !strings.Contains(got, defaultContact) {
		t.Errorf("user agent = %q, want the default contact", got)
	}

	t.Setenv(ContactEnv, "admin@example.org")
	got := geocoderUserAgent()
	if !strings.Contains(got, "admin@example.org") || strings.Contains(got, defaultContact) {
		t.Errorf("user agent = %q, want the configured contact only", got)
	}
}
