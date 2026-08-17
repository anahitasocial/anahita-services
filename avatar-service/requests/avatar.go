package requests

type AvatarRequest struct {
	Read   *AvatarReadRequest
	Add    *AvatarAddRequest
	Delete *AvatarDeleteRequest
}

func NewAvatarRequest() *AvatarRequest {
	return &AvatarRequest{
		Read:   &AvatarReadRequest{},
		Add:    &AvatarAddRequest{},
		Delete: &AvatarDeleteRequest{},
	}
}

type AvatarReadRequest struct {
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}

type AvatarAddRequest struct {
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}

type AvatarDeleteRequest struct {
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}
