package requests

import "fmt"

// ActorPurgeBatchMax is how many actors one request may name. A cleanup
// larger than this is several requests, each confirmed.
const ActorPurgeBatchMax = 500

// ActorPurgeBatchRequest asks for several actors to be destroyed at once.
//
// Confirm is the count, typed out: "PURGE 37" for thirty-seven actors. It is
// checked against the list, so a selection that changed after the dialog was
// read does not go through on the old confirmation.
type ActorPurgeBatchRequest struct {
	// The profiles: up to 500.
	IDs []int64 `json:"ids" validate:"required,min=1,max=500,dive,gt=0"`
	// `PURGE` and how many there are (`PURGE 12`), typed, to say it is meant.
	Confirm string `json:"confirm" validate:"required,max=20"`
}

// Confirmed reports whether Confirm names the number of actors in IDs.
func (r *ActorPurgeBatchRequest) Confirmed() bool {
	return r.Confirm == fmt.Sprintf("PURGE %d", len(r.IDs))
}

// ActorPurgeBatchReadRequest asks how far a purge request has got.
type ActorPurgeBatchReadRequest struct {
	// What the purge request answered as `batch_id`.
	BatchID string `param:"batch_id" validate:"required,uuid"`
}
