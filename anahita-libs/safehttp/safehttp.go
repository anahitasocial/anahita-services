// Package safehttp is an HTTP client for going where somebody else said to.
//
// THE DANGER IT IS BUILT AROUND: a service that is told an address by
// anybody, and goes to it, goes from inside the installation's network.
// Left alone it would fetch the cloud provider's metadata address, a
// database's admin page, another service's port: whatever its network can
// reach and the one who named the address cannot. So a client made here
// goes only to public addresses, on the web's two ports, and decides that
// at the moment it connects.
//
// Two services are told where to go by strangers: the one that reads the
// page a link points to, and the one that speaks to other servers of the
// fediverse. Both fetch with this.
package safehttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

var (
	// ErrNotAllowed is an address that is not gone to.
	ErrNotAllowed = errors.New("address not allowed")
	// ErrTooManyRedirects is being sent on more often than is followed.
	ErrTooManyRedirects = errors.New("too many redirects")
)

// Nets that are nobody's public address. The standard library names most of
// what matters (loopback, private, link-local, multicast, unspecified);
// these are the rest.
var notPublic = mustParse(
	"0.0.0.0/8",       // "this network"
	"100.64.0.0/10",   // carrier-grade NAT
	"192.0.0.0/24",    // protocol assignments
	"192.0.2.0/24",    // documentation
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // documentation
	"203.0.113.0/24",  // documentation
	"240.0.0.0/4",     // reserved
	"64:ff9b::/96",    // NAT64: an IPv4 address in disguise
	"2001:db8::/32",   // documentation
)

func mustParse(cidrs ...string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		nets = append(nets, network)
	}
	return nets
}

// PublicAddress reports whether an address is somewhere on the open
// internet, on a port the web uses.
//
// This is asked of the address a connection is about to be made to, after
// the name has been looked up. Asking it of the name beforehand would be
// asking about a different lookup: a name can answer with a public address
// the first time and 127.0.0.1 the second.
func PublicAddress(ip net.IP, port string) bool {
	if port != "80" && port != "443" {
		return false
	}
	if ip == nil {
		return false
	}

	// An IPv4 address written as IPv6 is judged as the IPv4 address it is.
	if four := ip.To4(); four != nil {
		ip = four
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || !ip.IsGlobalUnicast() {
		return false
	}

	for _, network := range notPublic {
		if network.Contains(ip) {
			return false
		}
	}

	return true
}

// Allow decides whether a connection to this address may be made.
type Allow func(ip net.IP, port string) bool

// Options is what a client is held to.
type Options struct {
	// Allow decides each connection. PublicAddress unless said otherwise.
	Allow Allow
	// Timeout is for the whole of one fetch, and for each step of it.
	Timeout time.Duration
	// MaxRedirects is how often being sent on is followed. 0 for never:
	// the caller is then given the answer that sends it on.
	MaxRedirects int
}

// NewClient builds a client that goes only where Allow lets it.
//
// The check is in the dialer's Control, which the operating system's
// connect goes through: it is given the address actually being connected
// to, for the first request and for every redirect alike. Nothing is
// fetched through a proxy, which would make that address the proxy's.
func NewClient(options Options) *http.Client {
	allow := options.Allow
	if allow == nil {
		allow = PublicAddress
	}

	dialer := &net.Dialer{
		Timeout: options.Timeout,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return ErrNotAllowed
			}
			if !allow(net.ParseIP(host), port) {
				return fmt.Errorf("%w: %s", ErrNotAllowed, address)
			}
			return nil
		},
	}

	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, address)
		},
		TLSHandshakeTimeout:    options.Timeout,
		ResponseHeaderTimeout:  options.Timeout,
		MaxResponseHeaderBytes: 64 << 10,
		// One fetch, one connection: nothing is kept open to a stranger.
		DisableKeepAlives: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   options.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if options.MaxRedirects <= 0 {
				return http.ErrUseLastResponse
			}
			if len(via) > options.MaxRedirects {
				return ErrTooManyRedirects
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: redirect to %s", ErrNotAllowed, req.URL.Scheme)
			}
			return nil
		},
	}
}
