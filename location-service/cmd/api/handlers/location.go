package handlers

import (
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_responses "anahita-libs/responses"
	"fmt"
	"location/constants"
	"location/data/validators"
	"location/requests"
	"location/responses"
	"log"
	"net/http"
	"strings"

	"github.com/gosimple/slug"
	"github.com/labstack/echo/v4"
)

type locationHandler struct {
	*Env
}

// NewLocationHandler creates a new location handler
func NewLocationHandler(env *Env) *locationHandler {
	return &locationHandler{
		env,
	}
}

// Browse returns a list of location
func (h *locationHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewLocationRequest().Browse
		viewer = getViewer(c)
		nodes  = []*entities.AnahitaNode{}
		total  uint64
	)

	if ok, err := h.Perms.Locations.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	resp := &an_responses.List[*responses.LocationNodeItem]{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	if req.SourceID == 0 {
		gReq := services.NewGraphNodeRequest().Browse
		gReq.Types = []string{an_constants.NodeBaseLocationEntityType}
		gReq.NearbyLatitude = req.NearbyLatitude
		gReq.NearbyLongitude = req.NearbyLongitude
		gReq.NearbyRadius = req.NearbyRadius
		gReq.Search = req.Search
		gReq.Ordering = req.Ordering
		gReq.OrderingDir = req.Direction
		gReq.Limit = req.Limit
		gReq.Offset = req.Offset
		gReq.Ordering = req.Ordering
		gReq.Viewer = viewer
		gReq.AccessCheck = false
		resp.Pagination.Total, err = h.Srv.GraphNodeGRPC.Count(gReq)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		nodes, err = h.Srv.GraphNodeGRPC.Browse(gReq)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	} else {
		gReq := services.NewGraphRequest().BrowseConnectionNodes
		gReq.NodeId = req.SourceID
		gReq.EdgeType = an_constants.EdgeTagLocationType
		gReq.Direction = an_constants.GraphQueryDirectionInbound
		gReq.Limit = req.Limit
		gReq.Offset = req.Offset
		gReq.Ordering = req.Ordering
		gReq.Viewer = viewer
		gReq.AccessCheck = false
		nodes, total, err = h.Srv.GraphGRPC.BrowseConnectionNodes(gReq)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		resp.Pagination.Total = total
	}

	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	locationResp := &responses.LocationNodeResponse{}
	resp.Data = locationResp.MapLocationNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// BrowseOrderedByDegree returns a list of popular hashtags
func (h *locationHandler) BrowseOrderedByDegree(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = requests.NewLocationRequest().BrowseOrderedByDegree
		gReq   = services.NewGraphRequest().BrowseOrderedByDegree
	)

	if ok, err := h.Perms.Locations.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	resp := &an_responses.List[*responses.LocationNodeItem]{}

	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.NodeType = an_constants.NodeBaseLocationEntityType
	gReq.EdgeTypes = []string{an_constants.EdgeTagLocationType}
	gReq.Direction = an_constants.GraphQueryDirectionOutbound
	gReq.Limit = req.Limit
	gReq.Offset = req.Offset
	gReq.Viewer = viewer
	gReq.AccessCheck = false
	nodes, total, err := h.Srv.GraphGRPC.BrowseOrderedByDegree(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	locationResp := &responses.LocationNodeResponse{}
	resp.Data = locationResp.MapLocationNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// BrowseTrending returns a list of trending hashtags
func (h *locationHandler) BrowseTrending(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = requests.NewLocationRequest().BrowseTrending
		gReq   = services.NewGraphRequest().Trending
	)

	if ok, err := h.Perms.Locations.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	gReq.NodeType = an_constants.NodeBaseLocationEntityType
	gReq.EdgeTypes = []string{an_constants.EdgeTagLocationType}
	gReq.Direction = an_constants.GraphQueryDirectionOutbound
	gReq.Limit = req.Limit
	gReq.TimeWindow = strings.ToUpper(req.TimeWindow)
	gReq.Viewer = viewer
	gReq.AccessCheck = false
	nodes, err := h.Srv.GraphGRPC.Trending(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	locationResp := &responses.LocationNodeResponse{}
	resp := locationResp.MapLocationNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Read returns a location
func (h *locationHandler) Read(c echo.Context) error {
	var (
		err    error
		req    = requests.NewLocationRequest().Read
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	node, err := h.Srv.GraphNodeGRPC.Read(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Locations.CanRead(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	h.onAfterFetched(c, node)

	locationResp := &responses.LocationNodeResponse{}
	resp := locationResp.MapLocationNodeItem(node)

	return c.JSON(http.StatusOK, resp)
}

// Edit updates a location
func (h *locationHandler) Edit(c echo.Context) error {
	var (
		err    error
		req    = requests.NewLocationRequest().Edit
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	node, err := h.Srv.GraphNodeGRPC.Read(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Locations.CanEdit(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	addrChanged := addressChanged(req, node)

	node.Name = req.Name
	node.Alias = slug.Make(req.Name)
	node.GeoAddress = req.Address
	node.GeoCity = req.City
	node.GeoStateProvince = req.StateProvince
	node.GeoCountry = req.Country
	node.ModifiedBy = viewer.ID

	v := validators.NewLocationValidator()
	if err = v.ValidateEdit(node); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if addrChanged {
		addr := fmt.Sprintf("%s, %s, %s, %s", node.GeoAddress, node.GeoCity, node.GeoStateProvince, node.GeoCountry)
		location, err := h.Srv.Location.GeocodeSimple(addr)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusBadRequest)
		}
		node.GeoLatitude = location.Latitude
		node.GeoLongitude = location.Longitude
	}

	node, err = h.Srv.GraphNodeGRPC.Edit(node)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.onAfterFetched(c, node)

	locationResp := &responses.LocationNodeResponse{}
	resp := locationResp.MapLocationNodeItem(node)

	return c.JSON(http.StatusOK, resp)
}

// Add creates a new location
func (h *locationHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewLocationRequest().Add
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if ok, err := h.Perms.Locations.CanAdd(viewer, nil); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	node := entities.NewAnahitaNode()
	node.Type = an_constants.NodeBaseLocationEntityType
	node.Service = constants.LocationService
	node.Enabled = true
	node.Name = req.Name
	node.Alias = slug.Make(req.Name)
	node.GeoAddress = req.Address
	node.GeoCity = req.City
	node.GeoStateProvince = req.StateProvince
	node.GeoCountry = req.Country
	node.CreatedBy = viewer.ID
	node.ModifiedBy = viewer.ID

	v := validators.NewLocationValidator()
	if err = v.ValidateAdd(node); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	addr := fmt.Sprintf("%s, %s, %s, %s", node.GeoAddress, node.GeoCity, node.GeoStateProvince, node.GeoCountry)
	location, err := h.Srv.Location.GeocodeSimple(addr)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	node.GeoLatitude = location.Latitude
	node.GeoLongitude = location.Longitude

	node, err = h.Srv.GraphNodeGRPC.Add(node)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.onAfterFetched(c, node)

	locationResp := &responses.LocationNodeResponse{}
	resp := locationResp.MapLocationNodeItem(node)

	return c.JSON(http.StatusCreated, resp)
}

// Delete removes a location
func (h *locationHandler) Delete(c echo.Context) error {
	var (
		req    = requests.NewLocationRequest().Edit
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	location, err := h.Srv.GraphNodeGRPC.Read(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Locations.CanDelete(viewer, location); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	_, err = h.Srv.GraphNodeGRPC.Delete(location.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

// onAfterFetched is called after fetching a location
func (h *locationHandler) onAfterFetched(c echo.Context, location *entities.AnahitaNode) {
	viewer := getViewer(c)

	location.Authorized.Edit, _ = h.Perms.Locations.CanEdit(viewer, location)
	location.Authorized.Delete, _ = h.Perms.Locations.CanDelete(viewer, location)
	location.Authorized.Administration, _ = h.Perms.Locations.CanAdministrate(viewer, location)
}
