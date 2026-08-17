package responses

import (
	"auth/data/entities"
	"time"
)

type AuthlogItem struct {
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

type AuthlogResponse struct{}

func (r *AuthlogResponse) MapAuthlogItem(authlog *entities.AuthLog) *AuthlogItem {
	return &AuthlogItem{
		ID:        authlog.ID,
		CreatedAt: authlog.CreatedOn,
		Device:    authlog.Device,
		OS:        authlog.OS,
		Name:      authlog.Name,
		City:      authlog.City,
		Country:   authlog.Country,
		ClientID:  authlog.ClientID,
		IsActive:  authlog.IsActive,
		IsViewer:  authlog.IsViewer,
	}
}

func (r *AuthlogResponse) MapAuthlogItems(authlogs []*entities.AuthLog) []*AuthlogItem {
	items := make([]*AuthlogItem, 0, len(authlogs))
	for _, authlog := range authlogs {
		items = append(items, r.MapAuthlogItem(authlog))
	}
	return items
}
