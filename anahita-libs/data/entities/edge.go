package entities

import "time"

func NewAnahitaEdge() *AnahitaEdge {
	return &AnahitaEdge{}
}

type AnahitaEdge struct {
	ID         int64     `json:"id"`
	Type       string    `json:"type"`
	NodeAID    int64     `json:"node_a_id"`
	NodeAType  string    `json:"node_a_type"`
	NodeBID    int64     `json:"node_b_id"`
	NodeBType  string    `json:"node_b_type"`
	Meta       string    `json:"meta,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	CreatedBy  int64     `json:"created_by,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	ModifiedBy int64     `json:"modified_by,omitempty"`
	Ordering   int32     `json:"ordering,omitempty"`
	StartsAt   time.Time `json:"starts_at,omitempty"`
	EndsAt     time.Time `json:"ends_at,omitempty"`
}
