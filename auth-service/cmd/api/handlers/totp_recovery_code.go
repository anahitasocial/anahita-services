package handlers

import (
	"math/rand"
	"time"
)

type totpRecoveryCodes struct{}

// Generates a string of recovery codes separated by a separator (,)
func (t *totpRecoveryCodes) generateCodes(count int, length int) []string {
	codes := make([]string, 0)

	i := 0
	for i < count {
		code := t.getCode(length)
		codes = append(codes, code)
		i++
	}

	return codes
}

// Generates a single recovery code
func (t *totpRecoveryCodes) getCode(length int) string {
	charset := "ABCDEFGHIKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}
