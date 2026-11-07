package services

import (
	"errors"
	"testing"
)

// Browse lists only what the services store: under assets/v1/, and nowhere
// it could climb out of. Refused before the store is asked.
func TestBrowseListsOnlyUnderAssets(t *testing.T) {
	storage := &Storage{}
	for _, each := range []struct{ prefix, after string }{
		{"", ""},
		{"assets/", ""},
		{"assets/v0/", ""},
		{"backups/", ""},
		{"assets/v1/../secrets/", ""},
		{"assets/v1/photo-service/", "elsewhere/key"},
	} {
		if _, _, err := storage.Browse(each.prefix, each.after, 10); !errors.Is(err, ErrOutsideAssets) {
			t.Errorf("Browse(%q, %q): %v, want ErrOutsideAssets", each.prefix, each.after, err)
		}
	}
}
