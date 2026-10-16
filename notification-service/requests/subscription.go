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
	// The id of the post or the profile to be told about.
	TargetID int64 `form:"target_id" validate:"required"`
}

type SubscriptionDeleteRequest struct {
	// The id of the post or the profile.
	TargetID int64 `param:"target_id" validate:"required"`
}
