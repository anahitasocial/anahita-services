package handlers

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	libconstants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libservices "anahita-libs/data/services"

	"identityimage/cmd/api/permissions"
	"identityimage/data/services"

	"github.com/labstack/echo/v4"
)

const (
	personType = "node.actor.person-service.person.v1"
	noteType   = "node.medium.text-service.note.v1"

	ada   = int64(7)  // the person whose profile it is
	grace = int64(99) // somebody else
)

// What the test's world holds: one node, read by every client.
type world struct {
	node *entities.AnahitaNode

	// What happened, in order.
	log []string

	stored  map[string][]byte
	deleted []string

	failEdit  bool
	failClear bool

	// Whether somebody who is not signed in is refused the node.
	hidden bool
}

func (w *world) copyOfNode() *entities.AnahitaNode {
	copied := *w.node
	return &copied
}

type fakeNodes struct {
	libservices.GraphNodeGRPCInterface
	w *world
}

func (f *fakeNodes) Read(id int64) (*entities.AnahitaNode, error) {
	if f.w.node == nil || f.w.node.ID != id {
		return nil, errors.New("not found")
	}
	return f.w.copyOfNode(), nil
}

func (f *fakeNodes) ReadByUUID(uuid string) (*entities.AnahitaNode, error) {
	if f.w.node == nil || f.w.node.UUID != uuid {
		return nil, nil
	}
	return f.w.copyOfNode(), nil
}

func (f *fakeNodes) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	if f.w.hidden || viewer == nil || viewer.ID != 0 {
		return nil, nil
	}
	return ids, nil
}

func (f *fakeNodes) ClearFile(id int64, kind string, modifiedBy int64) (bool, error) {
	if f.w.failClear {
		return false, errors.New("graph is away")
	}
	f.w.log = append(f.w.log, "clear "+kind)
	if kind == "avatar" {
		f.w.node.AvatarFilename = ""
	} else {
		f.w.node.CoverFilename = ""
	}
	return true, nil
}

type fakeActors struct {
	libservices.ActorsGRPCInterface
	w *world
}

func (f *fakeActors) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return f.w.copyOfNode(), nil
}

func (f *fakeActors) Edit(node *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	if f.w.failEdit {
		return nil, errors.New("graph is away")
	}
	f.w.log = append(f.w.log, "save")
	f.w.node = node
	return node, nil
}

type fakeMedia struct {
	libservices.MediaGRPCInterface
	w *world
}

func (f *fakeMedia) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return f.w.copyOfNode(), nil
}

func (f *fakeMedia) Edit(node *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	f.w.log = append(f.w.log, "save")
	f.w.node = node
	return node, nil
}

type fakeFollows struct {
	libservices.SocialgraphGRPCInterface
}

func (f *fakeFollows) Read(a, b int64) (*entities.ActorRelation, error) {
	return &entities.ActorRelation{}, nil
}

func (f *fakeFollows) Browse(ids []int64, viewer int64) (map[int64]*entities.ActorRelation, error) {
	out := map[int64]*entities.ActorRelation{}
	for _, id := range ids {
		out[id] = &entities.ActorRelation{}
	}
	return out, nil
}

type fakeFiles struct {
	libservices.FilesGRPCInterface
	w *world
}

func (f *fakeFiles) Add(req *libservices.FileAddRequest) (*entities.File, error) {
	f.w.log = append(f.w.log, "store")
	f.w.stored[req.Path] = req.Content
	return &entities.File{}, nil
}

func (f *fakeFiles) Delete(req *libservices.FileDeleteRequest) error {
	f.w.log = append(f.w.log, "delete")
	f.w.deleted = append(f.w.deleted, req.Path)
	delete(f.w.stored, req.Path)
	return nil
}

func (f *fakeFiles) Presign(req *libservices.FilePresignRequest) (string, error) {
	return "https://media.example/" + req.Path, nil
}

