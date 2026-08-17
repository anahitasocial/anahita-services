package requests

type AuthlogRequest struct {
	Browse *AuthlogBrowseRequest
	Delete *AuthlogDeleteRequest
}

func NewAuthlogRequest() *AuthlogRequest {
	return &AuthlogRequest{
		Browse: &AuthlogBrowseRequest{},
		Delete: &AuthlogDeleteRequest{},
	}
}

type AuthlogBrowseRequest struct {
	ID int64 `param:"id" validate:"required"`
}

type AuthlogDeleteRequest struct {
	UserID int64 `param:"uid" validate:"required"`
	ID     int64 `param:"id" validate:"required"`
}
