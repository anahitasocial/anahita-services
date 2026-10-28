package handlers

import "testing"

// A code in a subject never reaches the log; the rest of the subject does.
func TestTheLogLeavesCodesOutOfSubjects(t *testing.T) {
	for subject, want := range map[string]string{
		"665832 is your banu.io email change code": "… is your banu.io email change code",
		"Your sign-in code: 12345678":              "Your sign-in code: …",
		"A passkey was added to your account":      "A passkey was added to your account",
		"Invited by 42 people":                     "Invited by 42 people",
	} {
		if got := withoutCodes(subject); got != want {
			t.Errorf("withoutCodes(%q) = %q, want %q", subject, got, want)
		}
	}
}
