package federation

import (
	"testing"
	"time"

	"anahita-libs/data/entities"
)

func TestWhatWasWrittenTogether(t *testing.T) {
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	parent := &entities.AnahitaNode{ID: 1, CreatedBy: 7, CreatedAt: at}
	reply := func(by int64, after time.Duration) *entities.AnahitaNode {
		return &entities.AnahitaNode{ID: 2, ParentID: 1, CreatedBy: by, CreatedAt: at.Add(after)}
	}
	for _, tc := range []struct {
		name  string
		reply *entities.AnahitaNode
		want  bool
	}{
		{"the next part of a thread", reply(7, 0), true},
		{"a moment later", reply(7, 30*time.Second), true},
		{"the writer answering an old post", reply(7, 2*time.Hour), false},
		{"somebody else", reply(8, 0), false},
	} {
		if got := WrittenTogether(parent, tc.reply); got != tc.want {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
