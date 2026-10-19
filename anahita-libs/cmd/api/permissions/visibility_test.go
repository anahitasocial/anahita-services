package permissions

import (
	"errors"
	"testing"

	"anahita-libs/data/entities"
)

type fakeVisibility struct {
	visible map[int64]bool
	err     error
	asked   [][]int64
}

func (f *fakeVisibility) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	f.asked = append(f.asked, ids)
	if f.err != nil {
		return nil, f.err
	}
	out := []int64{}
	for _, id := range ids {
		if f.visible[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

func TestCanReadByID(t *testing.T) {
	gate := &fakeVisibility{visible: map[int64]bool{10: true}}
	if !CanReadByID(gate, 10, entities.NewViewer()) || CanReadByID(gate, 11, entities.NewViewer()) || CanReadByID(gate, 0, entities.NewViewer()) {
		t.Error("CanReadByID disagrees with the gate")
	}
	if CanReadByID(&fakeVisibility{err: errors.New("unreachable")}, 10, entities.NewViewer()) {
		t.Error("said yes without an answer")
	}
}

// Many at once, in one question, each id once.
func TestCanReadEach(t *testing.T) {
	gate := &fakeVisibility{visible: map[int64]bool{10: true, 12: true}}
	readable, err := CanReadEach(gate, []int64{10, 11, 12, 10, 0}, entities.NewViewer())
	if err != nil {
		t.Fatal(err)
	}
	if !readable[10] || readable[11] || !readable[12] {
		t.Errorf("readable %v", readable)
	}
	if len(gate.asked) != 1 || len(gate.asked[0]) != 3 {
		t.Errorf("asked %v, want once, for 10, 11 and 12", gate.asked)
	}

	if _, err := CanReadEach(&fakeVisibility{err: errors.New("unreachable")}, []int64{10}, entities.NewViewer()); err == nil {
		t.Error("no error without an answer")
	}

	none := &fakeVisibility{}
	if readable, err := CanReadEach(none, nil, entities.NewViewer()); err != nil || len(readable) != 0 || len(none.asked) != 0 {
		t.Error("asked about nothing")
	}
}
