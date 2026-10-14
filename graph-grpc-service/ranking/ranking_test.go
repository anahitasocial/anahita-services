package ranking

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTheFileShippedIsTheDefault(t *testing.T) {
	raw, err := os.ReadFile("../../graph-grpc-defaults/search_ranking.json")
	if err != nil {
		t.Fatal(err)
	}
	weights, ok := Parse(raw)
	if !ok || weights != Default() {
		t.Errorf("the file says %+v", weights)
	}
}

func TestWhatIsNotUnderstoodIsAsItComes(t *testing.T) {
	for _, raw := range []string{`not json`, `{"following": -1}`, `{"half_life_days": 5000}`, `{"useful": {"scale": "big"}}`} {
		if weights, ok := Parse([]byte(raw)); ok || weights != Default() {
			t.Errorf("%s: %+v", raw, weights)
		}
	}
	// What a file leaves out is as it comes; what it says is taken.
	weights, ok := Parse([]byte(`{"half_life_days": 30}`))
	if !ok || weights.HalfLifeDays != 30 || weights.Match != Default().Match {
		t.Errorf("%+v", weights)
	}
}

func TestTheFileIsReadAgainWhenItChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search_ranking.json")
	source := NewSource(path)
	if source.Weights() != Default() {
		t.Fatal("without a file the weights are not as they come")
	}

	if err := os.WriteFile(path, []byte(`{"half_life_days": 30}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := source.Weights().HalfLifeDays; got != 30 {
		t.Fatalf("read %v", got)
	}

	later := time.Now().Add(time.Minute)
	if err := os.WriteFile(path, []byte(`{"half_life_days": 0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(path, later, later)
	if got := source.Weights().HalfLifeDays; got != 0 {
		t.Errorf("read again %v", got)
	}
}