func newWorld(nodeType string) *world {
	node := entities.NewAnahitaNode()
	node.ID = ada
	node.Type = nodeType
	node.Enabled = true
	node.Access = "public"
	if nodeType == noteType {
		node.ID = 500
		node.OwnerID = ada
		node.CreatedBy = ada
		node.Owner.ID = ada
		node.Author.ID = ada
	}
	return &world{node: node, stored: map[string][]byte{}}
}

func handlerFor(w *world) *imageHandler {
	return NewImageHandler(&Env{
		Perms: permissions.NewPermissions(),
		Srv: &services.Services{
			FilesGRPC:       &fakeFiles{w: w},
			ActorsGRPC:      &fakeActors{w: w},
			MediaGRPC:       &fakeMedia{w: w},
			GraphNodeGRPC:   &fakeNodes{w: w},
			SocialgraphGRPC: &fakeFollows{},
		},
	})
}

func picture(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// request builds a request as the web app sends one: a form with the file in
// the field named for the kind of image.
func request(t *testing.T, method string, id int64, viewer int64, field string, content []byte) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var body bytes.Buffer
	contentType := ""
	if content != nil {
		form := multipart.NewWriter(&body)
		part, err := form.CreateFormFile(field, "upload.jpg")
		if err != nil {
			t.Fatal(err)
		}
		part.Write(content)
		form.Close()
		contentType = form.FormDataContentType()
	}

	req := httptest.NewRequest(method, "/x/", &body)
	if contentType != "" {
		req.Header.Set(echo.HeaderContentType, contentType)
	}
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetParamNames("node_id")
	c.SetParamValues(strconv.FormatInt(id, 10))

	who := entities.NewViewer()
	who.ID = viewer
	who.PersonType = "registered"
	// What a signed-in person's token carries. Without these nobody may edit
	// anything, their own profile included.
	who.Scopes = []string{libconstants.OAuthScopeActorsWrite, libconstants.OAuthScopeMediaWrite}
	c.Set(libconstants.ViewerContextKey, who)

	return c, rec
}

func sizeOf(t *testing.T, content []byte) (int, int) {
	t.Helper()
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("what was stored is not an image: %v", err)
	}
	return config.Width, config.Height
}

func find(t *testing.T, stored map[string][]byte, fragment string) []byte {
	t.Helper()
	for path, content := range stored {
		if strings.Contains(path, fragment) {
			return content
		}
	}
	t.Fatalf("nothing stored with %q in its path; stored: %v", fragment, keys(stored))
	return nil
}

func keys(stored map[string][]byte) []string {
	var out []string
	for path := range stored {
		out = append(out, path)
	}
	return out
}

func TestAnAvatarIsStoredAtEverySizeAndTheSquareIsNotSqueezed(t *testing.T) {
	w := newWorld(personType)
	c, rec := request(t, http.MethodPost, ada, ada, "avatar", picture(t, 1200, 600))

	if err := handlerFor(w).SetAvatar(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}

	if len(w.stored) != len(w.node.GetAvatarSizes()) {
		t.Errorf("stored %d sizes, want %d: %v", len(w.stored), len(w.node.GetAvatarSizes()), keys(w.stored))
	}
	if w.node.AvatarFilename == "" {
		t.Fatal("the profile does not name its new avatar")
	}
	// In the column that is an avatar's own. `Filename` is the file a node
	// is, and a person is not a file.
	if w.node.Filename != "" {
		t.Errorf("the avatar was also written as the node's own file: %q", w.node.Filename)
	}

	// Twice as wide as it is tall, so a square made by pressing the whole
	// picture in would still be square: the test is that the other sizes
	// kept their shape and the square was cut.
	if width, height := sizeOf(t, find(t, w.stored, "_square.")); width != height {
		t.Errorf("the square is %dx%d", width, height)
	}
	if width, height := sizeOf(t, find(t, w.stored, "_large.")); width != 2*height {
		t.Errorf("the large size lost its shape: %dx%d", width, height)
	}

	// Stored where avatars have always been, under the service name they
	// were first stored by. That name is in the address of every avatar
	// there is.
	for path := range w.stored {
		if !strings.HasPrefix(path, "assets/v1/identity-image-service/avatars/person/7/") {
			t.Errorf("stored at %s", path)
		}
	}
}

