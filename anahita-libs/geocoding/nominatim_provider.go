package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const (
	// NominatimProvider is a geocoding provider that uses the Nominatim API
	nominatimBaseURL = "https://nominatim.openstreetmap.org"
	nominatimName    = "Nominatim"
)

// NominatimProvider implements the Provider interface for Nominatim
type NominatimProvider struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// NewNominatimProvider reaches Nominatim at baseURL, or at the public
// instance when baseURL is empty.
func NewNominatimProvider(httpClient *http.Client, baseURL string) *NominatimProvider {
	if baseURL == "" {
		baseURL = nominatimBaseURL
	}
	return &NominatimProvider{
		baseURL:    baseURL,
		httpClient: httpClient,
		userAgent:  geocoderUserAgent(),
	}
}

func (n *NominatimProvider) Name() string {
	return nominatimName
}

func (n *NominatimProvider) Geocode(ctx context.Context, address string) (*Location, error) {
	params := url.Values{}
	params.Add("q", address)
	params.Add("format", "json")
	params.Add("limit", "1")

	requestURL := fmt.Sprintf("%s/search?%s", n.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", n.userAgent)

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var results []struct {
		PlaceID     int    `json:"place_id"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	result := results[0]

	var lat, lon float64
	fmt.Sscanf(result.Lat, "%f", &lat)
	fmt.Sscanf(result.Lon, "%f", &lon)

	return &Location{
		Latitude:    lat,
		Longitude:   lon,
		DisplayName: result.DisplayName,
		PlaceID:     fmt.Sprintf("nominatim-%d", result.PlaceID),
	}, nil
}

// GeocodeSimple is a convenience method that calls Geocode with a background context
func (n *NominatimProvider) GeocodeSimple(address string) (*Location, error) {
	return n.Geocode(context.Background(), address)
}

func (n *NominatimProvider) ReverseGeocode(ctx context.Context, lat, lon float64) (*Location, error) {
	params := url.Values{}
	params.Add("lat", fmt.Sprintf("%f", lat))
	params.Add("lon", fmt.Sprintf("%f", lon))
	params.Add("format", "json")

	requestURL := fmt.Sprintf("%s/reverse?%s", n.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", n.userAgent)

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		PlaceID     int    `json:"place_id"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var latitude, longitude float64
	fmt.Sscanf(result.Lat, "%f", &latitude)
	fmt.Sscanf(result.Lon, "%f", &longitude)

	return &Location{
		Latitude:    latitude,
		Longitude:   longitude,
		DisplayName: result.DisplayName,
		PlaceID:     fmt.Sprintf("nominatim-%d", result.PlaceID),
	}, nil
}
