package responses

import (
	"auth/data/entities"
	"time"
)

type AuthLogItem struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Device    string    `json:"device"`
	OS        string    `json:"os"`
	Name      string    `json:"name"`
	City      string    `json:"city"`
	Country   string    `json:"country"`
	ClientID  string    `json:"client_id"`
	IsActive  bool      `json:"is_active"`
	IsViewer  bool      `json:"is_viewer"`
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
