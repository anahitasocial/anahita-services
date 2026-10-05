package language

import "testing"

func TestNormalize(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"en", "en", true},
		{"FR", "fr", true},
		{"fr-ca", "fr-CA", true},
		{"FR_ca", "fr-CA", true}, // some systems write an underscore
		{"pt-BR", "pt-BR", true},
		{"es-419", "es-419", true}, // Latin American Spanish, a numbered region
		{" fa ", "fa", true},
		{"und", "und", true},
		{"yue", "yue", true}, // Cantonese has no two-letter code

		{"", "", false},
		{"xx", "", false},      // two letters, but not a language
		{"english", "", false}, // a name, not a code
		{"en-", "", false},
		{"en-GBR", "", false},
		{"e", "", false},
		{"en fr", "", false},
		{"en,fr", "", false},
		{"en-US-posix", "", false}, // more than a language and a region
		{"<script>", "", false},
	} {
		got, ok := Normalize(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Normalize(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestEveryTwoLetterCodeIsKnown(t *testing.T) {
	// ISO 639-1 has a little over 180 codes. A list that lost a line, or
	// gained a stray word, shows as a count well away from that.
	if len(iso6391) < 180 || len(iso6391) > 190 {
		t.Errorf("%d two-letter codes; the list has been damaged", len(iso6391))
	}
	for code := range iso6391 {
		if len(code) != 2 {
			t.Errorf("%q is in the list of two-letter codes", code)
		}
	}
	for _, code := range []string{"en", "fr", "fa", "ar", "zh", "es", "de", "ja", "hi", "pt"} {
		if !iso6391[code] {
			t.Errorf("%s is missing", code)
		}
	}
}

func TestPrimary(t *testing.T) {
	for in, want := range map[string]string{"fr-CA": "fr", "en": "en", "PT_br": "pt", "": ""} {
		if got := Primary(in); got != want {
			t.Errorf("Primary(%q) = %q, want %q", in, got, want)
		}
	}
}
