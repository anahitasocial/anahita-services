package linkpreviews

import (
	"errors"
	"testing"
	"time"
)

type mapCache map[string]string

func (m mapCache) Get(key string) (string, bool, error) {
	value, found := m[key]
	return value, found, nil
}

func (m mapCache) Set(key, value string, _ time.Duration) error {
	m[key] = value
	return nil
}

// THE RULE THIS GUARDS: a cache that is not there when a service starts is
// found once it is. Looked for once and given up on, every note was saved
// without its card until the service was started again.
func TestACacheThatIsLateIsFound(t *testing.T) {
	kept := mapCache{}
	there := false
	tries := 0
	late := &lateCache{connect: func() (Cache, error) {
		tries++
		if !there {
			return nil, errors.New("still starting")
		}
		return kept, nil
	}}
	store := &Store{cache: late}
	preview := &Preview{URL: "https://example.com/a", Title: "A", Status: StatusOK}

	if _, found := store.Get(preview.URL); found || tries != 1 {
		t.Fatalf("found %v after %d tries", found, tries)
	}
	// Not asked for again at once: every note would wait on it.
	if _, found := store.Get(preview.URL); found || tries != 1 {
		t.Fatalf("asked for again at once: %d tries", tries)
	}

	there = true
	late.triedAt = time.Now().Add(-2 * reconnectAfter)
	if err := store.Put(preview); err != nil {
		t.Fatal(err)
	}
	if got, found := store.Get(preview.URL); !found || got.Title != "A" || tries != 2 {
		t.Fatalf("after it came: %+v, %v, %d tries", got, found, tries)
	}
}
