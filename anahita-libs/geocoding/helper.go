package geocoding

import (
	"regexp"
	"strings"
)

func SanitizeAddress(address string) string {
	// Remove apartment/unit numbers (e.g., "#210 - ", "Apt 5, ", "Unit 3B - ")
	address = regexp.MustCompile(`(?i)^(#\d+\s*-?\s*|apt\.?\s*\d+\w?\s*[,\-]\s*|unit\s*\d+\w?\s*[,\-]\s*)`).ReplaceAllString(address, "")

	// Replace multiple commas with single comma
	address = regexp.MustCompile(`,\s*,+`).ReplaceAllString(address, ",")

	// Remove leading/trailing commas
	address = strings.Trim(address, ", ")

	// Remove extra spaces
	address = strings.Join(strings.Fields(address), " ")

	return strings.TrimSpace(address)
}
