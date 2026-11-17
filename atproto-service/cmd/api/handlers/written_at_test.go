package handlers

import (
	"testing"
	"time"

	"anahita-libs/data/entities"
)

// The parts of a thread are written in one second, and Bluesky lists only
// one of the posts of the same moment: each is told apart by its id, as
// milliseconds, in the order they were written.
func TestWrittenAtTellsAThreadsPartsApart(t *testing.T) {
	second := time.Date(2026, 11, 17, 9, 48, 31, 0, time.UTC)
	var stamps []string
	for id := int64(188180); id <= 188183; id++ {
		stamps = append(stamps, writtenAt(&entities.AnahitaNode{ID: id, CreatedAt: second}).Format("15:04:05.000"))
	}
	want := []string{"09:48:31.180", "09:48:31.181", "09:48:31.182", "09:48:31.183"}
	for i := range want {
		if stamps[i] != want[i] {
			t.Fatalf("stamps = %v, want %v", stamps, want)
		}
	}
}

// A time that has its own milliseconds keeps them, and no time stays none.
func TestWrittenAtKeepsATimeThatIsExact(t *testing.T) {
	exact := time.Date(2026, 11, 17, 9, 48, 31, 250_000_000, time.UTC)
	if got := writtenAt(&entities.AnahitaNode{ID: 7, CreatedAt: exact}); !got.Equal(exact) {
		t.Errorf("writtenAt = %v, want %v", got, exact)
	}
	if got := writtenAt(&entities.AnahitaNode{ID: 7}); !got.IsZero() {
		t.Errorf("writtenAt of no time = %v", got)
	}
}
