package sessions

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"net/url"

	"anahita-libs/cors"

	"github.com/labstack/echo/v4"
)

// GenerateRandomString generates a random string of length n
func GenerateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}

// HashString hashes a string
func HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)
}

// ExtractValidatedHostname extracts and validates the hostname from the request context.
// It checks the Origin header for browser clients and falls back to a default for Electron clients.
// If no valid hostname can be determined, it returns an error.
func ExtractValidatedHostname(c echo.Context, config ConfigInterface) (string, error) {
	origin := c.Request().Header.Get("Origin")

	// 1. Origin provided (browser clients)
	if origin != "" {
		allowedOrigins := cors.GetAllowedOrigins(config)
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("unauthorized origin: %s", origin)
		}

		u, err := url.Parse(origin)
		if err != nil {
			return "", fmt.Errorf("invalid origin format: %v", err)
		}

		return u.Hostname(), nil
	}

	// 2. Fallback for Electron clients
	clientType := c.Request().Header.Get("X-Client-Type")
	if clientType == "electron" {
		domain := config.GetDomainName()

		switch {
		case config.IsProd():
			return fmt.Sprintf("main.%s", domain), nil
		case config.IsStaging():
			return fmt.Sprintf("main.staging.%s", domain), nil
		case config.IsDev():
			return "localhost", nil
		}
	}

	// 3. No origin or fallback available
	return "", errors.New("missing Origin header and unsupported client type")
}
