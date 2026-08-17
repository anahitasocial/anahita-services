package responses

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/responses"
	"notification/data/entities"
	"time"
)

type Subject struct {
	ID     int64                      `json:"id"`
	Type   string                     `json:"type,omitempty"`
	Alias  string                     `json:"alias,omitempty"`
	Name   string                     `json:"name,omitempty"`
	Avatar map[string]responses.Image `json:"avatar_urls,omitempty"`
}

type Object struct {
	ID       int64                      `json:"id"`
	Type     string                     `json:"type"`
	Service  string                     `json:"service,omitempty"`
	Name     string                     `json:"name,omitempty"`
	Alias    string                     `json:"alias,omitempty"`
	Body     string                     `json:"body,omitempty"`
	Portrait map[string]responses.Image `json:"portrait_urls,omitempty"`
}

type Target struct {
	ID    int64  `json:"id"`
	Type  string `json:"type,omitempty"`
	Alias string `json:"alias,omitempty"`
	Name  string `json:"name,omitempty"`
}

type NotificationItem struct {
	ID           int64     `json:"id"`
	Type         string    `json:"type"`
	Service      string    `json:"service"`
	RecipientID  int64     `json:"-"`
	Subject      *Subject  `json:"subject"`
	Object       *Object   `json:"object"`
	Target       *Target   `json:"target"`
	CreationTime time.Time `json:"creationTime"`
	UpdateTime   time.Time `json:"updateTime"`
	Meta         string    `json:"meta"`
	IsRead       bool      `json:"isRead"`
}

type NotificationResponse struct{}

func (r *NotificationResponse) MapNotificationItem(item *entities.Notification) *NotificationItem {
	if item == nil {
		return nil
	}

	return &NotificationItem{
		ID:           item.ID,
		Type:         item.Type,
		Service:      item.Service,
		RecipientID:  item.RecipientID,
		Subject:      r.MapSubject(item.Subject),
		Object:       r.MapObject(item.Object),
		Target:       r.MapTarget(item.Target),
		CreationTime: item.CreationTime,
		UpdateTime:   item.UpdateTime,
		Meta:         item.Meta,
		IsRead:       item.IsRead,
	}
}

func (r *NotificationResponse) MapSubject(subject *an_entities.AnahitaNode) *Subject {
	if subject == nil {
		return nil
	}

	return &Subject{
		ID:    subject.ID,
		Type:  subject.Type,
		Alias: subject.Alias,
		Name:  subject.Name,
		Avatar: func() map[string]responses.Image {
			if subject.Avatar != nil {
				return responses.NewAvatar(subject.Avatar)
			}
			return nil
		}(),
	}
}

func (r *NotificationResponse) MapObject(object *an_entities.AnahitaNode) *Object {
	if object == nil {
		return nil
	}

	return &Object{
		ID:      object.ID,
		Type:    object.Type,
		Service: object.Service,
		Name:    object.Name,
		Alias:   object.Alias,
		Body:    object.Body,
		Portrait: func() map[string]responses.Image {
			if object.Portrait != nil {
				return responses.NewPortrait(object.Portrait)
			}
			return nil
		}(),
	}
}

func (r *NotificationResponse) MapTarget(target *an_entities.AnahitaNode) *Target {
	if target == nil {
		return nil
	}

	return &Target{
		ID:    target.ID,
		Type:  target.Type,
		Alias: target.Alias,
		Name:  target.Name,
	}
}

func (r *NotificationResponse) MapNotificationItems(items []*entities.Notification) []*NotificationItem {
	var responseItems []*NotificationItem
	for _, item := range items {
		responseItems = append(responseItems, r.MapNotificationItem(item))
	}
	return responseItems
}
