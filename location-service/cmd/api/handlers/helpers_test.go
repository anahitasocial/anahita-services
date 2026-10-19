package handlers

import (
	"errors"
	"testing"

	"anahita-libs/data/entities"
)

// A geocoder that answers one point, or fails, and remembers what it was
// asked.
type fakeGeocoder struct {
	asked []string
	fail  bool
}

func (f *fakeGeocoder) find(address string) (float64, float64, error) {
	f.asked = append(f.asked, address)
	if f.fail {
		return 0, 0, errors.New("not found")
	}
	return 49.2827, -123.1207, nil
}

func location(address, city, country string, lat, lng float64) *entities.AnahitaNode {
	node := entities.NewAnahitaNode()
	node.GeoAddress, node.GeoCity, node.GeoCountry = address, city, country
	node.GeoLatitude, node.GeoLongitude = lat, lng
	return node
}

func TestAPointGivenIsKeptAsItIs(t *testing.T) {
	geo := &fakeGeocoder{}
	node := location("1700 Beach Ave", "Vancouver", "CA", 0, 0)

	if err := setPoint(node, "49.2869", "-123.1430", true, geo.find); err != nil {
		t.Fatal(err)
	}
	if node.GeoLatitude != 49.2869 || node.GeoLongitude != -123.1430 {
		t.Errorf("point = %v, %v; want the one placed", node.GeoLatitude, node.GeoLongitude)
	}
	if len(geo.asked) != 0 {
		t.Errorf("the geocoder was asked %v; a point given needs nothing looked up", geo.asked)
	}
}

func TestALocationWithNoAddressIsMadeByItsPoint(t *testing.T) {
	geo := &fakeGeocoder{}
	node := location("", "", "", 0, 0)

	if err := setPoint(node, "49.2869", "-123.1430", true, geo.find); err != nil {
		t.Fatal(err)
	}
	if node.GeoLatitude == 0 || len(geo.asked) != 0 {
		t.Errorf("point = %v, asked %v", node.GeoLatitude, geo.asked)
	}
}

func TestAnAddressIsLookedUpWhenNoPointIsGiven(t *testing.T) {
	geo := &fakeGeocoder{}
	node := location("1700 Beach Ave", "Vancouver", "", 0, 0)

	if err := setPoint(node, "", "", true, geo.find); err != nil {
		t.Fatal(err)
	}
	if len(geo.asked) != 1 || geo.asked[0] != "1700 Beach Ave, Vancouver" {
		t.Errorf("asked %v; want the address, its empty parts left out", geo.asked)
	}
	if node.GeoLatitude != 49.2827 {
		t.Errorf("latitude = %v; want the geocoder's", node.GeoLatitude)
	}
}

func TestAnAddressThatCannotBeFoundIsRefused(t *testing.T) {
	geo := &fakeGeocoder{fail: true}
	if err := setPoint(location("Nowhere 1", "", "", 0, 0), "", "", true, geo.find); err == nil {
		t.Error("an address the geocoder cannot find, and no point, was taken")
	}
}

func TestNeitherAPointNorAnAddressIsRefused(t *testing.T) {
	geo := &fakeGeocoder{}
	if err := setPoint(location("", "", "", 0, 0), "", "", true, geo.find); !errors.Is(err, errNoPoint) {
		t.Errorf("err = %v; want errNoPoint", err)
	}
}

func TestAnEditThatClearsTheAddressKeepsThePoint(t *testing.T) {
	geo := &fakeGeocoder{}
	// The address was taken off: changed, and now empty.
	node := location("", "", "", 49.2869, -123.1430)

	if err := setPoint(node, "", "", true, geo.find); err != nil {
		t.Fatal(err)
	}
	if node.GeoLatitude != 49.2869 || len(geo.asked) != 0 {
		t.Errorf("point = %v, asked %v; want the point kept", node.GeoLatitude, geo.asked)
	}
}

func TestAnEditThatLeavesTheAddressLooksNothingUp(t *testing.T) {
	geo := &fakeGeocoder{}
	node := location("1700 Beach Ave", "Vancouver", "CA", 49.2869, -123.1430)

	if err := setPoint(node, "", "", false, geo.find); err != nil {
		t.Fatal(err)
	}
	if len(geo.asked) != 0 || node.GeoLatitude != 49.2869 {
		t.Errorf("asked %v; an unchanged address is not looked up again", geo.asked)
	}
}

func TestHalfAPointIsRefused(t *testing.T) {
	geo := &fakeGeocoder{}
	if err := setPoint(location("", "", "", 0, 0), "49.2869", "", true, geo.find); err == nil {
		t.Error("a latitude without a longitude was taken")
	}
}

func TestThePostalCodeIsPartOfTheAddressLookedUp(t *testing.T) {
	geo := &fakeGeocoder{}
	node := location("1700 Beach Ave", "Vancouver", "CA", 0, 0)
	node.GeoPostalcode = "V6G 1Z4"

	if err := setPoint(node, "", "", true, geo.find); err != nil {
		t.Fatal(err)
	}
	if len(geo.asked) != 1 || geo.asked[0] != "1700 Beach Ave, Vancouver, V6G 1Z4, CA" {
		t.Errorf("asked %v; want the postal code before the country", geo.asked)
	}
}
