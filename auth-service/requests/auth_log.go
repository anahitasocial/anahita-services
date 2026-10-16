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
	// The person's id.
	ID int64 `param:"id" validate:"required"`
}

type AuthLogDeleteRequest struct {
	// The person's id.
	UserID int64 `param:"uid" validate:"required"`
	// The session's id.
	ID int64 `param:"id" validate:"required"`
}
