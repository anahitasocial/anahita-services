package middleware

import "github.com/labstack/echo/v4"

// ClientIPExtractor decides what c.RealIP() returns. Every service sets it
// (e.IPExtractor), so that none reads the address a client typed.
//
// Echo's default returns the FIRST X-Forwarded-For entry, the one the client
// supplied. In auth-service that value keys the IP-side throttles on password
// reset and signup and is recorded as the source address in the sign-in log,
// so a forged header bought an unlimited budget on an endpoint that sends
// mail, and a forgeable log.
//
// ExtractIPFromXFFHeader appends the direct peer to the header's list and
// walks it right to left, returning the first address it does not trust. Every
// proxy appends what it saw (nginx.conf: $proxy_add_x_forwarded_for), so
// anything a client wrote sits to the LEFT of the truth and is never reached.
//
// The default trust set (loopback, link-local, private) fits both ways this
// runs: behind nginx in docker-compose, and behind an ingress in a cluster,
// where every hop is a private address.
//
// Accepted, knowingly: a client whose own address is private (an internal or
// VPN-only caller) is trusted and skipped, so an entry forged to its left
// would be returned. Closing that means naming the proxy ranges exactly, which
// differ per installation. Revisit if this is ever reachable from inside.
func ClientIPExtractor() echo.IPExtractor {
	return echo.ExtractIPFromXFFHeader()
}
