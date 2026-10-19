package handlers

import (
	"log"
	"net/http"

	libs_constants "anahita-libs/constants"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Changing a person's type: registered, administrator or super administrator
// (the user, 2026-10-18). An administrator may make somebody registered an
// administrator; making a super administrator, and taking a type away, are a
// super administrator's (constants.MayChangePersonType). Always after proving
// who they are, and never their own, which graph-grpc refuses so that the last
// super administrator can never be demoted (decided 2026-09-07,
// docs/plans/actor-profile-management.md).
//
// It takes effect at the person's next token, within the access token's
// fifteen minutes: a token is not checked again once issued, and every new
// one reads the type from the database. Signing them out would not shorten
// that, so it is not done.

// PersonTypeRequest is the type to give somebody.
type PersonTypeRequest struct {
	// The person's id.
	ID int64 `param:"id" validate:"required,gt=0"`
	// registered, administrator or super-administrator.
	PersonType string `json:"person_type" validate:"required,oneof=registered administrator super-administrator"`
}

// PersonTypeChanged is a person's type, and whether this changed it.
type PersonTypeChanged struct {
	ID int64 `json:"id"`
	// The type they have now.
	PersonType string `json:"person_type"`
	// The type they had before.
	Previous string `json:"previous"`
	// False when they already had it.
	Changed bool `json:"changed"`
}

// EditPersonType gives a person a type.
//
//	PATCH /accounts/:id/person-type   { "person_type": "administrator" }
func (h *accountsHandler) EditPersonType(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.Account.CanEditPersonType(viewer); err != nil {
		log.Println("person type:", err)
		return c.NoContent(http.StatusForbidden)
	}

	req := &PersonTypeRequest{}
	if err := c.Bind(req); err != nil {
		log.Println("person type:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println("person type:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Before the proof is asked for, so nobody proves who they are only to
	// be told they could not have done it.
	if req.ID == viewer.ID {
		return refuseOwnPersonType(c)
	}

	// Whether this viewer may make this change, against the type the person
	// has now; graph-grpc asks the same again, where it writes.
	person, err := h.Repos.Users.ReadByID(req.ID)
	if err != nil || person == nil {
		return refuseNotAPerson(c)
	}
	if person.PersonType == req.PersonType {
		return c.JSON(http.StatusOK, &PersonTypeChanged{
			ID:         req.ID,
			PersonType: req.PersonType,
			Previous:   person.PersonType,
			Changed:    false,
		})
	}
	if !libs_constants.MayChangePersonType(viewer.PersonType, person.PersonType, req.PersonType) {
		return refusePersonTypeNotAllowed(c)
	}

	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("person type:", err)
		return stepUpRequired(c, err)
	}

	changed, previous, err := h.Srv.ActorsGRPC.SetPersonType(req.ID, req.PersonType, viewer)
	switch status.Code(err) {
	case codes.OK:
	case codes.NotFound:
		return refuseNotAPerson(c)
	case codes.FailedPrecondition:
		return refuseOwnPersonType(c)
	case codes.PermissionDenied:
		return refusePersonTypeNotAllowed(c)
	case codes.Unauthenticated:
		return c.NoContent(http.StatusForbidden)
	default:
		log.Println("person type:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One proof, one change.
	h.clearStepUp(viewer.ID)

	return c.JSON(http.StatusOK, &PersonTypeChanged{
		ID:         req.ID,
		PersonType: req.PersonType,
		Previous:   previous,
		Changed:    changed,
	})
}

// refuseOwnPersonType is the 409 for changing one's own type, saying how a
// super administrator who wants to step down does it.
func refuseOwnPersonType(c echo.Context) error {
	return refuse(c, http.StatusConflict, "own_person_type",
		"Nobody changes their own account type. Another super "+
			"administrator can change it; to step down, make somebody else "+
			"a super administrator first, and they can change yours.")
}

func refuseNotAPerson(c echo.Context) error {
	return refuse(c, http.StatusNotFound, "not_a_person",
		"There is no person here with that id.")
}

// refusePersonTypeNotAllowed is an administrator asking for what is a super
// administrator's: making a super administrator, or taking a type away.
func refusePersonTypeNotAllowed(c echo.Context) error {
	return refuse(c, http.StatusForbidden, "person_type_not_allowed",
		"An administrator may make somebody registered an administrator. "+
			"Making a super administrator, and taking an account type away, "+
			"are a super administrator's.")
}
