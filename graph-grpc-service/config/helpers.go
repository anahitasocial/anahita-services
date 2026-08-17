package config

import (
	"os"
)

// getEnv Helper function to get environment variable with a fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
