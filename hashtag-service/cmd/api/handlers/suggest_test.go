package handlers

import (
	"reflect"
	"testing"
)

func TestWordsOfAText(t *testing.T) {
	got := wordsOf("Open science is open. See [the paper](https://example.org/open-paper) and https://x.org/y, `code here`, #Garden @ana. L'école!", 20)
	if got[0] != "open" {
		t.Errorf("most used first: %v", got)
	}
	for _, never := range []string{"https", "example", "org", "code", "garden", "ana", "is", "openpaper"} {
		for _, word := range got {
			if word == never {
				t.Errorf("%q is not a word of the text: %v", never, got)
			}
		}
	}
	for _, must := range []string{"science", "openscience", "paper", "lécole"} {
		found := false
		for _, word := range got {
			found = found || word == must
		}
		if !found {
			t.Errorf("%q missing: %v", must, got)
		}
	}

	if got := wordsOf("one two three four", 2); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Errorf("most: %v", got)
	}
}
