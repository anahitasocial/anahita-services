package handlers

import (
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"socialgraph/requests"
	"time"

	an_entities "anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/federation"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// The servers a person blocked for themselves (roadmap row 152): theirs
// alone, made, listed and lifted by them. Nothing from such a server, or
// one under it, is shown to them, and no follow crosses; blocking one ends
// the follows there through the ordinary removal of a follow, so that the
// other server is told as for any follow that ends (an Undo of theirs, a
// Reject of the other's). Nobody there is told of the block itself.
type serverBlockHandler struct {
	*Env
}

func NewServerBlockHandler(env *Env) *serverBlockHandler {
	return &serverBlockHandler{env}
}

// ServerBlock is a blocked server as it is listed.
type ServerBlock struct {
	// The server's domain, lowercase.
	Domain string `json:"domain"`
	// When it was blocked.
	CreatedAt time.Time `json:"created_at"`
}

// A domain: labels of letters, digits and hyphens, at least two of them.
var serverShape = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)

// Bluesky's posts are all kept as from bsky.app: one server cannot be told
// from another there, and a person blocks accounts on Bluesky one by one.
const blueskyServer = "bsky.app"

// serverIn is the server a domain, a handle (`naomi@tycho.example`) or an
// address on it (`https://tycho.example/@naomi`) names: an address's host,
// whose path may hold an @, and otherwise what CleanDomain makes of it.
func serverIn(said string) string {
	said = strings.TrimSpace(said)
	if strings.Contains(said, "://") {
		if parsed, err := url.Parse(said); err == nil && parsed.Host != "" {
			said = parsed.Host
		}
	}
	return federation.CleanDomain(said)
}

// ownViewer is the person asking, when the address names them.
func ownViewer(c echo.Context, actorID int64) (*an_entities.Viewer, bool) {
	viewer := getViewer(c)
	return viewer, viewer != nil && viewer.ID > 0 && viewer.ID == actorID
}

func (h *serverBlockHandler) Browse(c echo.Context) error {
	req := &requests.ServerBlockBrowseRequest{}
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if _, ok := ownViewer(c, req.ActorID); !ok {
		return c.NoContent(http.StatusForbidden)
	}
	limit := req.Limit
	if limit == 0 {
		limit = 20
	}
	blocks, total, err := h.Srv.SocialgraphServerBlockGRPC.Browse(req.ActorID, req.Offset, limit)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	listed := make([]*ServerBlock, 0, len(blocks))
	for _, b := range blocks {
		listed = append(listed, &ServerBlock{Domain: b.Domain, CreatedAt: b.CreatedAt})
	}
	return c.JSON(http.StatusOK, &responses.List[*ServerBlock]{
		Data:       listed,
		Pagination: responses.Pagination{Limit: uint64(limit), Offset: uint64(req.Offset), Total: uint64(total)},
	})
}

func (h *serverBlockHandler) Add(c echo.Context) error {
	req := &requests.ServerBlockAddRequest{}
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	viewer, ok := ownViewer(c, req.ActorID)
	if !ok {
		return c.NoContent(http.StatusForbidden)
	}
	domain := serverIn(req.Domain)
	if !serverShape.MatchString(domain) {
		return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "server_invalid"})
	}
	if domain == blueskyServer {
		return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "server_on_bluesky"})
	}

	if err := h.Srv.SocialgraphServerBlockGRPC.Add(viewer.ID, domain); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	h.endFollows(viewer, domain)

	return c.JSON(http.StatusCreated, &ServerBlock{Domain: domain, CreatedAt: time.Now().UTC()})
}

// endFollows ends every follow, either way, between the person and anybody
// on the server or under it. One that fails is logged and the rest go on:
// the block holds already, and refuses what was missed (IsServerBlocked).
func (h *serverBlockHandler) endFollows(viewer *an_entities.Viewer, domain string) {
	follows, err := h.Srv.SocialgraphServerBlockGRPC.Follows(viewer.ID, domain)
	if err != nil {
		log.Printf("server block %s for %d: the follows were not read: %v", domain, viewer.ID, err)
		return
	}
	for _, follow := range follows {
		source, err := h.Srv.ActorsGRPC.Read(follow.SourceID, viewer)
		if err != nil {
			log.Println(err)
			continue
		}
		target, err := h.Srv.ActorsGRPC.Read(follow.TargetID, viewer)
		if err != nil {
			log.Println(err)
			continue
		}
		remove := services.NewSocialgraphFollowRequest().Delete
		remove.Source, remove.Target, remove.Viewer = source, target, viewer
		if err := h.Srv.SocialgraphFollowGRPC.Delete(remove); err != nil {
			log.Printf("server block %s for %d: the follow %d>%d stays: %v", domain, viewer.ID, follow.SourceID, follow.TargetID, err)
		}
	}
}

func (h *serverBlockHandler) Delete(c echo.Context) error {
	req := &requests.ServerBlockDeleteRequest{}
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	viewer, ok := ownViewer(c, req.ActorID)
	if !ok {
		return c.NoContent(http.StatusForbidden)
	}
	if err := h.Srv.SocialgraphServerBlockGRPC.Delete(viewer.ID, federation.CleanDomain(req.Domain)); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusNoContent)
}