// A small picture is not blown up to fill a large size.
func TestASmallPictureIsNotEnlarged(t *testing.T) {
	w := newWorld(personType)
	c, _ := request(t, http.MethodPost, ada, ada, "avatar", picture(t, 100, 80))

	if err := handlerFor(w).SetAvatar(c); err != nil {
		t.Fatal(err)
	}

	if width, height := sizeOf(t, find(t, w.stored, "_large.")); width != 100 || height != 80 {
		t.Errorf("a 100x80 picture was stored as %dx%d", width, height)
	}
	if width, height := sizeOf(t, find(t, w.stored, "_square.")); width != 80 || height != 80 {
		t.Errorf("its square is %dx%d, want 80x80", width, height)
	}
}

// Replacing an avatar removes the one before, and only once the profile
// names the new one.
func TestReplacingRemovesTheOldImageAfterTheNewOneIsSaved(t *testing.T) {
	w := newWorld(personType)
	w.node.AvatarFilename = "old.jpg"
	h := handlerFor(w)

	c, rec := request(t, http.MethodPost, ada, ada, "avatar", picture(t, 400, 400))
	if err := h.SetAvatar(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}

	if len(w.deleted) != len(w.node.GetAvatarSizes()) {
		t.Fatalf("deleted %v; want every size of the old avatar", w.deleted)
	}
	for _, path := range w.deleted {
		if !strings.Contains(path, "/old_") {
			t.Errorf("deleted %s, which is not the old avatar", path)
		}
	}

	saved, firstDelete := -1, -1
	for i, step := range w.log {
		if step == "save" && saved < 0 {
			saved = i
		}
		if step == "delete" && firstDelete < 0 {
			firstDelete = i
		}
	}
	if saved < 0 || firstDelete < saved {
		t.Errorf("order was %v; the old image must go after the profile is saved", w.log)
	}
}

