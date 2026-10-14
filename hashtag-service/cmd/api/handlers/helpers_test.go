package handlers

import (
	"fmt"
	"testing"
)

// THE RULE THIS GUARDS: a hashtag is one whatever its capitals, so a post
// writing #Anahita finds #anahita and makes no second one.
func TestAHashtagIsOneWhateverItsCapitals(t *testing.T) {
	aliases, names := hashtagAliases([]string{"Anahita", " anahita ", "BeaconBits", "", "Vancouver"})
	if got := fmt.Sprint(aliases); got != "[anahita beaconbits vancouver]" {
		t.Errorf("aliases %s", got)
	}
	if names["anahita"] != "Anahita" || names["beaconbits"] != "BeaconBits" {
		t.Errorf("names %v", names)
	}
}
