package handlers

import (
	"log"
	"net/http"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"anahita-libs/requests"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A group's and an event's rules (roadmap row 102, part 5): written by its
// administrators, at most ten, each a short title and a line more if it
// needs one; ticked one by one by whoever follows it or answers it; shown on
// its profile at all times.

// Rule is one of a profile's rules.
type Rule struct {
	// A few words: "Be kind".
	Title string `json:"title" validate:"required,max=100"`
	// A line more, if it needs one.
	Body string `json:"body" validate:"max=300"`
}

// Rules is a profile's rules, as they are now.
type Rules struct {
	// Raised by every change; 0 for a profile that never had any.
	Version int64 `json:"version"`
	// When they last changed, RFC 3339, or empty.
	UpdatedAt string `json:"updated_at"`
	Items     []Rule `json:"items"`
	// The version the viewer last agreed to, 0 for none: behind Version,
	// they are asked again.
	AgreedVersion int64 `json:"agreed_version"`
}

// RulesSetRequest is a profile's rules as they are to be from now on.
type RulesSetRequest struct {
	ID    int64  `param:"id"`
	Items []Rule `json:"items" validate:"max=10,dive"`
}

// RulesAgreeRequest is the viewer agreeing to the version shown them.
type RulesAgreeRequest struct {
	ID      int64 `param:"id"`
	Version int64 `json:"version" validate:"required,gt=0"`
}

func rulesFromGrpc(rules *graphpb.ProfileRules, agreed int64) Rules {
	out := Rules{Items: []Rule{}, AgreedVersion: agreed}
	if rules == nil {
		return out
	}
	out.Version = rules.Version
	out.UpdatedAt = rules.UpdatedAt
	for _, item := range rules.Items {
		out.Items = append(out.Items, Rule{Title: item.Title, Body: item.Body})
	}
	return out
}

// ReadRules is a profile's rules, for whoever may see it, and for whoever it
// would let ask to follow: they are what somebody agrees to before joining.
//
//	GET /groups/:id/rules, GET /events/:id/rules
func (h *groupHandler) ReadRules(c echo.Context) error {
	req := requests.NewActorRequest().Read
	viewer := getViewer(c)
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	accessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, _ := h.Perms.Actors.CanRead(accessCtx); !ok && limitedProfile(h.Env, actor, accessCtx) == nil {
		return c.NoContent(http.StatusForbidden)
	}

	read, err := h.Srv.ActorsGRPC.ReadRules(actor.ID, viewer.ID)
	if err != nil {
		log.Println("rules:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, rulesFromGrpc(read.GetRules(), read.GetAgreedVersion()))
}

// SetRules replaces a profile's rules, for its administrators: whoever may
// change the profile. An empty list takes them away.
//
//	PUT /groups/:id/rules, PUT /events/:id/rules
func (h *groupHandler) SetRules(c echo.Context) error {
	req := &RulesSetRequest{}
	viewer := getViewer(c)
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, responses.Refusal{Error: "rules_invalid"})
	}
	actor, refused := h.editableActor(c, req.ID, viewer)
	if refused != nil {
		return refused
	}

	items := make([]*graphpb.ProfileRule, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, &graphpb.ProfileRule{Title: item.Title, Body: item.Body})
	}
	set, err := h.Srv.ActorsGRPC.SetRules(actor.ID, items, viewer)
	if status.Code(err) == codes.InvalidArgument {
		return c.JSON(http.StatusBadRequest, responses.Refusal{Error: "rules_invalid"})
	}
	if err != nil {
		log.Println("rules:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, rulesFromGrpc(set, 0))
}

// AgreeRules records the viewer agreeing, each rule ticked, to the version
// they were shown. Another version is refused: the rules changed since.
//
//	POST /groups/:id/rules/agree, POST /events/:id/rules/agree
func (h *groupHandler) AgreeRules(c echo.Context) error {
	req := &RulesAgreeRequest{}
	viewer := getViewer(c)
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if _, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer); err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	err := h.Srv.ActorsGRPC.AgreeRules(req.ID, req.Version, viewer)
	if status.Code(err) == codes.FailedPrecondition {
		return c.JSON(http.StatusConflict, responses.Refusal{Error: "rules_not_current"})
	}
	if err != nil {
		log.Println("rules:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusNoContent)
}

// editableActor reads a profile of this handler's kind for whoever may
// change it, or answers why not.
func (h *groupHandler) editableActor(c echo.Context, id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	actor, err := h.Srv.ActorsGRPC.ReadOfType(id, h.nodeType, viewer)
	if err != nil {
		return nil, c.NoContent(http.StatusNotFound)
	}
	accessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return nil, c.NoContent(http.StatusInternalServerError)
	}
	if ok, _ := h.Perms.Actors.CanEdit(accessCtx); !ok {
		return nil, c.NoContent(http.StatusForbidden)
	}
	return actor, nil
}
