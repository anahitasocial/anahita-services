package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/feeds"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/permissions"
	"anahita-libs/policy"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// Custom feeds: a feed somebody builds, saves under a name of their own and
// keeps among the tabs of their home page.
//
// A feed is a node owned by whoever made it, with what it shows (its
// definition) in the node's meta. It has an audience like a post: "only me"
// unless its owner says otherwise. Whoever may see a feed may read it, copy
// it and keep it on their home page, and what they read in it is what they
// could have seen anyway: the graph reads every page for the reader.

type customFeedHandler struct {
	*Env
}

func NewCustomFeedHandler(env *Env) *customFeedHandler {
	return &customFeedHandler{Env: env}
}

// Who a feed may be shown to. Not "people I follow" or "mutuals": a feed is
// shared widely or kept.
var customFeedAccess = []string{
	constants.AnahitaAccessMyself,
	constants.AnahitaAccessFollowers,
	constants.AnahitaAccessRegistered,
	constants.AnahitaAccessPublic,
}

// CustomFeedFields is what makes a custom feed.
type CustomFeedFields struct {
	// What it is called.
	Name string `json:"name" validate:"required,max=100"`
	// What it is for, in its owner's words.
	Body string `json:"body" validate:"omitempty,max=500"`
	// Who may see it: `myself`, `followers`, `registered` or `public`.
	Access string `json:"access" validate:"omitempty,oneof=myself followers registered public"`
	// Its picture, by name: one of feeds.Icons. The default when left out.
	Icon string `json:"icon" validate:"omitempty,max=20"`
	// What it shows, as anahita-libs/feeds reads it.
	Definition json.RawMessage `json:"definition"`
}

type CustomFeedEditRequest struct {
	// The feed's id.
	ID int64 `param:"id" validate:"required,numeric"`
	CustomFeedFields
}

type CustomFeedIDRequest struct {
	// The feed's id.
	ID int64 `param:"id" validate:"required,numeric"`
}

type CustomFeedCopyRequest struct {
	// The id of the feed copied.
	ID int64 `param:"id" validate:"required,numeric"`
	// What to call the copy, in the copier's own language. "Copy of" and
	// the feed's name when left out.
	Name string `json:"name" validate:"omitempty,max=100"`
}

type CustomFeedItemsRequest struct {
	// The feed's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Where the page begins: the `next_cursor` of the page before. The
	// latest when left out.
	Cursor string `query:"cursor" validate:"omitempty,max=100"`
	// The most to send: up to 40.
	Limit uint64 `query:"limit" validate:"omitempty,numeric,max=40"`
	// Where the reader is, for a feed of what is near them: its area is
	// around this point for this page, and the point is kept nowhere.
	// Rounded to three decimals. Needed by such a feed, unread by any
	// other.
	NearLatitude  *float64 `query:"near_latitude"`
	NearLongitude *float64 `query:"near_longitude"`
}

type CustomFeedDiscoverRequest struct {
	// Part of a name, to find a feed by.
	Search string `query:"q" validate:"omitempty,max=100"`
	// How many to pass over.
	Start uint64 `query:"start" validate:"omitempty,numeric"`
	// The most to send: up to 40.
	Limit uint64 `query:"limit" validate:"omitempty,numeric,max=40"`
}

type CustomFeedPreviewRequest struct {
	// What it would show, as feeds.md describes it.
	Definition json.RawMessage `json:"definition"`
}

type CustomFeedPinsRequest struct {
	// The feeds kept on the viewer's home page, in order: up to 10.
	IDs []int64 `json:"ids" validate:"max=10,dive,numeric,min=1"`
}

