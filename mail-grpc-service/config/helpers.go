package config

import (
	"os"
	"strconv"
)

// getEnv Helper function to get environment variable with a fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getIntEnv Helper function to get environment variable with a fallback
func getIntEnv(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	value, _ := parseBoolEnv(key, fallback)
	return value
}

// parseBoolEnv reports the value AND whether it could be understood.
//
// The second return is why this exists. strconv.ParseBool rejects anything but
// 1/t/T/TRUE/true/True and their false counterparts, and the plain getBoolEnv
// above swallows the failure and hands back the fallback — so SMTP_SECURE="ssl"
// silently became false.
//
// For a transport-security setting that is the wrong way to be wrong. "ssl" is
// a completely reasonable thing to write, it means the opposite of what it
// quietly did, and nothing said so. Callers that care use this form and refuse
// to start; see Config.Validate.
func parseBoolEnv(key string, fallback bool) (value bool, ok bool) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, true
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback, false
	}

	return parsed, true
}