// If the profile cannot be saved, the picture just stored is taken away
// again and the old one is left alone.
func TestAFailedSaveLeavesNothingBehindAndKeepsTheOldImage(t *testing.T) {
	w := newWorld(personType)
	w.node.AvatarFilename = "old.jpg"
	w.failEdit = true

	c, rec := request(t, http.MethodPost, ada, ada, "avatar", picture(t, 400, 400))
	if err := handlerFor(w).SetAvatar(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
	if len(w.stored) != 0 {
		t.Errorf("left in storage: %v", keys(w.stored))
	}
	for _, path := range w.deleted {
		if strings.Contains(path, "/old_") {
			t.Errorf("the old avatar was deleted though the new one was never saved: %s", path)
		}
	}
	if w.node.AvatarFilename != "old.jpg" {
		t.Errorf("the profile names %q", w.node.AvatarFilename)
	}
}

func TestWhatIsNotAnImageIsRefusedAndNothingIsStored(t *testing.T) {
	w := newWorld(personType)
	c, rec := request(t, http.MethodPost, ada, ada, "avatar", []byte("<html><script>alert(1)</script></html>"))

	if err := handlerFor(w).SetAvatar(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusUnsupportedMediaType || !strings.Contains(rec.Body.String(), errFileUnsupported) {
		t.Errorf("status = %d, body %s", rec.Code, rec.Body)
	}
	if len(w.stored) != 0 || len(w.log) != 0 {
		t.Errorf("something was done with it: %v", w.log)
	}
}

func TestSomebodyElseCannotChangeAPersonsAvatar(t *testing.T) {
	w := newWorld(personType)
	w.node.AvatarFilename = "old.jpg"
	h := handlerFor(w)

	c, rec := request(t, http.MethodPost, ada, grace, "avatar", picture(t, 400, 400))
	if err := h.SetAvatar(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("setting: status = %d", rec.Code)
	}

	c, rec = request(t, http.MethodDelete, ada, grace, "", nil)
	if err := h.RemoveAvatar(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("removing: status = %d", rec.Code)
	}

	if len(w.log) != 0 || w.node.AvatarFilename != "old.jpg" {
		t.Errorf("something changed: %v, names %q", w.log, w.node.AvatarFilename)
	}
}

// On a post the same column names the photo, so an avatar there is refused
// outright.
func TestAPostCannotHaveAnAvatarButCanHaveACover(t *testing.T) {
	w := newWorld(noteType)
	h := handlerFor(w)

	c, rec := request(t, http.MethodPost, 500, ada, "avatar", picture(t, 400, 400))
	if err := h.SetAvatar(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusForbidden || len(w.stored) != 0 {
		t.Errorf("an avatar on a post: status = %d, stored %v", rec.Code, keys(w.stored))
	}

	c, rec = request(t, http.MethodPost, 500, ada, "cover", picture(t, 2400, 800))
	if err := h.SetCover(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("a cover on a post: status = %d, body %s", rec.Code, rec.Body)
	}
	// The cover, and nothing else: not the post's own file, and not an
	// avatar a post cannot have.
	if w.node.CoverFilename == "" || w.node.Filename != "" || w.node.AvatarFilename != "" {
		t.Errorf("cover %q, filename %q, avatar %q", w.node.CoverFilename, w.node.Filename, w.node.AvatarFilename)
	}
	if width, _ := sizeOf(t, find(t, w.stored, "_large.")); width != 1600 {
		t.Errorf("the large cover is %d wide, want 1600", width)
	}
}

// Removing clears the name on the profile first, through the call made for
// that, and deletes the stored picture after.
func TestRemovingClearsTheNameAndThenDeletesTheImage(t *testing.T) {
	w := newWorld(personType)
	w.node.AvatarFilename = "old.jpg"

	c, rec := request(t, http.MethodDelete, ada, ada, "", nil)
	if err := handlerFor(w).RemoveAvatar(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if w.node.AvatarFilename != "" {
		t.Errorf("the profile still names %q", w.node.AvatarFilename)
	}
	if len(w.log) == 0 || w.log[0] != "clear avatar" {
		t.Errorf("order was %v; the name must be cleared before anything is deleted", w.log)
	}
	if len(w.deleted) != len(w.node.GetAvatarSizes()) {
		t.Errorf("deleted %v", w.deleted)
	}
}

// If the name cannot be cleared, the picture stays: a profile naming a
// picture that is gone is what this must never leave.
func TestIfTheNameCannotBeClearedTheImageIsKept(t *testing.T) {
	w := newWorld(personType)
	w.node.AvatarFilename = "old.jpg"
	w.failClear = true

	c, rec := request(t, http.MethodDelete, ada, ada, "", nil)
	if err := handlerFor(w).RemoveAvatar(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
	if len(w.deleted) != 0 {
		t.Errorf("deleted %v though the profile still names it", w.deleted)
	}
}

func TestRemovingWhatIsNotThereIsNotAFailure(t *testing.T) {
	w := newWorld(personType)

	c, rec := request(t, http.MethodDelete, ada, ada, "", nil)
	if err := handlerFor(w).RemoveCover(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK || len(w.log) != 0 {
		t.Errorf("status = %d, did %v", rec.Code, w.log)
	}
}

func TestANodeThatIsNotThereIsNotFound(t *testing.T) {
	w := newWorld(personType)

	c, rec := request(t, http.MethodPost, 12345, ada, "avatar", picture(t, 400, 400))
	if err := handlerFor(w).SetAvatar(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}