// CustomFeedResponse is a feed as it is sent.
type CustomFeedResponse struct {
	// Its id.
	ID int64 `json:"id"`
	// Always a custom feed's type.
	Type string `json:"type"`
	// What it is called.
	Name string `json:"name"`
	// What it is for, in its owner's words.
	Body string `json:"body"`
	// Who may see it.
	Access string `json:"access"`
	// The name of its picture.
	Icon string `json:"icon"`
	// What it shows (feeds.md).
	Definition json.RawMessage `json:"definition"`
	// Whose it is.
	Owner *responses.NodeItem `json:"owner,omitempty"`
	// Whether the viewer keeps it on their home page.
	IsPinned bool `json:"is_pinned"`
	// The profiles and places its definition names, the ones the viewer may
	// see, for the editor to show by name. Sent with a single feed.
	References []*responses.NodeItem `json:"references,omitempty"`
	// What the viewer may do with it.
	Authorized CustomFeedAuthorized `json:"authorized"`
	// When it was made.
	CreatedAt time.Time `json:"created_at"`
	// When it last changed.
	UpdatedAt time.Time `json:"updated_at"`
}

type CustomFeedAuthorized struct {
	// Whether the viewer may change it.
	Edit bool `json:"edit"`
	// Whether the viewer may delete it.
	Delete bool `json:"delete"`
	// Keep it on the home page, and make a copy to change.
	Pin  bool `json:"pin"`
	Copy bool `json:"copy"`
}

// CustomFeedPage is a page of a custom feed's posts.
type CustomFeedPage struct {
	// Its posts, the latest first.
	Data []*responses.NodeItem `json:"data"`
	// Where the page is.
	Pagination CustomFeedPagination `json:"pagination"`
}

// A feed is paged by what came before, and has no total: counting it would
// be reading all of it.
type CustomFeedPagination struct {
	// The most a page holds.
	Limit uint64 `json:"limit"`
	// What to ask for as `cursor` for the next page.
	NextCursor string `json:"next_cursor"`
	// Whether there is a next page.
	HasMore bool `json:"has_more"`
}

// DiscoverPage is a page of the feeds people have shared.
type DiscoverPage struct {
	// The feeds.
	Data []*CustomFeedResponse `json:"data"`
	// Where the page is.
	Pagination DiscoverPagination `json:"pagination"`
}

// DiscoverPagination is where a page of shared feeds is: by how many came
// before it, and with no total.
type DiscoverPagination struct {
	// The most a page holds.
	Limit uint64 `json:"limit"`
	// How many came before this page.
	Start uint64 `json:"start"`
	// What to ask for as `start` for the next page.
	NextStart uint64 `json:"next_start"`
	// Whether there is a next page.
	HasMore bool `json:"has_more"`
}

func refuseFeed(c echo.Context, status int, reason string) error {
	return c.JSON(status, responses.Refusal{Error: reason})
}

// mayWrite reports whether the viewer may make and change feeds of their own.
func mayWriteFeeds(viewer *entities.Viewer) bool {
	return viewer != nil && viewer.Authenticated && viewer.ID > 0 &&
		viewer.HasScope(constants.OAuthScopeMediaWrite)
}

func ownsFeed(viewer *entities.Viewer, feed *entities.AnahitaNode) bool {
	return viewer != nil && viewer.ID > 0 && feed.OwnerID == viewer.ID
}

// read finds a feed the viewer may see. Nil for one that is not there and
// for one that is not theirs to see, which are not told apart.
func (h *customFeedHandler) read(id int64, viewer *entities.Viewer) *entities.AnahitaNode {
	feed, err := h.Srv.GraphNodeGRPC.Read(id)
	if err != nil || feed == nil || feed.ID == 0 || feed.Type != feeds.NodeType {
		return nil
	}

	if ownsFeed(viewer, feed) || viewer.IsAdmin() {
		return feed
	}

	if !permissions.CanReadByID(h.Srv.GraphNodeGRPC, feed.ID, viewer) {
		return nil
	}

	return feed
}

// storedOf reads what a feed's node keeps: what it shows, and its picture.
func storedOf(feed *entities.AnahitaNode) (*feeds.Stored, error) {
	return feeds.ReadStored(feed.Meta)
}

