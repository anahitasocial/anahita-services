package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"

	"github.com/labstack/echo/v4"
)

func TestPreviewTextLeavesWhatFitsExactlyAsWritten(t *testing.T) {
	short := "A **short** note with a [link](https://example.com)."

	got, cut := previewText(short)
	if cut || got != short {
		t.Errorf("a note that fits was changed: %q, cut=%v", got, cut)
	}
}

func TestPreviewTextCutsAtAWordAndDropsTheFormatting(t *testing.T) {
	long := "# A heading\n\n" +
		"This is **bold**, this is a [link to somewhere](https://example.com/a/very/long/address), " +
		"and here is a picture ![alt text](https://example.com/picture.jpg) in the middle of it. " +
		strings.Repeat("Then it goes on for a good while longer than a preview allows. ", 8)

	got, cut := previewText(long)
	if !cut {
		t.Fatal("a long text was not cut")
	}
	if n := utf8.RuneCountInString(got); n > PreviewBodyLength+1 {
		t.Errorf("the preview is %d characters, over the %d allowed", n, PreviewBodyLength)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("the preview does not say it was cut: %q", got)
	}
	for _, leftover := range []string{"#", "**", "](", "https://", "![", "\n"} {
		if strings.Contains(got, leftover) {
			t.Errorf("the preview still has %q in it: %q", leftover, got)
		}
	}
	if !strings.HasPrefix(got, "A heading This is bold, this is a link to somewhere") {
		t.Errorf("the words were not kept: %q", got)
	}
	// Ends on a whole word.
	if strings.HasSuffix(strings.TrimSuffix(got, "…"), "allow") {
		t.Errorf("cut in the middle of a word: %q", got)
	}
}

// Counted in characters, not bytes: a text in a script that takes three
// bytes a character must get the same 280 and not be cut mid-character.
func TestPreviewTextCountsCharacters(t *testing.T) {
	long := strings.Repeat("آناهیتا ", 60)

	got, cut := previewText(long)
	if !cut || !utf8.ValidString(got) {
		t.Fatalf("cut=%v valid=%v: %q", cut, utf8.ValidString(got), got)
	}
	if n := utf8.RuneCountInString(got); n < PreviewBodyLength/2 || n > PreviewBodyLength+1 {
		t.Errorf("the preview is %d characters", n)
	}
}

func decode(t *testing.T, document string) map[string]any {
	t.Helper()

	rewritten, ok := previewJSON([]byte(document))
	if !ok {
		t.Fatalf("not rewritten: %s", document)
	}

	var value map[string]any
	if err := json.Unmarshal(rewritten, &value); err != nil {
		t.Fatalf("the rewritten document is not JSON: %s", rewritten)
	}
	return value
}

func TestPreviewOfAListOfPosts(t *testing.T) {
	long := strings.Repeat("word ", 100)

	list := decode(t, `{
		"data": [
			{"id": 9007199254740993, "content": "`+long+`", "owner": {"id": 5, "body": "`+long+`",
			 "avatar_urls": {"small": {"url": "a"}, "large": {"url": "b"}}},
			 "portrait_urls": {"original": {"url": "o"}, "small": {"url": "s"}, "large": {"url": "l"}}},
			{"id": 2, "content": "short"}
		],
		"pagination": {"offset": 0, "limit": 10, "total": 165}
	}`)

	items := list["data"].([]any)
	first := items[0].(map[string]any)

	if first["truncated"] != true || !strings.HasSuffix(first["content"].(string), "…") {
		t.Errorf("the long post was not cut and marked: %v", first["content"])
	}

	// Only the smallest picture of a photo.
	portraits := first["portrait_urls"].(map[string]any)
	if len(portraits) != 1 || portraits["small"] == nil {
		t.Errorf("portraits sent: %v", portraits)
	}

	// The profile inside it: its text is cut the same way, and its avatar is
	// left at every size.
	owner := first["owner"].(map[string]any)
	if owner["truncated"] != true {
		t.Error("a long profile text inside a post was not cut")
	}
	if avatars := owner["avatar_urls"].(map[string]any); len(avatars) != 2 {
		t.Errorf("avatars were cut down: %v", avatars)
	}

	second := items[1].(map[string]any)
	if second["content"] != "short" || second["truncated"] != nil {
		t.Errorf("a short post was touched: %v", second)
	}

	// There is more, and no way to page to it: the total is what was sent.
	pagination := list["pagination"].(map[string]any)
	if pagination["more"] != true || pagination["total"] != float64(2) {
		t.Errorf("pagination: %v", pagination)
	}
}

// An id too large for a float must come out as it went in.
// A photo post with several images shows a visitor one of them, small, and
// says how many more there are.
func TestPreviewOfAPostWithSeveralImages(t *testing.T) {
	post := decode(t, `{
		"id": 4,
		"portrait_urls": {"small": {"url": "s"}, "large": {"url": "l"}},
		"files": [
			{"id": "a.jpg", "alt_text": "a heron", "urls": {"small": {"url": "as"}, "xxlarge": {"url": "ax"}}},
			{"id": "b.jpg", "alt_text": "", "urls": {"small": {"url": "bs"}, "xxlarge": {"url": "bx"}}},
			{"id": "c.jpg", "alt_text": "", "urls": {"small": {"url": "cs"}, "xxlarge": {"url": "cx"}}}
		]
	}`)

	files := post["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("%d images sent, want 1", len(files))
	}

	first := files[0].(map[string]any)
	if first["id"] != "a.jpg" || first["alt_text"] != "a heron" {
		t.Errorf("the image kept is not the first, or lost its description: %v", first)
	}
	if urls := first["urls"].(map[string]any); len(urls) != 1 || urls["small"] == nil {
		t.Errorf("sizes sent: %v", urls)
	}

	if post["files_held_back"] != float64(2) {
		t.Errorf("held back = %v, want 2", post["files_held_back"])
	}
}

