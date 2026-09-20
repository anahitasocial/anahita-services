package requests

type AuthLogRequest struct {
	Browse *AuthLogBrowseRequest
	Delete *AuthLogDeleteRequest
}

func NewAuthLogRequest() *AuthLogRequest {
	return &AuthLogRequest{
		Browse: &AuthLogBrowseRequest{},
		Delete: &AuthLogDeleteRequest{},
	}
}

type AuthLogBrowseRequest struct {
	ID int64 `param:"id" validate:"required"`
}

type AuthLogDeleteRequest struct {
	UserID int64 `param:"uid" validate:"required"`
	ID     int64 `param:"id" validate:"required"`
}