// definitionOf reads what a stored feed shows. A feed whose definition can
// no longer be read shows nothing, and says so to whoever tries to read it.
func definitionOf(feed *entities.AnahitaNode) (*feeds.Definition, error) {
	stored, err := storedOf(feed)
	if err != nil {
		return nil, err
	}

	return stored.Definition, nil
}

func (h *customFeedHandler) respond(feed *entities.AnahitaNode, viewer *entities.Viewer, pinned map[int64]bool) *CustomFeedResponse {
	own := ownsFeed(viewer, feed) || viewer.IsAdmin()
	signedIn := viewer.Authenticated && viewer.ID > 0

	definition := json.RawMessage("null")
	icon := feeds.DefaultIcon
	if stored, err := storedOf(feed); err == nil {
		definition = stored.Definition.JSON()
		icon = stored.Icon
	}

	nodeResp := &responses.NodeResponse{}

	return &CustomFeedResponse{
		ID:         feed.ID,
		Type:       feed.Type,
		Name:       feed.Name,
		Body:       feed.Body,
		Access:     feed.Access,
		Icon:       icon,
		Definition: definition,
		Owner:      nodeResp.MapNodeItem(feed.Owner),
		IsPinned:   pinned[feed.ID],
		Authorized: CustomFeedAuthorized{
			Edit:   own,
			Delete: own,
			Pin:    signedIn,
			Copy:   mayWriteFeeds(viewer),
		},
		CreatedAt: feed.CreatedAt,
		UpdatedAt: feed.UpdatedAt,
	}
}

func (h *customFeedHandler) respondMany(list []*entities.AnahitaNode, viewer *entities.Viewer, pinned map[int64]bool) []*CustomFeedResponse {
	owners := make([]entities.AnahitaNodeAvatarInterface, 0, len(list))
	for _, feed := range list {
		if feed.Owner != nil {
			owners = append(owners, feed.Owner)
		}
	}
	if len(owners) > 0 {
		services.BatchPresignAvatar(h.Srv.FilesGRPC, owners)
	}

	out := make([]*CustomFeedResponse, 0, len(list))
	for _, feed := range list {
		out = append(out, h.respond(feed, viewer, pinned))
	}

	return out
}

// pinEdges reads the feeds a person keeps on their home page, in order.
func (h *customFeedHandler) pinEdges(personID int64) ([]*entities.AnahitaEdge, error) {
	if personID <= 0 {
		return nil, nil
	}

	req := services.NewGraphEdgeRequest().BatchReadByFilters
	req.NodeAID = personID
	req.Types = []string{feeds.EdgePinType}
	req.Ordering = "ordering"
	req.OrderingDir = constants.DirAsc

	edges, err := h.Srv.GraphEdgeGRPC.BatchReadByFilters(req)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(edges, func(i, j int) bool { return edges[i].Ordering < edges[j].Ordering })

	return edges, nil
}

func (h *customFeedHandler) pinnedIDs(personID int64) ([]int64, map[int64]bool, error) {
	edges, err := h.pinEdges(personID)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]int64, 0, len(edges))
	pinned := make(map[int64]bool, len(edges))
	for _, edge := range edges {
		if !pinned[edge.NodeBID] {
			pinned[edge.NodeBID] = true
			ids = append(ids, edge.NodeBID)
		}
	}

	return ids, pinned, nil
}

// writePins replaces a person's pinned feeds with these, in this order.
func (h *customFeedHandler) writePins(viewer *entities.Viewer, ids []int64) error {
	clear := services.NewGraphEdgeRequest().BatchDeleteByFilters
	clear.NodeAID = viewer.ID
	clear.Types = []string{feeds.EdgePinType}
	// Narrowed three ways. Left to the person alone, this would be every
	// edge they have: whoever they follow, what they like.
	clear.NodeAType = constants.NodeActorPersonEntityType
	clear.NodeBType = feeds.NodeType
	if err := h.Srv.GraphEdgeGRPC.BatchDeleteByFilters(clear); err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	edges := make([]*entities.AnahitaEdge, 0, len(ids))
	for place, id := range ids {
		edges = append(edges, &entities.AnahitaEdge{
			Type:      feeds.EdgePinType,
			NodeAID:   viewer.ID,
			NodeAType: constants.NodeActorPersonEntityType,
			NodeBID:   id,
			NodeBType: feeds.NodeType,
			Ordering:  int32(place + 1),
			CreatedBy: viewer.ID,
		})
	}

	_, err := h.Srv.GraphEdgeGRPC.BatchAdd(edges)

	return err
}

