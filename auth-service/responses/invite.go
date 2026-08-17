package responses

import (
	"auth/data/entities"
	"strconv"

	"anahita-libs/responses"
)

type InviteItem struct {
	ID             string            `json:"id"`
	RecipientEmail string            `json:"recipient_email"`
	Status         string            `json:"status"`
	CreatedAt      string            `json:"created_at"`
	Author         *responses.Author `json:"author"`
}

type InviteResponse struct{}

func (r *InviteResponse) MapInviteItem(token *entities.Token) *InviteItem {
	authorResponse := &responses.Author{}
	return &InviteItem{
		ID:             strconv.FormatInt(token.ID, 10),
		RecipientEmail: token.RecipientEmail,
		Status:         token.Status,
		CreatedAt:      token.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Author:         authorResponse.MapAuthor(token.Creator),
	}
}

func (r *InviteResponse) MapInviteItems(tokens []*entities.Token) []*InviteItem {
	inviteItems := make([]*InviteItem, 0)
	for _, token := range tokens {
		inviteItems = append(inviteItems, r.MapInviteItem(token))
	}
	return inviteItems
}
