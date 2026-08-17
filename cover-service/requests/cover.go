package requests

type CoverRequest struct {
	Read   *CoverReadRequest
	Add    *CoverAddRequest
	Delete *CoverDeleteRequest
}

func NewCoverRequest() *CoverRequest {
	return &CoverRequest{
		Read:   &CoverReadRequest{},
		Add:    &CoverAddRequest{},
		Delete: &CoverDeleteRequest{},
	}
}

type CoverReadRequest struct {
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}

type CoverAddRequest struct {
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}

type CoverDeleteRequest struct {
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}