// ownedCount is how many feeds a person has made.
func (h *customFeedHandler) ownedCount(personID int64) (uint64, error) {
	req := services.NewGraphNodeRequest().Browse
	req.Types = []string{feeds.NodeType}
	req.OwnerIDs = []int64{personID}

	return h.Srv.GraphNodeGRPC.Count(req)
}

// checkFields reads the definition out of what was sent and settles the
// audience. The reason is empty when all is well.
func checkFields(fields *CustomFeedFields, current string) (*feeds.Definition, string) {
	fields.Name = strings.TrimSpace(fields.Name)
	fields.Body = strings.TrimSpace(fields.Body)
	if fields.Name == "" {
		return nil, "name_required"
	}

	def, err := feeds.Parse(fields.Definition)
	if err != nil {
		if refusal, ok := feeds.AsRefusal(err); ok {
			return nil, string(refusal)
		}
		return nil, string(feeds.RefusedMalformed)
	}
	// Of an area that is wherever its reader is, the radius is kept, and
	// never where its maker was when they made it.
	def.ForgetPoint()

	if fields.Access == "" {
		fields.Access = current
	}
	if fields.Access == "" {
		fields.Access = constants.AnahitaAccessMyself
	}
	// An installation that takes no new public content takes no new public
	// feed. One that is public already may stay so.
	if !policy.MayChangeAccess(current, fields.Access) {
		return nil, "access_not_allowed"
	}

	return def, ""
}

// Add makes a feed, the viewer's own.
func (h *customFeedHandler) Add(c echo.Context) error {
	viewer := getViewer(c)
	if !mayWriteFeeds(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	fields := &CustomFeedFields{}
	if err := c.Bind(fields); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(fields); err != nil {
		return refuseFeed(c, http.StatusUnprocessableEntity, "invalid")
	}

	def, reason := checkFields(fields, "")
	if reason != "" {
		return refuseFeed(c, http.StatusUnprocessableEntity, reason)
	}

	feed, status, reason := h.create(viewer, fields.Name, fields.Body, fields.Access, feeds.NewStored(def, fields.Icon))
	if feed == nil {
		return refuseFeed(c, status, reason)
	}

	pinned := map[int64]bool{feed.ID: h.keepOnHome(viewer, feed.ID)}

	return c.JSON(http.StatusCreated, h.respondMany([]*entities.AnahitaNode{feed}, viewer, pinned)[0])
}

// keepOnHome puts a feed its owner has just made on their home page, after
// the ones there, when there is room. A feed is made to be read, and one
// that had to be looked for and added by hand read as if making it had not
// worked. With the home page full the feed is made all the same, and is on
// the list of the viewer's feeds to be added when they choose.
func (h *customFeedHandler) keepOnHome(viewer *entities.Viewer, id int64) bool {
	current, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		log.Println("custom feed: keep on home:", err)
		return false
	}
	if pinned[id] {
		return true
	}
	if len(current) >= feeds.MaxPinned {
		return false
	}

	if err := h.writePins(viewer, append(current, id)); err != nil {
		log.Println("custom feed: keep on home:", err)
		return false
	}

	return true
}

