package repositories

import (
	"testing"

	"anahita-libs/constants"
)

// THE RULE THIS GUARDS (roadmap row 170): a list reads no more than a page
// and one more, however many are asked for, the one more being how a
// caller knows there is a page after it.
func TestAListReadsAPageAndOneMoreAtMost(t *testing.T) {
	for asked, want := range map[uint64]uint64{
		0:                         constants.DefaultPageSize,
		20:                        20,
		constants.MaxPageSize + 1: constants.MaxPageSize + 1,
		constants.MaxPageSize + 2: constants.MaxPageSize + 1,
		1000000:                   constants.MaxPageSize + 1,
	} {
		if got := pageLimit(asked); got != want {
			t.Errorf("pageLimit(%d) = %d, want %d", asked, got, want)
		}
	}
}
