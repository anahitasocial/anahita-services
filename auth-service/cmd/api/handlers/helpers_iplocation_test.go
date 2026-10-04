package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"auth/messages"
)

// getIPLocation decorates a sign-in with a city and country. Nothing depends
// on the answer, but it runs while a token is being issued, so how it FAILS
// matters far more than what it returns. It used to panic on a network error
// and on a JSON null, and to wait forever on a provider that did not answer.

// stubIPLocation points the lookup at a test server and puts the real client
// and URL back afterwards.
func stubIPLocation(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	url, client := ipLocationBaseURL, ipLocationClient
	ipLocationBaseURL = server.URL
	ipLocationClient = &http.Client{Timeout: 200 * time.Millisecond}

	t.Cleanup(func() {
		ipLocationBaseURL, ipLocationClient = url, client
		server.Close()
	})
}

func TestIPLocationDoesNotLookUpAddressesNobodyCanPlace(t *testing.T) {
	stubIPLocation(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the provider was asked about %s", r.URL.Path)
	})

	local := IPLocation{City: messages.AuthLoginLocalCity, Country: messages.AuthLoginLocalCountry}

	for _, ip := range []string{
		"127.0.0.1", "::1", "10.1.2.3", "192.168.65.1",
		"172.16.0.1", "172.31.255.254", // the private part of 172/8
		"169.254.10.10", "fe80::1", "fd00::1", "0.0.0.0",
		"100.64.0.1", "100.127.255.254", // carrier-grade NAT
	} {
		if got := getIPLocation(ip); got != local {
			t.Errorf("%s = %+v, want it reported as local", ip, got)
		}
	}
}

// Only 172.16.0.0/12 is private. Matching on the prefix "172." called the
// whole of 172.0.0.0/8 local, so public addresses in it were never looked up.
func TestIPLocationLooksUpPublicAddressesIn172(t *testing.T) {
	asked := ""
	stubIPLocation(t, func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Path
		w.Write([]byte(`{"city":"Toronto","country_name":"Canada"}`))
	})

	got := getIPLocation("172.32.5.9")
	if asked != "/172.32.5.9/json/" {
		t.Errorf("the provider was asked %q, want /172.32.5.9/json/", asked)
	}
	if got != (IPLocation{City: "Toronto", Country: "Canada"}) {
		t.Errorf("location = %+v, want Toronto, Canada", got)
	}
}

func TestIPLocationFailuresReturnNothingAndDoNotPanic(t *testing.T) {
	cases := []struct {
		what    string
		handler http.HandlerFunc
	}{
		{"a JSON null for the city", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"city":null,"country_name":null}`))
		}},
		{"the provider's own error", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"error":true,"reason":"RateLimited"}`))
		}},
		{"an error with no reason", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"error":true}`))
		}},
		{"a body that is not JSON", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<html>upstream error</html>`))
		}},
		{"a server error", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}},
		{"a provider that does not answer in time", func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(500 * time.Millisecond)
		}},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			stubIPLocation(t, c.handler)
			if got := getIPLocation("203.0.113.9"); got != (IPLocation{}) {
				t.Errorf("location = %+v, want nothing", got)
			}
		})
	}
}

// The case that used to panic: the request never gets a response at all.
func TestIPLocationSurvivesAnUnreachableProvider(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()
	listener.Close() // nothing is listening there now

	url, client := ipLocationBaseURL, ipLocationClient
	ipLocationBaseURL = "http://" + address
	ipLocationClient = &http.Client{Timeout: 200 * time.Millisecond}
	t.Cleanup(func() { ipLocationBaseURL, ipLocationClient = url, client })

	if got := getIPLocation("203.0.113.9"); got != (IPLocation{}) {
		t.Errorf("location = %+v, want nothing", got)
	}
}

// A malformed address is not a place, and not local either. It means something
// upstream is wrong, and calling it localhost would hide that.
func TestIPLocationDoesNotCallAMalformedAddressLocal(t *testing.T) {
	stubIPLocation(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the provider was asked about %s", r.URL.Path)
	})

	for _, ip := range []string{"", "not-an-ip", "203.0.113.9/../../admin", "1.2.3"} {
		if got := getIPLocation(ip); got != (IPLocation{}) {
			t.Errorf("%q = %+v, want nothing", ip, got)
		}
	}
}

// With the lookup off, a public address is sent nowhere.
func TestIPLocationCanBeSwitchedOff(t *testing.T) {
	stubIPLocation(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the provider was asked about %s", r.URL.Path)
	})
	ipLocationBaseURL = "off"

	if got := getIPLocation("203.0.113.9"); got != (IPLocation{}) {
		t.Errorf("location = %+v, want nothing", got)
	}

	// A private address is still reported as local: that needs no lookup.
	if got := getIPLocation("10.0.0.1"); got.City != messages.AuthLoginLocalCity {
		t.Errorf("a private address = %+v, want it reported as local", got)
	}
}

func TestIPLocationURLFromEnv(t *testing.T) {
	for value, want := range map[string]string{
		"":                           "https://ipapi.co",
		"off":                        "off",
		"https://geoip.example.org/": "https://geoip.example.org",
	} {
		t.Setenv(IPLocationURLEnv, value)
		if got := ipLocationURLFromEnv(); got != want {
			t.Errorf("%q -> %q, want %q", value, got, want)
		}
	}
}
