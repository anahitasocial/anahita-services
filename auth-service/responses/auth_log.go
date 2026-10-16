package responses

import (
	"auth/data/entities"
	"time"
)

type AuthLogItem struct {
	// The session's id: what signing it out names.
	ID int64 `json:"id"`
	// When it began.
	CreatedAt time.Time `json:"created_at"`
	// The kind of device: `desktop`, `mobile`, `tablet` or `bot`.
	Device string `json:"device"`
	// Its operating system.
	OS string `json:"os"`
	// The app, or the browser.
	Name string `json:"name"`
	// Roughly where it began, from its address. Empty when not known.
	City string `json:"city"`
	// The country, likewise.
	Country string `json:"country"`
	// The app it signed in to.
	ClientID string `json:"client_id"`
	// Whether it is still signed in.
	IsActive bool `json:"is_active"`
	// Whether it is the session asking.
	IsViewer bool `json:"is_viewer"`
}

type AuthLogResponse struct{}

func (r *AuthLogResponse) MapAuthLogItem(authLog *entities.AuthLog) *AuthLogItem {
	return &AuthLogItem{
		ID:        authLog.ID,
		CreatedAt: authLog.CreatedAt,
		Device:    authLog.Device,
		OS:        authLog.OS,
		Name:      authLog.Name,
		City:      authLog.City,
		Country:   authLog.Country,
		ClientID:  authLog.ClientID,
		IsActive:  authLog.IsActive,
		IsViewer:  authLog.IsViewer,
	}
}

func (r *AuthLogResponse) MapAuthLogItems(authLogs []*entities.AuthLog) []*AuthLogItem {
	items := make([]*AuthLogItem, 0, len(authLogs))
	for _, authLog := range authLogs {
		items = append(items, r.MapAuthLogItem(authLog))
	}
	return items
}
