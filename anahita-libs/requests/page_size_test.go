package requests

import (
	"testing"

	"anahita-libs/constants"
)

// A page is what was asked, up to the most a page holds, and the usual
// page when nothing was.
func TestAPageIsWhatWasAskedUpToItsMost(t *testing.T) {
	for asked, want := range map[uint64]uint64{
		0:                         constants.DefaultPageSize,
		7:                         7,
		constants.MaxPageSize:     constants.MaxPageSize,
		constants.MaxPageSize + 1: constants.MaxPageSize,
		100000:                    constants.MaxPageSize,
	} {
		if got := PageSize(asked); got != want {
			t.Errorf("PageSize(%d) = %d, want %d", asked, got, want)
		}
	}
}
