package geocoding

import (
	"context"
	"crypto/tls"
	"fmt"
	"location/config"
	"log"
	"net/http"
	"time"
)

// Location represents geocoded coordinates
type Location struct {
	Latitude    float64
	Longitude   float64
	DisplayName string
	PlaceID     string
	Provider    string // Track which provider returned the result
}

// Provider interface for different geocoding services
type Provider interface {
	Geocode(ctx context.Context, address string) (*Location, error)
	GeocodeSimple(address string) (*Location, error)
	ReverseGeocode(ctx context.Context, lat, lon float64) (*Location, error)
	Name() string
}

// LocationService orchestrates multiple providers
type LocationService struct {
	providers  []Provider
	httpClient *http.Client
}

// NewLocationService creates a service with fallback providers
func NewLocationService(config *config.Config) *LocationService {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.IsDev(),
				MinVersion:         tls.VersionTLS12,
			},
			ForceAttemptHTTP2:     false, // Disable HTTP/2
			DisableKeepAlives:     false,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	// Create base providers
	photonProvider := NewPhotonProvider(httpClient)
	nominatimProvider := NewNominatimProvider(httpClient)

	return &LocationService{
		providers: []Provider{
			// Photon: allow 2 requests per second
			NewRateLimitedProvider(photonProvider, 2.0),

			// Nominatim: strict 1 request per second (required by their usage policy)
			NewRateLimitedProvider(nominatimProvider, 1.0),
		},
		httpClient: httpClient,
	}
}

// Geocode tries providers in order until one succeeds
func (s *LocationService) Geocode(ctx context.Context, address string) (*Location, error) {
	cleanAddress := SanitizeAddress(address)
	// log.Printf("Geocoding: '%s' -> cleaned: '%s'", address, cleanAddress)

	for i, provider := range s.providers {
		location, err := provider.Geocode(ctx, cleanAddress)
		if err == nil && location != nil {
			log.Printf("Provider %s succeeded on attempt %d for address: %s",
				provider.Name(), i+1, cleanAddress)
			return location, nil
		}
		log.Printf("Provider %s failed: %v", provider.Name(), err)
	}
	return nil, fmt.Errorf("all providers failed")
}

// Add convenience methods without context
func (s *LocationService) GeocodeSimple(address string) (*Location, error) {
	return s.Geocode(context.Background(), address)
}

// ReverseGeocode tries providers in order until one succeeds
func (s *LocationService) ReverseGeocode(ctx context.Context, lat, lon float64) (*Location, error) {
	var lastErr error

	for _, provider := range s.providers {
		location, err := provider.ReverseGeocode(ctx, lat, lon)
		if err == nil && location != nil {
			location.Provider = provider.Name()
			return location, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("no results found for coordinates: %f, %f", lat, lon)
}
