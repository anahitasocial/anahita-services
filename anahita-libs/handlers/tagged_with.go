package handlers

import (
	"log"
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	"anahita-libs/permissions"
	"anahita-libs/requests"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// TagReaders is what TaggedWith reads with: the graph, the files to sign,
// and who may browse tags.
type TagReaders struct {
	Perms permissions.TaggablePermissionsInterface
	Nodes services.GraphNodeGRPCInterface
	Graph services.GraphGRPCInterface
	Files services.FilesGRPCInterface
	Media services.MediaGRPCInterface
}

// TaggedWith answers a page of the posts a hashtag or a location is on, by
// edgeType, those the viewer may see: hashtag-service's and
// location-service's, which each kept a copy of this (roadmap row 170).
//
// GET /{hashtags,locations}/:id/inbounds
func TaggedWith(c echo.Context, r TagReaders, edgeType string) error {
	req, viewer := requests.NewInboundRequest().Browse, an_mw.GetViewer(c)
	if ok, err := r.Perms.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}
	if !Bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}

	source, err := r.Nodes.Read(req.SourceID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	resp := &responses.List[*responses.NodeItem]{}
	req.Limit = requests.PageSize(req.Limit)
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	graphReq := services.NewGraphRequest().BrowseConnectionNodes
	graphReq.NodeId = source.ID
	graphReq.EdgeType = edgeType
	graphReq.Direction = constants.GraphQueryDirectionOutbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	graphReq.Ordering = req.Ordering
	graphReq.OrderingDir = req.OrderingDir
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true
	nodes, total, err := r.Graph.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total
	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	services.BatchPresignNodes(r.Files, nodes)
	// A photo post with several images is drawn with all of them, as in a
	// feed, and each is signed: the cover alone is not enough.
	services.AttachPhotoFiles(r.Media, nodes)
	services.BatchPresignPhotoFiles(r.Files, nodes)

	nodeResp := &responses.NodeResponse{}
	resp.Data = nodeResp.MapNodeItems(nodes)
	return c.JSON(http.StatusOK, resp)
}
