package handlers

import (
	"fmt"
	"testing"

	"anahita-libs/policy"
)

// NodeInfo claims a protocol only when the operator says its service is
// running, and never on a site that keeps itself to its members.
func TestNodeInfoClaimsOnlyWhatIsSaidToRun(t *testing.T) {
	for said, want := range map[string]string{
		"":                           "[]",
		"activitypub":                "[activitypub]",
		" ActivityPub , activitypub": "[activitypub]",
		"activitypub,carrier-pigeon": "[activitypub]",
		"atproto":                    "[]",
	} {
		t.Setenv("FEDERATION_PROTOCOLS", said)
		if got := fmt.Sprint(federationProtocols()); got != want {
			t.Errorf("%q: %s, want %s", said, got, want)
		}
	}

	t.Setenv("FEDERATION_PROTOCOLS", "activitypub")
	for _, access := range []string{policy.ReadPreview, policy.ReadRegistered} {
		restore := policy.SetReadAccessForTest(access)
		if got := federationProtocols(); len(got) != 0 {
			t.Errorf("a site that is %s claims %v", access, got)
		}
		restore()
	}
}
