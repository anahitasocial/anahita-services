package requests

type ActorAdminsRequest struct {
	Browse *ActorAdminsBrowseRequest
	Add    *ActorAdminsAddRequest
	Delete *ActorAdminsDeleteRequest
}

func NewActorAdminsRequest() *ActorAdminsRequest {
	return &ActorAdminsRequest{
		Browse: &ActorAdminsBrowseRequest{
			Offset:    0,
			Limit:     20,
			Direction: "asc",
		},
		Add:    &ActorAdminsAddRequest{},
		Delete: &ActorAdminsDeleteRequest{},
	}
}

type ActorAdminsBrowseRequest struct {
	ActorID   int64  `param:"id" validate:"required,number"`
	Offset    uint64 `query:"start"`
	Limit     uint64 `query:"limit"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
}

type ActorAdminsAddRequest struct {
	ActorID int64 `param:"id" validate:"required,number"`
	AdminID int64 `form:"admin_id" validate:"required,number"`
}

type ActorAdminsDeleteRequest struct {
	ActorID int64 `param:"id" validate:"required,number"`
	AdminID int64 `param:"admin_id" form:"admin_id" validate:"required,number"`
}
