package handlers

import (
	"errors"
	"fmt"
	"testing"

	"auth/data/entities"
	"auth/responses"
)

// TestRefreshErrorCode pins the mapping clients branch on.
//
// Every refresh failure goes out as 400 invalid_grant per RFC 6749, so
// error_code is the only signal a client has for deciding whether to end
// the session or retry. Two directions matter here and are easy to break:
// refresh_raced must never be treated as terminal (it would log out a user
// whose token was merely rotated by a concurrent request), and an
// unrecognised error must map to "" so clients fail closed and re-login
// rather than silently retrying forever.
func TestRefreshErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "expired refresh token",
			err:  entities.ErrRefreshTokenExpired,
			want: responses.OAuthErrorCodeRefreshExpired,
		},
		{
			name: "unknown refresh token",
			err:  entities.ErrRefreshTokenNotFound,
			want: responses.OAuthErrorCodeRefreshNotFound,
		},
		{
			name: "reuse detected",
			err:  entities.ErrRefreshTokenReuse,
			want: responses.OAuthErrorCodeRefreshReuse,
		},
		{
			name: "lost a concurrent rotation",
			err:  entities.ErrRefreshTokenRaced,
			want: responses.OAuthErrorCodeRefreshRaced,
		},
		{
			name: "wrapped sentinel is still recognised",
			err:  fmt.Errorf("resolve person: %w", entities.ErrRefreshTokenRaced),
			want: responses.OAuthErrorCodeRefreshRaced,
		},
		{
			name: "unrecognised error yields no code",
			err:  errors.New("refresh token does not belong to this client"),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := refreshErrorCode(tt.err); got != tt.want {
				t.Errorf("refreshErrorCode(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

// TestRefreshErrorCodesAreDistinct guards against a copy-paste slip that
// would make two failure modes indistinguishable to the client.
func TestRefreshErrorCodesAreDistinct(t *testing.T) {
	codes := []string{
		responses.OAuthErrorCodeRefreshExpired,
		responses.OAuthErrorCodeRefreshNotFound,
		responses.OAuthErrorCodeRefreshReuse,
		responses.OAuthErrorCodeRefreshRaced,
	}

	seen := make(map[string]bool, len(codes))
	for _, code := range codes {
		if code == "" {
			t.Error("an error_code constant must not be empty")
		}
		if seen[code] {
			t.Errorf("duplicate error_code: %s", code)
		}
		seen[code] = true
	}
}
