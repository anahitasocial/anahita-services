package sessions

import "time"

const (
	SessionIDLength     = 128
	SessionTimeout      = 7 * 24 * time.Hour  // 7 days
	OAuthSessionTimeout = 30 * 24 * time.Hour // 30 days
)