// One image is all there is, so nothing is held back.
func TestPreviewOfAPostWithOneImageHoldsNothingBack(t *testing.T) {
	post := decode(t, `{"id": 4, "files": [{"id": "a.jpg", "urls": {"small": {"url": "as"}, "large": {"url": "al"}}}]}`)

	if len(post["files"].([]any)) != 1 || post["files_held_back"] != nil {
		t.Errorf("a single image was treated as several: %v", post)
	}
}

func TestPreviewKeepsLargeNumbers(t *testing.T) {
	rewritten, ok := previewJSON([]byte(`{"id": 9007199254740993, "body": "x"}`))
	if !ok || !strings.Contains(string(rewritten), "9007199254740993") {
		t.Errorf("the id was changed: %s", rewritten)
	}
}

func TestPreviewLeavesAListThatIsAllThere(t *testing.T) {
	list := decode(t, `{"data": [{"id": 1}], "pagination": {"offset": 0, "limit": 10, "total": 1}}`)

	if pagination := list["pagination"].(map[string]any); pagination["more"] != nil {
		t.Errorf("a complete list was marked as having more: %v", pagination)
	}
}

// ---------------------------------------------------------------------
// Through SetViewer
// ---------------------------------------------------------------------

func previewed(t *testing.T, target string, handler echo.HandlerFunc, more ...echo.MiddlewareFunc) (*httptest.ResponseRecorder, string) {
	t.Helper()

	restore := policy.SetReadAccessForTest(policy.ReadPreview)
	defer restore()

	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, target, nil), rec)

	seenLimit := ""
	wrapped := func(c echo.Context) error {
		seenLimit = c.QueryParam("limit")
		return handler(c)
	}

	chain := echo.HandlerFunc(wrapped)
	for i := len(more) - 1; i >= 0; i-- {
		chain = more[i](chain)
	}

	if err := SetViewer(nil, nil)(chain)(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return rec, seenLimit
}

func TestAnonymousReadersGetAPreview(t *testing.T) {
	long := strings.Repeat("word ", 100)

	rec, limit := previewed(t, "/notes/5/?limit=50", func(c echo.Context) error {
		viewer, _ := c.Get(constants.ViewerContextKey).(*entities.Viewer)
		if viewer == nil || viewer.ID != 0 {
			t.Errorf("the handler was not given an anonymous viewer")
		}
		return c.JSON(http.StatusOK, map[string]any{
			"data":       []map[string]any{{"id": 1, "content": long}},
			"pagination": map[string]any{"offset": 0, "limit": 10, "total": 40},
		})
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if limit != "10" {
		t.Errorf("the handler was asked for %s items, want a preview's 10", limit)
	}
	body := rec.Body.String()
	for _, want := range []string{`"truncated":true`, `"more":true`, `"total":1`} {
		if !strings.Contains(body, want) {
			t.Errorf("the answer has no %s: %s", want, body)
		}
	}
}

func TestAPreviewHasNoSecondPage(t *testing.T) {
	for _, target := range []string{"/notes/5/?start=10", "/people/?offset=20&limit=5"} {
		reached := false
		rec, _ := previewed(t, target, func(c echo.Context) error {
			reached = true
			return c.NoContent(http.StatusOK)
		})

		if reached || rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), ErrMembersOnly) {
			t.Errorf("%s: reached=%v, answered %d %s", target, reached, rec.Code, rec.Body.String())
		}
	}
}

// What is not a successful JSON answer goes out as the handler wrote it.
func TestPreviewLeavesErrorsAndOtherAnswersAlone(t *testing.T) {
	rec, _ := previewed(t, "/notes/999", func(c echo.Context) error {
		return c.NoContent(http.StatusNotFound)
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("a 404 came out as %d", rec.Code)
	}

	rec, _ = previewed(t, "/something", func(c echo.Context) error {
		return c.String(http.StatusOK, strings.Repeat("plain text ", 100))
	})
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "plain text plain text") ||
		len(rec.Body.String()) < 1000 {
		t.Errorf("plain text was changed: %d bytes", len(rec.Body.String()))
	}
}

// Replies and the lists of who follows whom are not part of a preview.
func TestSomeRoutesAreNotPreviewedAtAll(t *testing.T) {
	reached := false
	rec, _ := previewed(t, "/notes/100/thread", func(c echo.Context) error {
		reached = true
		return c.NoContent(http.StatusOK)
	}, MembersInPreview())

	if reached || rec.Code != http.StatusUnauthorized {
		t.Errorf("reached=%v, answered %d", reached, rec.Code)
	}

	// On an open site the same middleware does nothing.
	e := echo.New()
	open := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/notes/100/thread", nil), open)
	reached = false
	handler := SetViewer(nil, nil)(MembersInPreview()(func(c echo.Context) error {
		reached = true
		return c.NoContent(http.StatusOK)
	}))
	if err := handler(c); err != nil || !reached {
		t.Errorf("an open site refused replies: %v, reached=%v", err, reached)
	}
}
