package feeds

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cursor is a place in a feed: the last post of the page before. The next
// page is whatever is older than it, so a post written while somebody is
// reading neither repeats one nor pushes one out of sight, as counting from
// the top would.
type Cursor struct {
	CreatedAt time.Time
	ID        int64
}

// IsZero reports the start of the feed.
func (c Cursor) IsZero() bool {
	return c.ID == 0
}

// String is the cursor as it is sent: opaque to whoever holds it.
func (c Cursor) String() string {
	if c.IsZero() {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%d.%d", c.CreatedAt.Unix(), c.ID))
}

// ParseCursor reads a cursor back. Empty is the start of the feed.
func ParseCursor(value string) (Cursor, error) {
	if value == "" {
		return Cursor{}, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, fmt.Errorf("cursor: %w", err)
	}

	seconds, id, found := strings.Cut(string(raw), ".")
	if !found {
		return Cursor{}, fmt.Errorf("cursor: malformed")
	}

	at, err := strconv.ParseInt(seconds, 10, 64)
	if err != nil || at <= 0 {
		return Cursor{}, fmt.Errorf("cursor: malformed")
	}
	nodeID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || nodeID <= 0 {
		return Cursor{}, fmt.Errorf("cursor: malformed")
	}

	return Cursor{CreatedAt: time.Unix(at, 0).UTC(), ID: nodeID}, nil
}
