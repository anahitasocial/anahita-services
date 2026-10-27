// Package getenv reads a setting from the environment, or a fallback when it
// is unset or empty. Every service read its settings with its own copy of
// these two until 2026-10-26.
package getenv

import (
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
