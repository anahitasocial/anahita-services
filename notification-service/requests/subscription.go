package requests

type SubscriptionRequest struct {
	Add    *SubscriptionAddRequest
	Delete *SubscriptionDeleteRequest
}

func NewSubscriptionRequest() *SubscriptionRequest {
	return &SubscriptionRequest{
		Add:    &SubscriptionAddRequest{},
		Delete: &SubscriptionDeleteRequest{},
	}
}

type SubscriptionAddRequest struct {
	TargetID int64 `form:"target_id" validate:"required"`
}

type SubscriptionDeleteRequest struct {
	TargetID int64 `param:"target_id" validate:"required"`
}
