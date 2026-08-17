package entities

import (
	"errors"
	"time"
)

// AuthLog is the struct representation of the auth_log table that stores authentication logs data
type AuthLog struct {
	ID                  int64     `json:"id"`
	CreatedOn           time.Time `json:"createdOn"`
	CreatedBy           int64     `json:"-"`
	Device              string    `json:"device"`
	OS                  string    `json:"os"`
	Name                string    `json:"name"`
	IP                  string    `json:"-"`
	City                string    `json:"city"`
	Country             string    `json:"country"`
	OAuthRefreshTokenID int64     `json:"-"`
	ClientID            string    `json:"clientId"`
	IsActive            bool      `json:"isActive" db:"-"`
	IsViewer            bool      `json:"isViewer" db:"-"`
}

func NewAuthLog() *AuthLog {
	return &AuthLog{}
}

// Validate checks if the authlog fields are valid
func (a *AuthLog) Validate() error {
	if a.CreatedBy == 0 {
		return errors.New("created_by is required")
	}

	if a.Device == "" {
		return errors.New("device is required")
	}

	if a.OS == "" {
		return errors.New("os is required")
	}

	if a.Name == "" {
		return errors.New("name is required")
	}

	if a.IP == "" {
		return errors.New("ip is required")
	}

	if a.City == "" {
		return errors.New("city is required")
	}

	if a.Country == "" {
		return errors.New("country is required")
	}

	if a.OAuthRefreshTokenID == 0 {
		return errors.New("oauth_refresh_token_id is required")
	}

	if a.ClientID == "" {
		return errors.New("client_id is required")
	}

	return nil
}
