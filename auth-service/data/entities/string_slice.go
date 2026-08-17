package entities

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringSlice is a []string that serializes to/from a JSON column.
// Used for oauth_clients.redirect_uris, grant_types, scopes, and
// webauthn_credentials.transports.
//
// MySQL has no array type, so these are JSON columns rather than the
// Postgres TEXT[]/JSONB the service originally targeted. The driver
// hands JSON back as []byte and takes it as []byte, both of which this
// type already handles, so the same implementation covers both engines.
type StringSlice []string

// Scan implements sql.Scanner for JSON columns.
func (s *StringSlice) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("entities: cannot scan %T into StringSlice", src)
	}
	if len(b) == 0 {
		*s = nil
		return nil
	}
	return json.Unmarshal(b, s)
}

// Value implements driver.Valuer for JSONB columns.
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(s)
}

// Contains reports whether s includes v.
func (s StringSlice) Contains(v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}
