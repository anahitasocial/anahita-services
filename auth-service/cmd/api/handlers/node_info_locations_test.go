package handlers

import "testing"

// Who may add a location (LOCATIONS_FROM) is in NodeInfo, so the web app
// offers Add only to those location-service lets add one; left out, it is
// administrators, as location-service has it.
func TestNodeInfoSaysWhoMayAddALocation(t *testing.T) {
	if got := nodeInfoMetadata(t)["locationsFrom"]; got != "administrators" {
		t.Errorf("locationsFrom = %v; want administrators when it is left out", got)
	}
}
