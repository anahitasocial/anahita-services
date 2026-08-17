package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const (
	photonBaseURL = "https://photon.komoot.io"
	photonName    = "Photon"
)

// PhotonProvider implements the Provider interface for Photon
type PhotonProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewPhotonProvider(httpClient *http.Client) *PhotonProvider {
	return &PhotonProvider{
		baseURL:    photonBaseURL,
		httpClient: httpClient,
	}
}

func (p *PhotonProvider) Name() string {
	return photonName
}

// Geocode performs geocoding using the Photon API
func (p *PhotonProvider) Geocode(ctx context.Context, address string) (*Location, error) {
	params := url.Values{}
	params.Add("q", address)
	params.Add("limit", "1")

	requestURL := fmt.Sprintf("%s/api?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Name    string `json:"name"`
				City    string `json:"city"`
				Country string `json:"country"`
				OSMId   int64  `json:"osm_id"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	feature := result.Features[0]
	displayName := feature.Properties.Name
	if feature.Properties.City != "" {
		displayName = fmt.Sprintf("%s, %s", displayName, feature.Properties.City)
	}
	if feature.Properties.Country != "" {
		displayName = fmt.Sprintf("%s, %s", displayName, feature.Properties.Country)
	}

	return &Location{
		Longitude:   feature.Geometry.Coordinates[0], // GeoJSON is [lon, lat]
		Latitude:    feature.Geometry.Coordinates[1],
		DisplayName: displayName,
		PlaceID:     fmt.Sprintf("osm-%d", feature.Properties.OSMId),
	}, nil
}

// GeocodeSimple is a convenience method that calls Geocode with a background context
func (p *PhotonProvider) GeocodeSimple(address string) (*Location, error) {
	return p.Geocode(context.Background(), address)
}

// ReverseGeocode performs reverse geocoding using the Photon API
func (p *PhotonProvider) ReverseGeocode(ctx context.Context, lat, lon float64) (*Location, error) {
	params := url.Values{}
	params.Add("lat", fmt.Sprintf("%f", lat))
	params.Add("lon", fmt.Sprintf("%f", lon))
	params.Add("limit", "1")

	requestURL := fmt.Sprintf("%s/reverse?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Name    string `json:"name"`
				City    string `json:"city"`
				Country string `json:"country"`
				OSMId   int64  `json:"osm_id"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	feature := result.Features[0]
	displayName := feature.Properties.Name
	if feature.Properties.City != "" {
		displayName = fmt.Sprintf("%s, %s", displayName, feature.Properties.City)
	}

	return &Location{
		Longitude:   feature.Geometry.Coordinates[0],
		Latitude:    feature.Geometry.Coordinates[1],
		DisplayName: displayName,
		PlaceID:     fmt.Sprintf("osm-%d", feature.Properties.OSMId),
	}, nil
}
