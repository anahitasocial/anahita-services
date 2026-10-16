// Package fixture is a small service's requests and answers, for apidoc's
// tests: their comments are read from here, as a service's are.
package fixture

import (
	"encoding/json"
	"time"
)

// Paging is embedded in a request, as a service's shared paging is.
type Paging struct {
	// How many to pass over.
	Start uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit" validate:"max=50"`
}

// BrowseRequest lists things.
type BrowseRequest struct {
	Paging
	// The order they come in.
	Sort string `query:"sort" validate:"omitempty,oneof=recent popular"`
	// Used inside the service, and never read from the request.
	Internal []int64
}

// ReadRequest names one thing.
type ReadRequest struct {
	// Its name.
	Alias string `param:"alias" validate:"required,max=100,custom_rule"`
	// Of another route's path, not this one's.
	OwnerID int64 `param:"owner_id"`
}

// DeleteRequest names one thing by its number.
type DeleteRequest struct {
	// Its number.
	ID int64 `param:"id" validate:"required"`
}

// AddRequest makes a thing.
type AddRequest struct {
	// What it is called.
	Name string `json:"name" validate:"required,min=1,max=20"`
	// How sure it is.
	Weight float64 `json:"weight,omitempty" validate:"gte=0,lte=1"`
	// A picture of it.
	Picture string `form:"picture"`
}

// Thing is something there is.
type Thing struct {
	Base
	// What it is called.
	Name string `json:"name,omitempty"`
	// When it was made.
	CreatedAt time.Time `json:"created_at"`
	// What else it is tied to, null when nothing is.
	Tags []string `json:"tags"`
	// What it came with, as it came.
	Meta json.RawMessage `json:"meta,omitempty"`
	// The thing it belongs to.
	Parent *Thing `json:"parent,omitempty"`
	// What is left out.
	Secret string `json:"-"`
	hidden string
}

// Base is what every thing has.
type Base struct {
	// Its number.
	ID int64 `json:"id"`
}
