package requests

import "anahita-libs/constants"

type UserRequest struct {
	Browse *UserBrowseRequest
	Read   *UserReadRequest
	Edit   *UserEditRequest
	Add    *UserAddRequest
	Delete *UserDeleteRequest
}

func NewUserRequest() *UserRequest {
	return &UserRequest{
		Browse: &UserBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "id",
		},
		Read:   &UserReadRequest{},
		Edit:   &UserEditRequest{},
		Add:    &UserAddRequest{},
		Delete: &UserDeleteRequest{},
	}
}

type UserBrowseRequest struct {
	Offset    uint64 `query:"offset"`
	Limit     uint64 `query:"limit"`
	Keywords  string `query:"q" validate:"max=100,omitempty,alphanumunicode"`
	UserType  string `query:"type" validate:"max=20,omitempty,oneof=registered administrator super-administrator"`
	Ordering  string `query:"ordering" validate:"max=15,omitempty,oneof=id created_on usertype enabled"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Disabled  bool   `query:"disabled" validate:"omitempty"`
	UserTypes []string
}

type UserReadRequest struct {
	ID       int64  `param:"id" validate:"required,numeric"`
	Username string `param:"username" validate:"required,max=32,username"`
}

type UserEditRequest struct {
	ID       int64  `param:"id" validate:"required,numeric"`
	Username string `form:"username" validate:"required,min=3,max=32,username"`
	Email    string `form:"email" validate:"required,min=3,max=64,email"`
}

type UserAddRequest struct{}

type UserDeleteRequest struct {
	ID       int64  `param:"id"`
	Username string `param:"username" validate:"required,min=3,max=64,username"`
}
