package responses

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/responses"
	"notification/data/entities"
	"time"
)

// Subject is who did what a notification tells of.
type Subject struct {
	// Its id.
	ID int64 `json:"id"`
	// What kind of thing it is.
	Type string `json:"type,omitempty"`
	// Its handle, or its short name.
	Alias string `json:"alias,omitempty"`
	// Its name, or its title.
	Name string `json:"name,omitempty"`
	// Their picture, in each size there is.
	Avatar map[string]responses.Image `json:"avatar_urls,omitempty"`
}

// Object is what was done: a post, a reply, a like.
type Object struct {
	// Its id.
	ID int64 `json:"id"`
	// What kind of thing it is.
	Type string `json:"type"`
	// The service it belongs to.
	Service string `json:"service,omitempty"`
	// A profile's name. Left out on a post, which has a title instead.
	Name string `json:"name,omitempty"`
	// A post's title, where it has one.
	Title string `json:"title,omitempty"`
	// Its handle, or its short name.
	Alias string `json:"alias,omitempty"`
	// A profile's description. Left out on a post, whose words are its
	// content.
	Body string `json:"body,omitempty"`
	// A post's words, as Markdown.
	Content string `json:"content,omitempty"`
	// A photo post's first picture, in each size there is.
	Portrait map[string]responses.Image `json:"portrait_urls,omitempty"`
}

// Target is what it was done to: the viewer's post, or the viewer.
type Target struct {
	// Its id.
	ID int64 `json:"id"`
	// What kind of thing it is.
	Type string `json:"type,omitempty"`
	// Its handle, or its short name.
	Alias string `json:"alias,omitempty"`
	// A profile's name. Left out on a post, which has a title instead.
	Name string `json:"name,omitempty"`
	// A post's title, where it has one.
	Title string `json:"title,omitempty"`
}

// NotificationItem is something the viewer is told of.
type NotificationItem struct {
	// Its id.
	ID int64 `json:"id"`
	// What it tells of, as the event that made it is named.
	Type string `json:"type"`
	// The service that told it.
	Service     string `json:"service"`
	RecipientID int64  `json:"-"`
	// Who did it.
	Subject *Subject `json:"subject"`
	// What was done.
	Object *Object `json:"object"`
	// What it was done to.
	Target *Target `json:"target"`
	// When it was made.
	CreatedAt time.Time `json:"created_at"`
	// When it last changed: when it was read, for one.
	UpdatedAt time.Time `json:"updated_at"`
	// Whether the viewer has read it.
	IsRead bool `json:"is_read"`
}

// UnreadCount is how many notifications the viewer has not read.
type UnreadCount struct {
	// How many.
	Count uint64 `json:"count"`
}

type NotificationResponse struct{}

func (r *NotificationResponse) MapNotificationItem(item *entities.Notification) *NotificationItem {
	if item == nil {
		return nil
	}

	return &NotificationItem{
		ID:          item.ID,
		Type:        item.Type,
		Service:     item.Service,
		RecipientID: item.RecipientID,
		Subject:     r.MapSubject(item.Subject),
		Object:      r.MapObject(item.Object),
		Target:      r.MapTarget(item.Target),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		IsRead:      item.IsRead,
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

	dto := &Object{
		ID:      object.ID,
		Type:    object.Type,
		Service: object.Service,
		Alias:   object.Alias,
		Portrait: func() map[string]responses.Image {
			if object.Portrait != nil {
				return responses.NewPortrait(object.Portrait)
			}
			return nil
		}(),
	}
	if responses.IsPost(object.Type) {
		dto.Title, dto.Content = object.Name, object.Body
	} else {
		dto.Name, dto.Body = object.Name, object.Body
	}
	return dto
}

func (r *NotificationResponse) MapTarget(target *an_entities.AnahitaNode) *Target {
	if target == nil {
		return nil
	}

	dto := &Target{
		ID:    target.ID,
		Type:  target.Type,
		Alias: target.Alias,
	}
	if responses.IsPost(target.Type) {
		dto.Title = target.Name
	} else {
		dto.Name = target.Name
	}
	return dto
}

func (r *NotificationResponse) MapNotificationItems(items []*entities.Notification) []*NotificationItem {
	var responseItems []*NotificationItem
	for _, item := range items {
		responseItems = append(responseItems, r.MapNotificationItem(item))
	}
	return responseItems
}
