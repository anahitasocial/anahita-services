package handlers

import (
	"anahita-libs/responses"
	"net/http"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/remote"

	libhandlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/constants"
	"anahita-libs/crypt"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	an_mw "anahita-libs/middleware"
	"anahita-libs/policy"
	"anahita-libs/ratelimit"
	"anahita-libs/site"

	"github.com/labstack/echo/v4"
)

// Env is what the handlers work with.
type Env struct {
	Site     site.Addresses
	Nodes    services.GraphNodeGRPCInterface
	Accounts repositories.AccountsInterface
	Keys     repositories.KeysInterface
	Actors   repositories.ActorsInterface
	Follows  repositories.FollowsInterface
	Objects  repositories.ObjectsInterface
	// Where pictures of people elsewhere are fetched from and kept. Nil for
	// neither, in tests: then nobody's picture is copied.
	Pictures Pictures
	Files    services.FilesGRPCInterface
	// Runs what need not hold up an answer: copying a picture, reading
	// somebody's recent posts. Nil runs it at once, which tests want.
	Later func(func())
	// Where a post's list of pictures is kept. Nil for none, in tests that
	// are not of pictures.
	PhotoFiles PhotoFiles
	// The locations a post was tagged with, which go out with it. Nil for
	// none, in tests that are not of locations.
	Locations Locations
	// The card for a link in a post from elsewhere. Nil for none: in tests,
	// and where there is no link-preview-service.
	Cards Cards
	// Finds what the bus lost. Nil for no sweep, in tests that are not of
	// it.
	Missed repositories.MissedInterface
	// Says on the bus that a post from another server was kept, for the
	// hashtag service to give it its hashtags. Nil says nothing, in tests.
	Announce func(payload *event.EventPayload, name string) error
	// In the federation lab: nothing is sent to a server that is not the
	// lab's own. See Deliver.
	Lab bool
	// The servers that are refused. Nil for none, in tests.
	Blocks repositories.BlocksInterface
	// Who likes what, in the graph, as like-service keeps it.
	Likes services.LikesGRPCInterface
	// Reposts, in the graph, as feed-service keeps them.
	Reposts services.RepostGRPCInterface
	// The graph's own record of who follows whom, and who has blocked whom.
	FollowEdges services.SocialgraphFollowGRPCInterface
	Relations   services.SocialgraphGRPCInterface
	// What is waiting to be sent to other servers, and what sends it.
	Queue  Enqueuer
	Poster Poster
	Crypt  crypt.Crypt
	// What reads from other servers. Signed as the installation.
	Fetcher remote.Fetcher
	// Counts lookups, so that nobody can have this service call the whole
	// internet. Nil for no limit, in tests.
	Limiter ratelimit.Counter
}

type Handlers struct {
	*Env
	addresses ap.Addresses
	signing   signing
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{Env: env, addresses: ap.Addresses{Site: env.Site}}
}

func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}

func refuse(c echo.Context, status int, code string) error {
	return c.JSON(status, responses.Refusal{Error: code})
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
// fediverse: a person or a website, made here, whose profile anybody may see.
//
// Whether anybody may see it is asked of the graph, as a visitor: it knows
// whether they are disabled, archived or on their way to being deleted. And
// the profile must say of itself that it is public, not merely open to
// whoever has an account.
func (h *Handlers) mayFederate(person *entities.AnahitaNode) bool {
	if person == nil || person.ID == 0 || person.UUID == "" || person.IsRemote() {
		return false
	}
	if !isFederatingType(person.Type) || !person.Enabled || person.DeletedAt != nil {
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
	// A person chooses it for themselves, and a website's administrators
	// for it.
	account, err := h.Accounts.Read(person.ID)
	if err != nil || account == nil || !account.Enabled {
		return nil
	}
	return person
}

// isFederatingType is the kinds of profile here that may be on the
// fediverse: a person, and a website, when they choose to be.
func isFederatingType(nodeType string) bool {
	return nodeType == constants.NodeActorPersonEntityType || nodeType == constants.NodeActorWebsiteEntityType
}

func isWebsite(node *entities.AnahitaNode) bool {
	return node != nil && node.Type == constants.NodeActorWebsiteEntityType
}

// activityJSON answers in the type a server asked for.
func activityJSON(c echo.Context, body any) error {
	c.Response().Header().Set(echo.HeaderContentType, ap.ContentType)
	c.Response().Header().Set("Vary", "Accept")
	return c.JSON(http.StatusOK, body)
}
