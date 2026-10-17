// Package domains proves that somebody controls a domain.
//
// A Pulse is only made for a website whose owner proves the domain is
// theirs (docs/plans/pulse.md), and people and groups are to use the same
// proof later. The proof is a token given here, put in one of two places
// only the domain's owner can write:
//
//	a TXT record      _anahita.<domain>, holding the token
//	a file            https://<host>/.well-known/anahita-verification.txt,
//	                  holding the token: one committed file for a static site
//
// Either one is enough. The proof is checked again every day; a domain whose
// proof is gone is said to be failing, and what rests on it decides how long
// to wait.
package domains

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"anahita-libs/safehttp"
)

const (
	// RecordPrefix is the name the TXT record is put under, before the
	// domain.
	RecordPrefix = "_anahita."
	// FilePath is where the file is put on the site.
	FilePath = "/.well-known/anahita-verification.txt"

	// How long each check may take, and the most of the file read.
	checkTimeout = 5 * time.Second
	maxFile      = 4 << 10
	userAgent    = "Anahita/1.0"
)

// How a domain was proved.
const (
	MethodDNS  = "dns"
	MethodFile = "file"
)

var (
	// ErrNotADomain is an address with no domain a person could own: an IP
	// address, a name with no dot, localhost.
	ErrNotADomain = errors.New("not a domain")
	// ErrNotFound is neither place holding the token.
	ErrNotFound = errors.New("the token is in neither place")
)

// NewToken is a token to prove a domain with: 32 lowercase hexadecimal
// characters, prefixed so that it says what it is in a DNS zone.
func NewToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "anahita-" + hex.EncodeToString(b), nil
}

// Site is a website's address reduced to what is proved: its host, as the
// file is fetched from, and its domain, the host without a leading "www.",
// where the record goes.
type Site struct {
	Host   string
	Domain string
}

// SiteOf reads a website's address, with or without its scheme.
func SiteOf(address string) (Site, error) {
	address = strings.TrimSpace(address)
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return Site{}, ErrNotADomain
	}

	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || net.ParseIP(host) != nil || !strings.Contains(host, ".") ||
		host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return Site{}, ErrNotADomain
	}

	return Site{Host: host, Domain: strings.TrimPrefix(host, "www.")}, nil
}

// Covers says whether an address is on this domain or under it: a feed at
// example.com, www.example.com or blog.example.com is example.com's; one at
// feeds.example.net is not.
func (s Site) Covers(address string) bool {
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	return host == s.Domain || strings.HasSuffix(host, "."+s.Domain)
}

// RecordName is where the TXT record goes.
func (s Site) RecordName() string {
	return RecordPrefix + s.Domain
}

// FileURL is where the file goes.
func (s Site) FileURL() string {
	return "https://" + s.Host + FilePath
}

// Resolver looks up TXT records. net.Resolver is one.
type Resolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// Checker looks for a token in both places.
type Checker struct {
	Resolver Resolver
	Client   *http.Client
}

// NewChecker looks things up with the system's resolver and fetches with a
// client that goes only to public addresses.
func NewChecker() *Checker {
	return &Checker{
		Resolver: net.DefaultResolver,
		Client:   safehttp.NewClient(safehttp.Options{Timeout: checkTimeout}),
	}
}

// Check says where the token was found: MethodDNS or MethodFile. The record
// is asked first, since it costs nothing to the site. ErrNotFound when it is
// in neither.
func (c *Checker) Check(ctx context.Context, site Site, token string) (string, error) {
	if c.inRecord(ctx, site, token) {
		return MethodDNS, nil
	}
	if c.inFile(ctx, site, token) {
		return MethodFile, nil
	}
	return "", ErrNotFound
}

func (c *Checker) inRecord(ctx context.Context, site Site, token string) bool {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	records, err := c.Resolver.LookupTXT(ctx, site.RecordName())
	if err != nil {
		return false
	}
	for _, record := range records {
		if holds(record, token) {
			return true
		}
	}
	return false
}

func (c *Checker) inFile(ctx context.Context, site Site, token string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, site.FileURL(), nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/plain")

	res, err := c.Client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false
	}

	lines := bufio.NewScanner(io.LimitReader(res.Body, maxFile))
	for lines.Scan() {
		if holds(lines.Text(), token) {
			return true
		}
	}
	return false
}

// holds says whether a record or a line of the file is the token: as it is,
// or quoted, as some DNS panels keep it.
func holds(text, token string) bool {
	text = strings.Trim(strings.TrimSpace(text), `"`)
	return token != "" && text == token
}
