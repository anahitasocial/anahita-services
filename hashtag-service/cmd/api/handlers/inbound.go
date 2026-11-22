package handlers

import (
	libs_constants "anahita-libs/constants"
	an_handlers "anahita-libs/handlers"

	"github.com/labstack/echo/v4"
)

type inboundHandler struct {
	*Env
}

// NewInboundHandler creates a new inbound handler
func NewInboundHandler(env *Env) *inboundHandler {
	return &inboundHandler{
		env,
	}
}

// Browse answers a page of the posts it is on, as every kind of tag's is
// (anahita-libs/handlers, TaggedWith).
func (h *inboundHandler) Browse(c echo.Context) error {
	return an_handlers.TaggedWith(c, an_handlers.TagReaders{
		Perms: h.Perms.Taggables,
		Nodes: h.Srv.GraphNodeGRPC,
		Graph: h.Srv.GraphGRPC,
		Files: h.Srv.FilesGRPC,
		Media: h.Srv.MediaGRPC,
	}, libs_constants.EdgeTagHashtagType)
}