// create writes a new feed owned by the viewer, when they have room for one.
func (h *customFeedHandler) create(viewer *entities.Viewer, name, body, access string, stored *feeds.Stored) (*entities.AnahitaNode, int, string) {
	owned, err := h.ownedCount(viewer.ID)
	if err != nil {
		log.Println(err)
		return nil, http.StatusInternalServerError, "generic"
	}
	if owned >= feeds.MaxOwned {
		return nil, http.StatusConflict, "too_many_feeds"
	}

	node := entities.NewAnahitaNode()
	node.Type = feeds.NodeType
	node.Service = feeds.Service
	node.Name = name
	node.Body = body
	node.Access = access
	node.Meta = stored.JSON()
	node.Enabled = true
	node.OwnerID = viewer.ID
	node.OwnerType = constants.NodeActorPersonEntityType
	node.CreatedBy = viewer.ID
	node.ModifiedBy = viewer.ID

	added, err := h.Srv.GraphNodeGRPC.Add(node)
	if err != nil || added == nil || added.ID == 0 {
		log.Println("custom feed: add:", err)
		return nil, http.StatusInternalServerError, "generic"
	}

	// Read back, so that it is sent with its owner as every other is.
	if read, err := h.Srv.GraphNodeGRPC.Read(added.ID); err == nil && read != nil && read.ID > 0 {
		return read, http.StatusCreated, ""
	}

	return added, http.StatusCreated, ""
}

// Read returns one feed, with the profiles and places it names.
func (h *customFeedHandler) Read(c echo.Context) error {
	viewer := getViewer(c)

	req := &CustomFeedIDRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	feed := h.read(req.ID, viewer)
	if feed == nil {
		return c.NoContent(http.StatusNotFound)
	}

	_, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		log.Println(err)
	}

	resp := h.respondMany([]*entities.AnahitaNode{feed}, viewer, pinned)[0]
	resp.References = h.references(feed, viewer)

	return c.JSON(http.StatusOK, resp)
}

// references reads the profiles and places a feed's definition names, the
// ones the viewer may see.
func (h *customFeedHandler) references(feed *entities.AnahitaNode, viewer *entities.Viewer) []*responses.NodeItem {
	def, err := definitionOf(feed)
	if err != nil {
		return nil
	}

	ids := append([]int64{}, def.Sources.Actors...)
	ids = append(ids, def.Exclude.Actors...)
	ids = append(ids, def.Filters.Locations...)
	if len(ids) == 0 {
		return nil
	}

	visible, err := h.Srv.GraphNodeGRPC.FilterVisible(ids, viewer)
	if err != nil || len(visible) == 0 {
		return nil
	}

	req := services.NewGraphNodeRequest().BatchRead
	req.IDs = visible
	req.Viewer = viewer
	nodes, err := h.Srv.GraphNodeGRPC.BatchRead(req)
	if err != nil {
		log.Println(err)
		return nil
	}

	avatars := make([]entities.AnahitaNodeAvatarInterface, 0, len(nodes))
	for _, node := range nodes {
		avatars = append(avatars, node)
	}
	if len(avatars) > 0 {
		services.BatchPresignAvatar(h.Srv.FilesGRPC, avatars)
	}

	nodeResp := &responses.NodeResponse{}

	return nodeResp.MapNodeItems(nodes)
}

