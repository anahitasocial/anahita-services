package sessions

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

// GenerateRandomString generates a random string of length n
func GenerateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}

// removalCookie is what takes a cookie away: expired, and otherwise set as it
// was given, so that a browser treats it as the same cookie.
func removalCookie(name string, dev bool) *http.Cookie {
	sameSite := http.SameSiteStrictMode
	if dev {
		sameSite = http.SameSiteLaxMode
	}

	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   !dev,
		SameSite: sameSite,
	}
}

// HashString hashes a string
func HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)
}
