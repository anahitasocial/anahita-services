package requests

import "anahita-libs/constants"

type UserRequest struct {
	Browse *UserBrowseRequest
}

func NewUserRequest() *UserRequest {
	return &UserRequest{
		Browse: &UserBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "id",
		},
	}
}

type UserBrowseRequest struct {
	Offset      uint64 `query:"offset"`
	Limit       uint64 `query:"limit"`
	Keywords    string `query:"q" validate:"max=100,omitempty,alphanumunicode"`
	PersonType  string `query:"type" validate:"max=20,omitempty,oneof=registered administrator super-administrator"`
	Ordering    string `query:"ordering" validate:"max=15,omitempty,oneof=id created_at person_type enabled"`
	Direction   string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Disabled    bool   `query:"disabled" validate:"omitempty"`
	PersonTypes []string
}
