package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/middleware"
	"auth/messages"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
)

// getViewer returns the viewer from the context
func getViewer(c echo.Context) *entities.Viewer {
	return middleware.GetViewer(c)
}

// getClientIP is the address the request came from.
//
// It is c.RealIP(), which is only trustworthy because main() installs an
// IPExtractor that walks X-Forwarded-For from the right. This used to read the
// header itself and take the FIRST entry, which is whatever the client typed:
// the IP-keyed throttles on password reset and signup could be reset by
// changing a header, and the address in the sign-in log could be forged.
func getClientIP(c echo.Context) string {
	return c.RealIP()
}

// The lookup runs synchronously while a token is being issued, so it must not
// be able to hold that request open. Without a timeout an unresponsive
// provider turned somebody else's bad day into a sign-in outage here. Three
// seconds is generous for something whose whole output is two cosmetic strings.
//
// Package-level so tests can point both at a local server.
var (
	ipLocationClient = &http.Client{Timeout: 3 * time.Second}

	ipLocationBaseURL = "https://ipapi.co"
)

const ipLocationUserAgent = "ipapi.co/#go-v1.5"

// Carrier-grade NAT (RFC 6598). Not private in Go's sense, but as unplaceable,
// and mobile networks put real people behind it.
var _, cgnatRange, _ = net.ParseCIDR("100.64.0.0/10")

// isNonRoutableIP reports whether no geolocation service can place an address.
//
// net.IP's own predicates rather than string prefixes: HasPrefix(ip, "172.")
// claimed all of 172.0.0.0/8 when only 172.16.0.0/12 is private, so a public
// address such as 172.32.5.9 was labelled local and never looked up.
func isNonRoutableIP(parsed net.IP) bool {
	if parsed.IsLoopback() ||
		parsed.IsPrivate() ||
		parsed.IsLinkLocalUnicast() ||
		parsed.IsLinkLocalMulticast() ||
		parsed.IsUnspecified() {
		return true
	}

	return cgnatRange.Contains(parsed)
}

// getIPLocation resolves an address to a city and country through ipapi.co.
//
// Every failure returns the zero IPLocation, which the caller reads as "leave
// the unknown-city placeholders alone". Nothing here is worth failing a
// sign-in over: the result decorates a row in the sign-in log.
func getIPLocation(ip string) IPLocation {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		// Absent or malformed. That is not a place, so do not claim it is
		// local: a malformed value means something upstream is wrong.
		return IPLocation{}
	}

	// Before reaching for the network. The provider cannot place a private
	// address and answers with an error, and on a developer's machine every
	// address is the docker bridge.
	if isNonRoutableIP(parsed) {
		return IPLocation{
			City:    messages.AuthLoginLocalCity,
			Country: messages.AuthLoginLocalCountry,
		}
	}

	endpoint := fmt.Sprintf("%s/%s/json/", ipLocationBaseURL, url.PathEscape(parsed.String()))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		log.Println("iplocation: build request failed:", err)
		return IPLocation{}
	}
	req.Header.Set("User-Agent", ipLocationUserAgent)

	resp, err := ipLocationClient.Do(req)
	if err != nil {
		// Return, do not fall through. This used to log and carry on into
		// `defer resp.Body.Close()` on a nil response, so any network blip
		// reaching the provider panicked the token endpoint.
		log.Println("iplocation: lookup failed:", err)
		return IPLocation{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("iplocation: unexpected status:", resp.Status)
		return IPLocation{}
	}

	// A typed struct rather than a map. The old code asserted
	// json_map["city"].(string) unchecked, and the provider answers with a
	// JSON null for city on plenty of real addresses, datacentres especially.
	var body struct {
		Error   bool   `json:"error"`
		Reason  string `json:"reason"`
		City    string `json:"city"`
		Country string `json:"country_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		log.Println("iplocation: decode failed:", err)
		return IPLocation{}
	}

	if body.Error {
		log.Println("iplocation: provider error:", body.Reason)
		return IPLocation{}
	}

	return IPLocation{City: body.City, Country: body.Country}
}
