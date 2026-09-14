package entities

import "testing"

// AcceptedAt is a pointer so that "never" is expressible. A zero time.Time
// would be both a fabricated date and, to MySQL under a strict sql_mode,
// '0000-00-00' — which is rejected outright.
func TestConsentTimestampIsAbsentUntilAccepted(t *testing.T) {
	c := Consent{}

	if c.AcceptedAt != nil {
		t.Fatal("an unaccepted consent carries a timestamp")
	}

	c.Accept("1.0", "203.0.113.9", "curl/8", "Linux")

	if c.AcceptedAt == nil {
		t.Fatal("accepting recorded no timestamp")
	}
	if !c.Accepted || c.Version != "1.0" {
		t.Errorf("Accept did not record the acceptance: %+v", c)
	}
	if c.IP != "203.0.113.9" || c.UserAgent != "curl/8" || c.OS != "Linux" {
		t.Errorf("the audit trail was not recorded: %+v", c)
	}
}
