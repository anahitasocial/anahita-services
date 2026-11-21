// Package getenv reads a setting from the environment, or a fallback when it
// is unset or empty. Every service read its settings with its own copy of
// these two until 2026-10-26.
package getenv

import (
	"log"
	"os"
	"strconv"
)

// String is the variable's value, or fallback when it is unset or empty.
func String(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Int is the variable's value as a whole number, or fallback when it is
// unset, empty or not a number.
func Int(key string, fallback int) int {
	if value, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return value
	}
	return fallback
}

// Renamed is a setting that was called old until it was renamed new: new's
// value, or, when it is unset or empty, old's, with a line in the log saying
// so; "" when neither is set. Kept for one release after a rename.
func Renamed(key, old string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if value := os.Getenv(old); value != "" {
		log.Printf("%s is %s now; %s is read for this release only", old, key, old)
		return value
	}
	return ""
}

// RenamedInt is Renamed for a whole number, or fallback.
func RenamedInt(key, old string, fallback int) int {
	if value, err := strconv.Atoi(Renamed(key, old)); err == nil {
		return value
	}
	return fallback
}

// RenamedString is Renamed with a fallback for when neither is set.
func RenamedString(key, old, fallback string) string {
	if value := Renamed(key, old); value != "" {
		return value
	}
	return fallback
}
