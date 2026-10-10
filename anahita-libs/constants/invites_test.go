package constants

import "testing"

func TestInviteMaxPendingFor(t *testing.T) {
	cases := []struct {
		capacity uint32
		want     uint64
	}{
		{0, 500},   // no limit on places: the general one
		{10, 50},   // never fewer than fifty
		{25, 50},   // twice 25
		{100, 200}, // twice the places
		{400, 500}, // never more than the general one
	}

	for _, c := range cases {
		if got := InviteMaxPendingFor(c.capacity); got != c.want {
			t.Errorf("%d places: %d, want %d", c.capacity, got, c.want)
		}
	}
}
