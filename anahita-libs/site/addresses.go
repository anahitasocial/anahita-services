// Package site says where this installation is, as the rest of the internet
// sees it: its domain, and the addresses of its web app and of its API.
//
// Every address that is told to another server is made here, from one
// reading of the settings, because such an address is kept by whoever was
// told it. A person's address on another server is a URL on this one; if two
// services disagreed about the host, or it changed between two starts, the
// person would be two people there, or nobody.
package site

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"anahita-libs/constants"
)

// Addresses is where the installation is.
type Addresses struct {
	// Domain is the part of a handle after the second @: ana@example.org. It
	// is the host people know the site by, which is the web app's.
	Domain string
	// WebURL is the web app's address, with no slash at its end. What a
	// person is sent to in order to look at something.
	WebURL string
	// APIURL is the API's address, with no slash at its end. What another
	// server is sent to in order to fetch something.
	APIURL string
}

// Problems with the addresses that stop an installation being spoken of to
// other servers. None of them stops it running.
var (
	ErrNoDomain  = errors.New("the installation has no domain: set DOMAIN_NAME or PUBLIC_DOMAIN")
	ErrNotPublic = errors.New("the installation's addresses are this machine's own, which no other server can reach")
	ErrNotSecure = errors.New("other servers are only spoken to over https")
)

// FromEnv reads the installation's addresses from its settings.
//
//   - PUBLIC_DOMAIN, PUBLIC_WEB_URL and PUBLIC_API_URL say each outright.
//   - Without them, each is worked out from DOMAIN_NAME and ENV as
//     auth-service works out its own: example.org, https://example.org and
//     https://api.example.org on a live site, the same under staging. on a
//     staging one, and this machine in development.
//   - FRONTEND_URL, which auth-service already goes by for the web app, is
//     gone by here too when PUBLIC_WEB_URL is not set.
func FromEnv() Addresses {
	return from(os.Getenv)
}

func from(getenv func(string) string) Addresses {
	env := getenv("ENV")
	name := strings.ToLower(strings.TrimSpace(getenv("DOMAIN_NAME")))

	// What the three are when nothing says otherwise.
	var host, web, api string
	switch {
	case name == "":
	case env == constants.ConfigEnvProd:
		host, web, api = name, "https://"+name, "https://api."+name
	case env == constants.ConfigEnvStaging:
		host, web, api = "staging."+name, "https://staging."+name, "https://api.staging."+name
	}
	if host == "" {
		host, web, api = "localhost", "http://localhost:3000", "http://localhost"
	}

	addresses := Addresses{
		WebURL: first(clean(getenv("PUBLIC_WEB_URL")), clean(getenv("FRONTEND_URL")), web),
		APIURL: first(clean(getenv("PUBLIC_API_URL")), api),
	}

	// The domain follows the web app when the web app was moved and the
	// domain not said: a handle is the name people already know the site by.
	addresses.Domain = strings.ToLower(strings.TrimSpace(getenv("PUBLIC_DOMAIN")))
	if addresses.Domain == "" {
		addresses.Domain = host
		if moved, err := url.Parse(addresses.WebURL); err == nil && moved.Hostname() != "" && addresses.WebURL != web {
			addresses.Domain = strings.TrimPrefix(strings.ToLower(moved.Hostname()), "www.")
		}
	}

	return addresses
}

// Web is the address of a page of the web app. The path begins with a slash.
func (a Addresses) Web(path string) string {
	return a.WebURL + path
}

// API is the address of something the API serves. The path begins with a
// slash.
func (a Addresses) API(path string) string {
	return a.APIURL + path
}

// Handle is what a person here is called from anywhere: ana@example.org.
func (a Addresses) Handle(alias string) string {
	return alias + "@" + a.Domain
}

// IsOurs says whether an address is one of this installation's, by its host.
// For telling what was made here from what came from elsewhere.
func (a Addresses) IsOurs(address string) bool {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" {
		return false
	}
	for _, mine := range []string{a.APIURL, a.WebURL} {
		ours, err := url.Parse(mine)
		if err == nil && strings.EqualFold(ours.Host, parsed.Host) {
			return true
		}
	}
	return false
}

// CanFederate says whether other servers could reach this installation at
// its addresses, and what is wrong when they could not.
//
// They have to: a server that is told of a person here fetches that person
// from the address it was given, and writes back to it. So the addresses
// must be names on the internet, over https. A site on a developer's machine
// is neither, and is spoken of to nobody.
func (a Addresses) CanFederate() error {
	if a.Domain == "" {
		return ErrNoDomain
	}
	if local(a.Domain) {
		return fmt.Errorf("%w: %s", ErrNotPublic, a.Domain)
	}
	for _, address := range []string{a.WebURL, a.APIURL} {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Hostname() == "" {
			return fmt.Errorf("%w: %q", ErrNotPublic, address)
		}
		if local(parsed.Hostname()) {
			return fmt.Errorf("%w: %s", ErrNotPublic, address)
		}
		if parsed.Scheme != "https" {
			return fmt.Errorf("%w: %s", ErrNotSecure, address)
		}
	}
	return nil
}

// local says whether a host is this machine or a network's own.
func local(host string) bool {
	host = strings.ToLower(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || !strings.Contains(host, ".") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
	}
	return false
}

// clean is an address as it is kept: nothing around it, no slash at its end.
func clean(address string) string {
	return strings.TrimRight(strings.TrimSpace(address), "/")
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
