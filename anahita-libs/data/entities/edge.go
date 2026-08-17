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
	CreatedOn  time.Time `json:"created_on,omitempty"`
	CreatedBy  int64     `json:"created_by,omitempty"`
	ModifiedOn time.Time `json:"modified_on,omitempty"`
	ModifiedBy int64     `json:"modified_by,omitempty"`
	Ordering   int32     `json:"ordering,omitempty"`
	StartDate  time.Time `json:"start_date,omitempty"`
	EndDate    time.Time `json:"end_date,omitempty"`
}