// Edit changes a feed: its name, what it is for, who may see it and what it
// shows. Its owner's to do.
func (h *customFeedHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	if !mayWriteFeeds(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &CustomFeedEditRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return refuseFeed(c, http.StatusUnprocessableEntity, "invalid")
	}

	feed := h.read(req.ID, viewer)
	if feed == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if !ownsFeed(viewer, feed) && !viewer.IsAdmin() {
		return c.NoContent(http.StatusForbidden)
	}

	def, reason := checkFields(&req.CustomFeedFields, feed.Access)
	if reason != "" {
		return refuseFeed(c, http.StatusUnprocessableEntity, reason)
	}

	feed.Name = req.Name
	feed.Body = req.Body
	feed.Access = req.Access
	feed.Meta = feeds.NewStored(def, req.Icon).JSON()
	feed.ModifiedBy = viewer.ID

	if _, err := h.Srv.GraphNodeGRPC.Edit(feed); err != nil {
		log.Println("custom feed: edit:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	_, pinned, _ := h.pinnedIDs(viewer.ID)

	return c.JSON(http.StatusOK, h.respondMany([]*entities.AnahitaNode{feed}, viewer, pinned)[0])
}

// Delete removes a feed, and with it everybody's keeping of it.
func (h *customFeedHandler) Delete(c echo.Context) error {
	viewer := getViewer(c)
	if !mayWriteFeeds(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &CustomFeedIDRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	feed := h.read(req.ID, viewer)
	if feed == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if !ownsFeed(viewer, feed) && !viewer.IsAdmin() {
		return c.NoContent(http.StatusForbidden)
	}

	if _, err := h.Srv.GraphNodeGRPC.Delete(feed.ID); err != nil {
		log.Println("custom feed: delete:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}

// Copy makes a feed of the viewer's own from one they may see: the same
// thing shown, kept to themselves, to change as they like. It is called
// "Copy of" the other, or whatever name is sent.
func (h *customFeedHandler) Copy(c echo.Context) error {
	viewer := getViewer(c)
	if !mayWriteFeeds(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &CustomFeedCopyRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	source := h.read(req.ID, viewer)
	if source == nil {
		return c.NoContent(http.StatusNotFound)
	}

	stored, err := storedOf(source)
	if err != nil {
		return refuseFeed(c, http.StatusUnprocessableEntity, string(feeds.RefusedMalformed))
	}

	// Named apart from the one it was copied from, so that the two can be
	// told from each other in a list until the copy is given a name.
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Copy of " + source.Name
	}
	if runes := []rune(name); len(runes) > 100 {
		name = string(runes[:100])
	}

	feed, status, reason := h.create(viewer, name, source.Body, constants.AnahitaAccessMyself, stored)
	if feed == nil {
		return refuseFeed(c, status, reason)
	}

	pinned := map[int64]bool{feed.ID: h.keepOnHome(viewer, feed.ID)}

	return c.JSON(http.StatusCreated, h.respondMany([]*entities.AnahitaNode{feed}, viewer, pinned)[0])
}

// Mine lists the feeds the viewer has made, newest first.
func (h *customFeedHandler) Mine(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := services.NewGraphNodeRequest().Browse
	req.Types = []string{feeds.NodeType}
	req.OwnerIDs = []int64{viewer.ID}
	req.Limit = feeds.MaxOwned
	req.Viewer = viewer

	list, err := h.Srv.GraphNodeGRPC.Browse(req)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	_, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		log.Println(err)
	}

	return c.JSON(http.StatusOK, responses.Unpaged[*CustomFeedResponse]{Data: h.respondMany(list, viewer, pinned)})
}

// Discover lists the feeds people have shared, the ones the viewer may see,
// newest first: public ones, ones for anyone signed in, and ones for the
// followers of somebody the viewer follows. A feed kept to its owner is in
// nobody's list here, its owner's included: theirs are under their own.
//
// The graph's list of nodes gates each feed by its own audience and its
// owner's, for the viewer, as it does a post. The viewer's own private
// feeds pass that gate, being theirs, and are taken out here. So a page can
// come back a little short, and where the next one starts is said by
// `next_start`, counted in what the graph returned and not in what was
// sent.
func (h *customFeedHandler) Discover(c echo.Context) error {
	viewer := getViewer(c)

	req := &CustomFeedDiscoverRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if req.Limit == 0 {
		req.Limit = feeds.PageSize
	}

	browse := services.NewGraphNodeRequest().Browse
	browse.Types = []string{feeds.NodeType}
	browse.Search = strings.TrimSpace(req.Search)
	browse.Offset = req.Start
	// One more than the page, to know whether there is another.
	browse.Limit = req.Limit + 1
	browse.Viewer = viewer
	browse.AccessCheck = true

	found, err := h.Srv.GraphNodeGRPC.Browse(browse)
	if err != nil {
		log.Println("custom feed: discover:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	hasMore := uint64(len(found)) > req.Limit
	if hasMore {
		found = found[:req.Limit]
	}

	shared := make([]*entities.AnahitaNode, 0, len(found))
	for _, feed := range found {
		if feed.Type == feeds.NodeType && feed.Access != constants.AnahitaAccessMyself {
			shared = append(shared, feed)
		}
	}

	_, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		log.Println(err)
	}

	return c.JSON(http.StatusOK, DiscoverPage{
		Data: h.respondMany(shared, viewer, pinned),
		Pagination: DiscoverPagination{
			Limit:     req.Limit,
			Start:     req.Start,
			NextStart: req.Start + uint64(len(found)),
			HasMore:   hasMore,
		},
	})
}

// Pinned lists the feeds the viewer keeps on their home page, in order:
// their own and other people's. One they may no longer see is left out.
func (h *customFeedHandler) Pinned(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	list, pinned, err := h.pinnedFeeds(viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, responses.Unpaged[*CustomFeedResponse]{Data: h.respondMany(list, viewer, pinned)})
}

func (h *customFeedHandler) pinnedFeeds(viewer *entities.Viewer) ([]*entities.AnahitaNode, map[int64]bool, error) {
	ids, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return []*entities.AnahitaNode{}, pinned, nil
	}

	req := services.NewGraphNodeRequest().BatchRead
	req.IDs = ids
	req.Viewer = viewer
	nodes, err := h.Srv.GraphNodeGRPC.BatchRead(req)
	if err != nil {
		return nil, nil, err
	}

	visible, err := h.Srv.GraphNodeGRPC.FilterVisible(ids, viewer)
	if err != nil {
		return nil, nil, err
	}
	seen := make(map[int64]bool, len(visible))
	for _, id := range visible {
		seen[id] = true
	}

	byID := make(map[int64]*entities.AnahitaNode, len(nodes))
	for _, node := range nodes {
		if node.Type != feeds.NodeType {
			continue
		}
		if ownsFeed(viewer, node) || viewer.IsAdmin() || seen[node.ID] {
			byID[node.ID] = node
		}
	}

	list := make([]*entities.AnahitaNode, 0, len(ids))
	for _, id := range ids {
		if node, ok := byID[id]; ok {
			list = append(list, node)
		}
	}

	return list, pinned, nil
}

// SetPins replaces the feeds on the viewer's home page with these, in this
// order. An empty list takes them all off.
func (h *customFeedHandler) SetPins(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &CustomFeedPinsRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return refuseFeed(c, http.StatusUnprocessableEntity, "too_many_pins")
	}

	ids := make([]int64, 0, len(req.IDs))
	seen := map[int64]bool{}
	for _, id := range req.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if h.read(id, viewer) == nil {
			return refuseFeed(c, http.StatusUnprocessableEntity, "feed_not_found")
		}
		ids = append(ids, id)
	}

	if err := h.writePins(viewer, ids); err != nil {
		log.Println("custom feed: pins:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return h.Pinned(c)
}

// Pin keeps one feed on the viewer's home page, after the ones there.
func (h *customFeedHandler) Pin(c echo.Context) error {
	return h.setPinned(c, true)
}

// Unpin takes one feed off the viewer's home page.
func (h *customFeedHandler) Unpin(c echo.Context) error {
	return h.setPinned(c, false)
}

func (h *customFeedHandler) setPinned(c echo.Context, pin bool) error {
	viewer := getViewer(c)
	if !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &CustomFeedIDRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	current, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Taking one off needs nothing of the feed: it may be gone, or no
	// longer the viewer's to see.
	if !pin {
		if !pinned[req.ID] {
			return c.NoContent(http.StatusNoContent)
		}
		kept := make([]int64, 0, len(current))
		for _, id := range current {
			if id != req.ID {
				kept = append(kept, id)
			}
		}
		if err := h.writePins(viewer, kept); err != nil {
			log.Println("custom feed: unpin:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		return c.NoContent(http.StatusNoContent)
	}

	if h.read(req.ID, viewer) == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if pinned[req.ID] {
		return c.NoContent(http.StatusNoContent)
	}
	if len(current) >= feeds.MaxPinned {
		return refuseFeed(c, http.StatusConflict, "too_many_pins")
	}

	if err := h.writePins(viewer, append(current, req.ID)); err != nil {
		log.Println("custom feed: pin:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}

// Items returns a page of a feed: what it shows, as the viewer may see it.
func (h *customFeedHandler) Items(c echo.Context) error {
	viewer := getViewer(c)

	req := &CustomFeedItemsRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	feed := h.read(req.ID, viewer)
	if feed == nil {
		return c.NoContent(http.StatusNotFound)
	}

	def, err := definitionOf(feed)
	if err != nil {
		return refuseFeed(c, http.StatusUnprocessableEntity, string(feeds.RefusedMalformed))
	}

	before, err := feeds.ParseCursor(req.Cursor)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// A feed of what is near its reader is read around where they are,
	// which they say with each page.
	if def.NeedsPoint() {
		if req.NearLatitude == nil || req.NearLongitude == nil {
			return refuseFeed(c, http.StatusUnprocessableEntity, "location_needed")
		}
		if !def.AtPoint(*req.NearLatitude, *req.NearLongitude) {
			return refuseFeed(c, http.StatusUnprocessableEntity, string(feeds.RefusedNear))
		}
	}

	// "Whoever I follow" is whoever the feed's owner follows.
	return h.page(c, def, feed.OwnerID, before, req.Limit, viewer)
}

// Preview returns the first page of a feed that has not been saved, so that
// whoever is building it can see what it would show. As they would see it,
// and as theirs.
func (h *customFeedHandler) Preview(c echo.Context) error {
	viewer := getViewer(c)
	if !mayWriteFeeds(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &CustomFeedPreviewRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	def, err := feeds.Parse(req.Definition)
	if err != nil {
		reason := string(feeds.RefusedMalformed)
		if refusal, ok := feeds.AsRefusal(err); ok {
			reason = string(refusal)
		}
		return refuseFeed(c, http.StatusUnprocessableEntity, reason)
	}

	return h.page(c, def, viewer.ID, feeds.Cursor{}, feeds.PageSize, viewer)
}

func (h *customFeedHandler) page(c echo.Context, def *feeds.Definition, ownerID int64, before feeds.Cursor, limit uint64, viewer *entities.Viewer) error {
	if limit == 0 || limit > feeds.MaxPage {
		limit = feeds.PageSize
	}

	resp := &CustomFeedPage{Data: []*responses.NodeItem{}}
	resp.Pagination.Limit = limit

	// One more than the page, to know whether there is another.
	nodes, err := h.Srv.FeedCustomGRPC.Browse(def, ownerID, before, limit+1, viewer)
	if err != nil {
		if errors.Is(err, services.ErrFeedTooBroad) {
			return refuseFeed(c, http.StatusUnprocessableEntity, string(feeds.RefusedTooBroad))
		}
		log.Println("custom feed: items:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if uint64(len(nodes)) > limit {
		nodes = nodes[:limit]
		resp.Pagination.HasMore = true
	}
	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}
	if resp.Pagination.HasMore {
		last := nodes[len(nodes)-1]
		resp.Pagination.NextCursor = feeds.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.String()
	}

	if quoted := an_handlers.SetQuotes(h.Srv.MediaGRPC, h.Srv.GraphNodeGRPC, nodes, viewer); len(quoted) > 0 {
		batchPresignPaths(h.Env, quoted)
	}

	batchPresignPaths(h.Env, nodes)
	setFeedFlags(h.Env, nodes, viewer)

	nodeResp := &responses.NodeResponse{}
	resp.Data = nodeResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}
