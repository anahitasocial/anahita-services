package requests

type SignupRequest struct {
	Add *SignupAddRequest
}

func NewSignupRequest() *SignupRequest {
	return &SignupRequest{
		Add: &SignupAddRequest{},
	}
}

// SignupAddRequest is the public account-creation request, submitted as JSON
// by the web app.
//
// It and InviteAccept create the same thing by two routes and had drifted into
// two different sets of rules — six characters here against fifteen there, a
// name required there and not collected here at all. They now match.
type SignupAddRequest struct {
	// Name is the display name. It goes to nodes.name, which is THE name
	// everywhere now, so signup has to collect it — a Go-created account used
	// to land with none at all, and the 823 legacy people only have one
	// because they predate this path.
	//
	// No alphanumunicode: that tag is ^[\p{L}\p{N}]+$, which rejects the space
	// between two words along with Jean-Luc, O'Brien and van der Berg.
	Name     string `form:"name" json:"name" validate:"required,min=1,max=100"`
	Email    string `form:"email" validate:"required,email,min=3,max=64"`
	Username string `form:"username" validate:"required,username,min=3,max=32"`
	// Fifteen, matching invite-accept. Six was the outlier, and the asymmetry
	// ran the wrong way: the public path anyone can reach had the weaker rule.
	Password string `form:"password" validate:"required,min=15,max=150"`

	// What the person agreed to, and which version of it they were shown.
	//
	// `eq=true` rather than `required`: a bool that is false IS its zero
	// value, so `required` passes an unticked box straight through. The
	// version is checked against the one in force by the handler, not here —
	// this only enforces its shape.
	TosAccepted bool   `form:"tos_accepted" json:"tos_accepted" validate:"eq=true"`
	TosVersion  string `form:"tos_version" json:"tos_version" validate:"required,max=20"`
	PpAccepted  bool   `form:"pp_accepted" json:"pp_accepted" validate:"eq=true"`
	PpVersion   string `form:"pp_version" json:"pp_version" validate:"required,max=20"`
}
