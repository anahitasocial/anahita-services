// Package fetch reads a page somebody linked to, and its picture, off the
// open internet.
//
// THE DANGER IT IS BUILT AROUND: this service is told an address by anybody
// signed in and goes to it, from inside the installation's network. Left
// alone it would fetch the cloud provider's metadata address, a database's
// admin page, another service's port: whatever its network can reach and
// the person asking cannot. So it goes only to public addresses, on the web's
// two ports, and decides that at the moment it connects.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// What a fetch is held to.
const (
	Timeout      = 5 * time.Second
	MaxRedirects = 3
	// The most of a page that is read: its head is at the top.
	MaxPageBytes = 1 << 20
	// The most a picture may weigh as it comes, and the most pixels it may
	// say it has.
	MaxImageBytes  = 5 << 20
	MaxImagePixels = 40_000_000
	// How wide the picture kept is, at most.
	ThumbnailWidth = 800
)

var (
	// ErrNotAllowed is an address this service does not go to.
	ErrNotAllowed = errors.New("address not allowed")
	// ErrNotAPage is an answer that is not a page: another kind of file, an
	// error, too many redirects.
	ErrNotAPage = errors.New("not a page")
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

// newClient builds the client everything is fetched with.
//
// The check is in the dialer's Control, which the operating system's
// connect goes through: it is given the address actually being connected
// to, for the page, for every redirect and for the picture alike. Nothing
// is fetched through a proxy, which would make that address the proxy's.
func newClient(allow Allow) *http.Client {
	dialer := &net.Dialer{
		Timeout: Timeout,
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
		TLSHandshakeTimeout:    Timeout,
		ResponseHeaderTimeout:  Timeout,
		MaxResponseHeaderBytes: 64 << 10,
		// One page, one connection: nothing is kept open to a stranger.
		DisableKeepAlives: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > MaxRedirects {
				return fmt.Errorf("%w: too many redirects", ErrNotAPage)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: redirect to %s", ErrNotAllowed, req.URL.Scheme)
			}
			return nil
		},
	}
}
