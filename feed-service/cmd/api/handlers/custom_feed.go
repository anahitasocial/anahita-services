package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/feeds"
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

// CustomFeedFields is what makes a custom feed.
type CustomFeedFields struct {
	// What it is called.
	Name string `json:"name" validate:"required,max=100"`
	// What it is for, in its owner's words.
	Body string `json:"body" validate:"omitempty,max=500"`
	// Who may see it: `myself`, `followers`, `registered` or `public`. Not
	// "people I follow" or "mutuals": a feed is shared widely or kept.
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
