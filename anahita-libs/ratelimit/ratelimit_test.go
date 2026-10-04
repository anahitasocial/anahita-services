package ratelimit

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type memory struct {
	counts map[string]int64
	ttls   map[string]time.Duration
	err    error
}

func newMemory() *memory {
	return &memory{counts: map[string]int64{}, ttls: map[string]time.Duration{}}
}

func (m *memory) Incr(key string, ttl time.Duration) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.counts[key]++
	m.ttls[key] = ttl
	return m.counts[key], nil
}

func (m *memory) Del(keys ...string) error {
	for _, key := range keys {
		delete(m.counts, key)
	}
	return m.err
}

var three = Limit{Max: 3, Window: time.Hour}

func TestAllowsUpToTheLimitThenRefuses(t *testing.T) {
	store := newMemory()

	for try := 1; try <= 3; try++ {
		if exceeded, _ := Check(store, "reports", "ada", three); exceeded {
			t.Fatalf("try %d was refused", try)
		}
	}

	exceeded, first := Check(store, "reports", "ada", three)
	if !exceeded || !first {
		t.Errorf("the fourth try: exceeded=%v first=%v, want both", exceeded, first)
	}

	exceeded, first = Check(store, "reports", "ada", three)
	if !exceeded || first {
		t.Errorf("the fifth try: exceeded=%v first=%v, want exceeded and not first", exceeded, first)
	}
}

func TestSubjectsAndScopesAreCountedApart(t *testing.T) {
	store := newMemory()

	for range 4 {
		Check(store, "reports", "ada", three)
	}

	if Exceeded(store, "reports", "grace", three) {
		t.Error("one person's count limited another")
	}
	if Exceeded(store, "invites", "ada", three) {
		t.Error("one scope's count limited another")
	}
}

func TestTheWindowIsWhatTheCounterIsGiven(t *testing.T) {
	store := newMemory()
	Check(store, "reports", "ada", three)

	if got := store.ttls[Key("reports", "ada")]; got != time.Hour {
		t.Errorf("window = %v, want an hour", got)
	}
}

func TestTheSubjectIsNotInTheKey(t *testing.T) {
	key := Key("login", "ada@example.com")

	if strings.Contains(key, "ada") || strings.Contains(key, "example") {
		t.Errorf("key %q carries the address", key)
	}
	if !strings.HasPrefix(key, "ratelimit:login:") {
		t.Errorf("key %q", key)
	}
}

func TestAnEmptySubjectIsNeitherLimitedNorCounted(t *testing.T) {
	store := newMemory()

	for range 10 {
		if Exceeded(store, "reports", "", three) {
			t.Fatal("an empty subject was limited")
		}
	}
	if len(store.counts) != 0 {
		t.Error("an empty subject was counted")
	}
}

func TestFailsOpenWhenTheCounterIsDown(t *testing.T) {
	store := newMemory()
	store.err = errors.New("redis is away")

	for range 10 {
		if Exceeded(store, "reports", "ada", three) {
			t.Fatal("refused because the counter failed")
		}
	}
}

func TestResetClearsTheCount(t *testing.T) {
	store := newMemory()

	for range 4 {
		Check(store, "reports", "ada", three)
	}
	Reset(store, "reports", "ada")

	if Exceeded(store, "reports", "ada", three) {
		t.Error("still limited after a reset")
	}
}
