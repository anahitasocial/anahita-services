package handlers

import (
	"net/http"
	"regexp"

	libhandlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/policy"

	"github.com/labstack/echo/v4"
)

// An address for a picture that does not change.
//
// The address a browser is given for an image is signed and lasts a
// quarter of an hour, which suits a page that is read and closed. It does
// not suit another server: one that is told of a person here keeps the
// address of their avatar and shows it for weeks. So a public profile's
// pictures also have an address that stays the same, made of the node's
// UUID, which answers by sending whoever asks on to a freshly signed one.
//
// ANYBODY CAN ASK, SO ONLY WHAT ANYBODY MAY SEE IS ANSWERED FOR. The node
// is asked about as somebody who is not signed in, and has to say of itself
// that it is public. Everything else is not found, in the same words as a
// node that does not exist: the address says nothing about what is there.

// How long whoever was sent on may remember where to. Well inside the life
// of the signed address they were sent to.
const publicImageCache = "public, max-age=300"

var uuidShape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// PublicAvatar sends on to the avatar of a public actor.
func (h *imageHandler) PublicAvatar(c echo.Context) error { return h.public(c, avatar) }

// PublicCover sends on to the cover of a public actor or post.
func (h *imageHandler) PublicCover(c echo.Context) error { return h.public(c, cover) }

func (h *imageHandler) public(c echo.Context, k kind) error {
	// A site for members shows nothing to anybody else.
	if policy.MembersOnly() {
		return c.NoContent(http.StatusNotFound)
	}

	uuid := c.Param("uuid")
	if !uuidShape.MatchString(uuid) {
		return c.NoContent(http.StatusNotFound)
	}

	node, err := h.Srv.GraphNodeGRPC.ReadByUUID(uuid)
	if err != nil || node == nil || node.ID == 0 {
		return c.NoContent(http.StatusNotFound)
	}

	// Made here, of a sort that has such a picture, and has one.
	if node.IsRemote() || !k.allowed(node) || k.filename(node) == "" {
		return c.NoContent(http.StatusNotFound)
	}

	// The read above checks nothing. Whether somebody who is not signed in
	// may see the node is asked of the graph, which knows about its
	// profile, its state and everything else; and the node must be open to
	// everybody in its own right, not merely to whoever has an account.
	if node.Access != constants.AnahitaAccessPublic || !libhandlers.MaySee(h.Srv.GraphNodeGRPC, node.ID, entities.NewViewer()) {
		return c.NoContent(http.StatusNotFound)
	}

	sizes, err := k.sizes(node)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	image, ok := sizes[c.Param("size")]
	if !ok || image.Url == "" {
		return c.NoContent(http.StatusNotFound)
	}

	request := services.NewFileRequest().Presign
	request.Path = image.Url
	address, err := h.Srv.FilesGRPC.Presign(request)
	if err != nil || address == "" {
		return c.NoContent(http.StatusNotFound)
	}

	c.Response().Header().Set("Cache-Control", publicImageCache)
	return c.Redirect(http.StatusFound, address)
}
