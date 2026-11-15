package handlers

import (
	"slices"
	"testing"

	"anahita-libs/photos"
)

// A thread is checked for each kind of post its parts are, once each, so
// that a site of photos alone refuses one of words with "off".
func TestAThreadIsEachKindItsPartsAre(t *testing.T) {
	words := ThreadPart{Content: "words"}
	pictures := ThreadPart{Uploads: []photos.FileRef{{UploadID: 1}}}
	for _, each := range []struct {
		parts []ThreadPart
		want  []string
	}{
		{[]ThreadPart{words, words}, []string{"note"}},
		{[]ThreadPart{pictures, pictures}, []string{"photo"}},
		{[]ThreadPart{pictures, words, pictures}, []string{"photo", "note"}},
	} {
		if got := threadKinds(each.parts); !slices.Equal(got, each.want) {
			t.Errorf("threadKinds = %v, want %v", got, each.want)
		}
	}
}
