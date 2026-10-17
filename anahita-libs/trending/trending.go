// Package trending says over what span of time hashtags and places are
// trending: the operator's choice, or what suits the installation, widened
// when it holds too few (the user, 2026-10-17).
package trending

import (
	"os"
	"strings"

	"anahita-libs/constants"
)

// The windows, narrowest first: what a window is widened to when it holds
// too few.
var Windows = []string{
	constants.TimeWindowDaily,
	constants.TimeWindowWeekly,
	constants.TimeWindowMonthly,
	constants.TimeWindowYearly,
}

// Enough is how many a window must hold not to be widened, unless fewer
// are asked for.
const Enough = 10

// DefaultWindow is the window trending starts from when a client does not
// name one: TRENDING_WINDOW, one of daily, weekly, monthly or yearly; or,
// left unset, daily on an installation that federates (FEDERATION_PROTOCOLS
// names a protocol), where the fediverse brings enough every day, and
// monthly on one only its own members post to.
func DefaultWindow() string {
	if window := Normal(os.Getenv("TRENDING_WINDOW")); window != "" {
		return window
	}
	if strings.TrimSpace(os.Getenv("FEDERATION_PROTOCOLS")) != "" {
		return constants.TimeWindowDaily
	}
	return constants.TimeWindowMonthly
}

// Normal is a window as the graph takes it, or "" for one that is not one
// of Windows.
func Normal(window string) string {
	window = strings.ToUpper(strings.TrimSpace(window))
	for _, each := range Windows {
		if window == each {
			return each
		}
	}
	return ""
}

// Widen asks find for each window from start on, wider and wider, until one
// holds enough (Enough, or limit when fewer are asked for) or there is none
// wider; and answers with what the last held and which window it was.
func Widen[T any](start string, limit uint64, find func(window string) ([]T, error)) ([]T, string, error) {
	want := uint64(Enough)
	if limit > 0 && limit < want {
		want = limit
	}
	from := 0
	for i, each := range Windows {
		if each == Normal(start) {
			from = i
		}
	}

	var (
		found  []T
		window string
	)
	for _, each := range Windows[from:] {
		var err error
		found, err = find(each)
		if err != nil {
			return nil, "", err
		}
		window = each
		if uint64(len(found)) >= want {
			break
		}
	}
	return found, window, nil
}
