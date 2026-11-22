package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/permissions"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// settingsMedia keeps the posts a setting is asked of, and what was saved.
type settingsMedia struct {
	services.MediaGRPCInterface
	posts  map[int64]*entities.AnahitaNode
	saved  []*entities.AnahitaNode
	pinned map[int64]bool
}

func (f *settingsMedia) Read(id int64, _ *entities.Viewer) (*entities.AnahitaNode, error) {
	post, ok := f.posts[id]
	if !ok {
		return nil, echo.ErrNotFound
	}
	copied := *post
	return &copied, nil
}

func (f *settingsMedia) Edit(post *entities.AnahitaNode, _ *entities.Viewer) (*entities.AnahitaNode, error) {
	f.saved = append(f.saved, post)
	f.posts[post.ID] = post
	return post, nil
}

func (f *settingsMedia) SetPinned(id int64, pinned bool, _ *entities.Viewer) error {
	f.pinned[id] = pinned
	return nil
}

type settingsSocialgraph struct{ services.SocialgraphGRPCInterface }

func (settingsSocialgraph) Browse(ids []int64, viewer int64) (map[int64]*entities.ActorRelation, error) {
	return map[int64]*entities.ActorRelation{}, nil
}

// settingsPerms lets whoever it is told edit.
type settingsPerms struct {
	permissions.MediumPermissionsInterface
	mayEdit bool
}

func (p settingsPerms) CanEdit(*permissions.MediumAccessContext) (bool, error) { return p.mayEdit, nil }

// settingsWorld is the posts of one person, Naomi (id 5): a note (10), a
// reply under it (11), a channel's item of its feed (12) and a reply
// deleted (13).
func settingsWorld(mayEdit bool) (PostSettings, *settingsMedia, *int) {
	post := func(id, root int64) *entities.AnahitaNode {
		n := entities.NewAnahitaNode()
		n.ID, n.RootID, n.OwnerID, n.CreatedBy = id, root, 5, 5
		n.Type = "node.medium.text-service.note.v1"
		n.Access = constants.AnahitaAccessPublic
		n.Owner = entities.NewAnahitaNode()
		n.Owner.ID, n.Owner.Access = 5, constants.AnahitaAccessPublic
		return n
	}
	channelItem := post(12, 0)
	channelItem.OwnerType = constants.NodeActorChannelEntityType
	channelItem.Meta = []byte(`{"from_feed":true}`)
	gone := post(13, 10)
	gone.DeletedAt = &gone.CreatedAt

	media := &settingsMedia{
		posts:  map[int64]*entities.AnahitaNode{10: post(10, 0), 11: post(11, 10), 12: channelItem, 13: gone},
		pinned: map[int64]bool{},
	}
	answered := 0
	return PostSettings{
		Media:       media,
		Socialgraph: settingsSocialgraph{},
		Perms:       settingsPerms{mayEdit: mayEdit},
		Answer: func(c echo.Context, post *entities.AnahitaNode) error {
			answered++
			return c.NoContent(http.StatusOK)
		},
	}, media, &answered
}

// ask makes a request of a setting, as Naomi, with a form and the post's id.
func ask(handler echo.HandlerFunc, id int64, form url.Values) *httptest.ResponseRecorder {
	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())
	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(strconv.FormatInt(id, 10))
	viewer := entities.NewViewer()
	viewer.ID, viewer.PersonType, viewer.Authenticated = 5, constants.PersonTypeRegistered, true
	c.Set(constants.ViewerContextKey, viewer)
	_ = handler(c)
	return rec
}

// THE RULE THIS GUARDS (roadmap row 170): every kind of post's settings
// are answered by this one code, as text, photo and video posts' copies
// each did: a setting is changed by whoever may edit the post, and saved
// with who changed it; a reply has none of its own; nothing is saved for
// somebody who may not.
func TestAPostsSettingsAreChangedByWhoeverMayEditIt(t *testing.T) {
	settings, media, answered := settingsWorld(true)

	rec := ask(settings.EditQuotePolicy, 10, url.Values{"quote_policy": {"nobody"}})
	if rec.Code != http.StatusOK || *answered != 1 || len(media.saved) != 1 {
		t.Fatalf("quote policy: %d, answered %d, saved %d", rec.Code, *answered, len(media.saved))
	}
	if saved := media.saved[0]; saved.QuotePolicy != "nobody" || saved.ModifiedBy != 5 {
		t.Errorf("saved %q by %d", saved.QuotePolicy, saved.ModifiedBy)
	}
	if rec := ask(settings.EditReplyAccess, 10, url.Values{"reply_access": {"followers"}}); rec.Code != http.StatusOK || media.posts[10].ReplyAccess != "followers" {
		t.Errorf("reply access: %d %q", rec.Code, media.posts[10].ReplyAccess)
	}
	if rec := ask(settings.EditReplyAccess, 10, url.Values{"reply_access": {"everyone-at-once"}}); rec.Code != http.StatusBadRequest {
		t.Errorf("a reply access that is none: %d", rec.Code)
	}

	// A reply follows its thread, and is not pinned.
	for name, handler := range map[string]echo.HandlerFunc{
		"access": settings.EditAccess, "comment status": settings.EditCommentStatus,
		"reply access": settings.EditReplyAccess, "quote policy": settings.EditQuotePolicy,
	} {
		rec := ask(handler, 11, url.Values{"access": {"public"}, "status": {"1"}, "reply_access": {"anyone"}, "quote_policy": {"anyone"}})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "reply_follows_its_thread") {
			t.Errorf("%s of a reply: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if rec := ask(settings.Pin, 11, nil); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "reply_cannot_be_pinned") {
		t.Errorf("pinning a reply: %d %s", rec.Code, rec.Body.String())
	}

	// A channel's own item of its feed is public, whoever asks; a deleted
	// reply is gone.
	if rec := ask(settings.EditAccess, 12, url.Values{"access": {"followers"}}); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), permissions.ChannelFeedItemRefusal) {
		t.Errorf("a channel's item: %d %s", rec.Code, rec.Body.String())
	}
	if rec := ask(settings.EditWarning, 13, url.Values{"warning": {"other"}, "warning_text": {"spoilers"}}); rec.Code != http.StatusGone {
		t.Errorf("a deleted reply's warning: %d", rec.Code)
	}

	// Nothing is saved for somebody who may not edit it.
	settings, media, answered = settingsWorld(false)
	for name, handler := range map[string]echo.HandlerFunc{
		"comment status": settings.EditCommentStatus, "reply access": settings.EditReplyAccess,
		"quote policy": settings.EditQuotePolicy, "warning": settings.EditWarning,
	} {
		rec := ask(handler, 10, url.Values{"status": {"1"}, "reply_access": {"anyone"}, "quote_policy": {"anyone"}, "warning": {"other"}, "warning_text": {"spoilers"}})
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s by somebody who may not: %d", name, rec.Code)
		}
	}
	if len(media.saved) != 0 || *answered != 0 {
		t.Errorf("saved %d and answered %d for somebody who may not", len(media.saved), *answered)
	}
}
