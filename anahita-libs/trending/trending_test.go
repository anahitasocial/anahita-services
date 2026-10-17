package trending

import (
	"testing"

	"anahita-libs/constants"
)

func TestTheDefaultWindow(t *testing.T) {
	t.Setenv("TRENDING_WINDOW", "")
	t.Setenv("FEDERATION_PROTOCOLS", "")
	if got := DefaultWindow(); got != constants.TimeWindowMonthly {
		t.Errorf("only its own members: %q", got)
	}
	t.Setenv("FEDERATION_PROTOCOLS", "activitypub")
	if got := DefaultWindow(); got != constants.TimeWindowDaily {
		t.Errorf("federating: %q", got)
	}
	t.Setenv("TRENDING_WINDOW", "weekly")
	if got := DefaultWindow(); got != constants.TimeWindowWeekly {
		t.Errorf("chosen: %q", got)
	}
	t.Setenv("TRENDING_WINDOW", "hourly")
	if got := DefaultWindow(); got != constants.TimeWindowDaily {
		t.Errorf("not a window: %q", got)
	}
}

func TestAWindowIsWidenedUntilItHoldsEnough(t *testing.T) {
	held := map[string]int{
		constants.TimeWindowDaily:   3,
		constants.TimeWindowWeekly:  12,
		constants.TimeWindowMonthly: 40,
	}
	find := func(window string) ([]int, error) {
		return make([]int, held[window]), nil
	}

	found, window, _ := Widen("daily", 20, find)
	if window != constants.TimeWindowWeekly || len(found) != 12 {
		t.Errorf("got %d from %s", len(found), window)
	}
	// Asked for fewer than Enough, fewer are enough.
	found, window, _ = Widen("daily", 3, find)
	if window != constants.TimeWindowDaily || len(found) != 3 {
		t.Errorf("got %d from %s", len(found), window)
	}
	// Nothing wider than a year: what it holds, however few.
	found, window, _ = Widen("yearly", 20, func(string) ([]int, error) { return nil, nil })
	if window != constants.TimeWindowYearly || len(found) != 0 {
		t.Errorf("got %d from %s", len(found), window)
	}
}
