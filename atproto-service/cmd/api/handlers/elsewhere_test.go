package handlers

import (
	"context"
	"testing"
)

// An author's server is asked for once in a while, not for every post.
func TestAnAuthorsServerIsRemembered(t *testing.T) {
	asked := 0
	find := CachedPDS(func(context.Context, string) (string, error) {
		asked++
		return "https://pds.example", nil
	})
	for i := 0; i < 3; i++ {
		if pds, err := find(context.Background(), "did:plc:aaaa"); err != nil || pds != "https://pds.example" {
			t.Fatalf("%s %v", pds, err)
		}
	}
	if asked != 1 {
		t.Errorf("asked %d times", asked)
	}
}
