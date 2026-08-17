package geocoding

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimitedProvider wraps a provider with rate limiting
type RateLimitedProvider struct {
	provider Provider
	limiter  *rate.Limiter
}

// NewRateLimitedProvider creates a rate-limited wrapper around a provider
func NewRateLimitedProvider(provider Provider, requestsPerSecond float64) *RateLimitedProvider {
	return &RateLimitedProvider{
		provider: provider,
		limiter:  rate.NewLimiter(rate.Limit(requestsPerSecond), 1),
	}
}

// Name implements the Provider interface
func (r *RateLimitedProvider) Name() string {
	return r.provider.Name()
}

// Geocode implements the Provider interface with rate limiting
func (r *RateLimitedProvider) Geocode(ctx context.Context, address string) (*Location, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return r.provider.Geocode(ctx, address)
}

// GeocodeSimple implements the Provider interface with rate limiting
func (r *RateLimitedProvider) GeocodeSimple(address string) (*Location, error) {
	return r.Geocode(context.Background(), address)
}

// ReverseGeocode implements the Provider interface with rate limiting
func (r *RateLimitedProvider) ReverseGeocode(ctx context.Context, lat, lon float64) (*Location, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return r.provider.ReverseGeocode(ctx, lat, lon)
}
