package entities

// uniqueString returns a slice of unique string values
func UniqueString(slice []string) []string {
	if len(slice) <= 1 {
		return slice
	}

	seen := make(map[string]struct{}, len(slice))
	result := make([]string, 0, len(slice))

	for _, v := range slice {
		if _, exists := seen[v]; !exists {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}

	return result
}
