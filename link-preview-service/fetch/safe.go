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
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"anahita-libs/safehttp"
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
	ErrNotAllowed = safehttp.ErrNotAllowed
	// ErrNotAPage is an answer that is not a page: another kind of file, an
	// error, too many redirects.
	ErrNotAPage = errors.New("not a page")
)

// Which addresses are public, and the client that goes only to them, are
// shared with the service that speaks to other servers: anahita-libs/safehttp.

// PublicAddress reports whether an address is somewhere on the open
// internet, on a port the web uses.
func PublicAddress(ip net.IP, port string) bool { return safehttp.PublicAddress(ip, port) }

// Allow decides whether a connection to this address may be made.
type Allow = safehttp.Allow

// newClient builds the client everything is fetched with: the page, every
// redirect and the picture alike go only where `allow` lets them.
func newClient(allow Allow) *http.Client {
	client := safehttp.NewClient(safehttp.Options{Allow: allow, Timeout: Timeout, MaxRedirects: MaxRedirects})

	check := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		err := check(req, via)
		if errors.Is(err, safehttp.ErrTooManyRedirects) {
			return fmt.Errorf("%w: too many redirects", ErrNotAPage)
		}
		return err
	}
	return client
}
