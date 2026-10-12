package handlers

import (
	"net/http"

	"activitypub/ap"
	"activitypub/data/repositories"

	libhandlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/constants"
	"anahita-libs/crypt"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	"anahita-libs/policy"
	"anahita-libs/site"

	"github.com/labstack/echo/v4"
)

// Env is what the handlers work with.
type Env struct {
	Site     site.Addresses
	Nodes    services.GraphNodeGRPCInterface
	Accounts repositories.AccountsInterface
	Keys     repositories.KeysInterface
	Crypt    crypt.Crypt
}

type Handlers struct {
	*Env
	addresses ap.Addresses
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{Env: env, addresses: ap.Addresses{Site: env.Site}}
}

func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}

func refuse(c echo.Context, status int, code string) error {
	return c.JSON(status, map[string]string{"error": code})
}

// Why a site, or a person on it, cannot be on the fediverse.
const (
	// The site keeps what is on it to its members, or shows visitors only
	// the start of things. Federating would send it all out.
	reasonSitePrivate = "site_private"
	// The site has no address another server could reach.
	reasonSiteUnreachable = "site_unreachable"
	// Only somebody whose profile is open to everybody can be followed
	// from elsewhere.
	reasonProfileNotPublic = "profile_not_public"
)

// siteFederates says whether this installation speaks to other servers at
// all, and why not when it does not.
//
// Only a site that is open to read federates: on one for members, or one
// that shows visitors a preview, there is nothing that may be sent out
// (see docs/privacy.md). And it has to be somewhere other servers can
// reach.
func (h *Handlers) siteFederates() (bool, string) {
	if policy.SiteReadAccess() != policy.ReadPublic {
		return false, reasonSitePrivate
	}
	if err := h.Site.CanFederate(); err != nil {
		return false, reasonSiteUnreachable
	}
	return true, ""
}

// mayFederate says whether a node is somebody who could be on the
// fediverse: a person, made here, whose profile anybody may see.
//
// Whether anybody may see it is asked of the graph, as a visitor: it knows
// whether they are disabled, archived or on their way to being deleted. And
// the profile must say of itself that it is public, not merely open to
// whoever has an account.
func (h *Handlers) mayFederate(person *entities.AnahitaNode) bool {
	if person == nil || person.ID == 0 || person.UUID == "" || person.IsRemote() {
		return false
	}
	if person.Type != constants.NodeActorPersonEntityType || !person.Enabled || person.DeletedAt != nil {
		return false
	}
	if person.Access != constants.AnahitaAccessPublic {
		return false
	}
	return libhandlers.MaySee(h.Nodes, person.ID, entities.NewViewer())
}

// federating is the person with this UUID if they are on the fediverse now:
// they could be, and they have chosen to be. Nil otherwise, whatever the
// reason: to another server, somebody who has not chosen to be known is
// simply not there.
func (h *Handlers) federating(person *entities.AnahitaNode) *entities.AnahitaNode {
	if ok, _ := h.siteFederates(); !ok {
		return nil
	}
	if !h.mayFederate(person) {
		return nil
	}
	account, err := h.Accounts.Read(person.ID)
	if err != nil || account == nil || !account.Enabled {
		return nil
	}
	return person
}

// activityJSON answers in the type a server asked for.
func activityJSON(c echo.Context, body any) error {
	c.Response().Header().Set(echo.HeaderContentType, ap.ContentType)
	c.Response().Header().Set("Vary", "Accept")
	return c.JSON(http.StatusOK, body)
}
