package handlers

import (
	"errors"
	"strconv"
	"strings"

	"anahita-libs/data/entities"
	an_mw "anahita-libs/middleware"
	"location/requests"

	"github.com/labstack/echo/v4"
)

// getViewer returns the Viewer the OAuth middleware built from the
// access token claims. Returns a non-nil unauthenticated Viewer when no
// token was present, so callers never need a nil check.
func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}

func addressChanged(req *requests.LocationEditRequest, node *entities.AnahitaNode) bool {
	return req.Address != node.GeoAddress ||
		req.City != node.GeoCity ||
		req.StateProvince != node.GeoStateProvince ||
		req.PostalCode != node.GeoPostalcode ||
		req.Country != node.GeoCountry
}

// errNoPoint is a location that would have no point: none given, and no
// address to find one by.
var errNoPoint = errors.New("a location needs a point: one placed on a map, or an address to find it by")

// givenPoint reads a point given with a request: both coordinates, or
// neither. One without the other is refused.
func givenPoint(latitude, longitude string) (lat, lng float64, given bool, err error) {
	latitude, longitude = strings.TrimSpace(latitude), strings.TrimSpace(longitude)
	if latitude == "" && longitude == "" {
		return 0, 0, false, nil
	}
	if latitude == "" || longitude == "" {
		return 0, 0, false, errors.New("a point needs both a latitude and a longitude")
	}
	if lat, err = strconv.ParseFloat(latitude, 64); err != nil {
		return 0, 0, false, err
	}
	if lng, err = strconv.ParseFloat(longitude, 64); err != nil {
		return 0, 0, false, err
	}
	return lat, lng, true, nil
}

// addressOf is a location's address as one line for the geocoder, its empty
// parts left out: "" when it has none.
func addressOf(node *entities.AnahitaNode) string {
	parts := []string{}
	for _, part := range []string{node.GeoAddress, node.GeoCity, node.GeoStateProvince, node.GeoPostalcode, node.GeoCountry} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ", ")
}

// geocoder finds the point of an address.
type geocoder func(address string) (lat, lng float64, err error)

// setPoint decides a location's point, onto node (roadmap row 82):
//
//  1. a point given, placed on a map, is kept as it is;
//  2. without one, an address to look up (on an add, any address; on an
//     edit, one that changed) is found by the geocoder;
//  3. without either, an edit keeps the point it has, and an add is refused.
//
// So a location without an address is made by placing it, and clearing an
// address never loses a point.
func setPoint(node *entities.AnahitaNode, latitude, longitude string, lookUp bool, find geocoder) error {
	lat, lng, given, err := givenPoint(latitude, longitude)
	if err != nil {
		return err
	}
	if given {
		node.GeoLatitude, node.GeoLongitude = lat, lng
		return nil
	}

	if address := addressOf(node); lookUp && address != "" {
		lat, lng, err := find(address)
		if err != nil {
			return err
		}
		node.GeoLatitude, node.GeoLongitude = lat, lng
		return nil
	}

	if node.GeoLatitude == 0 && node.GeoLongitude == 0 {
		return errNoPoint
	}
	return nil
}
