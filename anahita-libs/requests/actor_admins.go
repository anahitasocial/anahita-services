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
	// The group's or the event's id.
	ActorID int64 `param:"id" validate:"required,number"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// Which way the order runs.
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
}

type ActorAdminsAddRequest struct {
	// The group's or the event's id.
	ActorID int64 `param:"id" validate:"required,number"`
	// The person to make an administrator.
	AdminID int64 `form:"admin_id" validate:"required,number"`
}

type ActorAdminsDeleteRequest struct {
	// The group's or the event's id.
	ActorID int64 `param:"id" validate:"required,number"`
	// The administrator to remove.
	AdminID int64 `param:"admin_id" form:"admin_id" validate:"required,number"`
}
