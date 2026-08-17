package entities

type SubscriptionStatus struct {
	SourceID     int64
	TargetID     int64
	IsSubscribed bool
}
