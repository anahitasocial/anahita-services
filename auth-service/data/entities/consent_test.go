package entities

import "testing"

// The version is the whole reason consent is more than a boolean, so these pin
// what it is for: finding who has not agreed to what is currently in force.

func TestConsentNeedsAcceptance(t *testing.T) {
	accepted := func(version string) Consent {
		c := Consent{}
		c.Accept(version, "203.0.113.9", "curl/8", "Linux")
		return c
	}

	for _, tc := range []struct {
		name    string
		consent Consent
		current string
		want    bool
		because string
	}{
		{
			name: "nothing on record", consent: Consent{}, current: "1.0", want: true,
			because: "an account that accepted nothing has not agreed to anything",
		},
		{
			name: "current version", consent: accepted("1.0"), current: "1.0", want: false,
			because: "asking again would train people to tick past the boxes",
		},
		{
			name: "superseded version", consent: accepted("1.0"), current: "2.0", want: true,
			because: "they are being held to text they were never shown",
		},
		{
			// A deployment rolled back, or an operator fixing a typo in the
			// version. Re-prompting is the safe direction to be wrong in:
			// the cost is one extra checkbox, where the other way round is a
			// consent record that claims more than it can support.
			name: "version went backwards", consent: accepted("2.0"), current: "1.0", want: true,
			because: "an unrecognised version is not proof of agreement",
		},
		{
			// Guards the shape of the check itself. Accepted without a
			// version is what a bare boolean column would have produced, and
			// it must not read as up to date.
			name:    "accepted but no version recorded",
			consent: Consent{Accepted: true}, current: "1.0", want: true,
			because: "an acceptance that cannot say what it accepted proves nothing",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.consent.NeedsAcceptance(tc.current); got != tc.want {
				t.Errorf("NeedsAcceptance(%q) = %v, want %v — %s",
					tc.current, got, tc.want, tc.because)
			}
		})
	}
}

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
