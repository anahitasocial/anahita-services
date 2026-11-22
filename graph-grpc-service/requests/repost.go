package requests

type RepostRequest struct {
	Browse *RepostBrowseRequest
	Add    *RepostAddRequest
	Delete *RepostDeleteRequest
}

type RepostBrowseRequest struct {
	Offset      uint64 `query:"start"`
	Limit       uint64 `query:"limit"`
	Ordering    string `query:"ordering" validate:"max=15,omitempty,oneof=updated_at created_at enabled"`
	OrderingDir string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	ParentID    int64  `param:"parent_id" validate:"required,numeric"`
}

type RepostAddRequest struct {
	ParentID int64 `param:"parent_id" validate:"required,numeric"`
}

type RepostDeleteRequest struct {
	ParentID int64 `param:"parent_id" validate:"required,numeric"`
}
