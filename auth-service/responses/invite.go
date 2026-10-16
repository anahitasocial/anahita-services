package responses

import (
	"auth/data/entities"

	"anahita-libs/responses"
)

// InviteItem is an invitation, as its sender sees it. Not the link: that
// is in the mail that was sent, and nowhere else.
type InviteItem struct {
	// Its id: what taking it back names.
	ID int64 `json:"id"`
	// Who it was sent to.
	RecipientEmail string `json:"recipient_email"`
	// `pending`, `used` or `expired`.
	Status string `json:"status"`
	// When it was sent.
	CreatedAt string `json:"created_at"`
	// Who sent it: the viewer.
	Author *responses.Author `json:"author"`
}

type InviteResponse struct{}

func (r *InviteResponse) MapInviteItem(token *entities.Token) *InviteItem {
	authorResponse := &responses.Author{}
	return &InviteItem{
		ID:             token.ID,
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
