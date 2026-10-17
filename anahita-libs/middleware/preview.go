package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"

	"github.com/labstack/echo/v4"
)

// Preview: what somebody who is not signed in gets on an installation set to
// SITE_READ_ACCESS=preview.
//
// They still pass every access gate for what is public. What changes is how
// much of it they are sent: the start of a post, one small picture, the first
// page of a list. The rest is behind signing in.
//
// IT IS DONE HERE, ON THE WAY OUT, and not in each handler. The API is public,
// so hiding things in the web app would hide nothing; and there are some
// thirty routes that serve anonymous readers, each building its own response.
// A rule every one of them has to remember to apply is a rule the next one
// will not. SetViewer is the one place all of them pass through, so it
// rewrites what they answer: any JSON, whatever its shape, by the names of its
// fields.
//
// The cost is decoding and encoding each response a second time. It is paid
// only for readers who are not signed in, on installations that chose this.

const (
	// PreviewPageSize is how many items of a list a preview shows. There is
	// no second page.
	PreviewPageSize = 10

	// PreviewBodyLength is how much of a post's text a preview shows, in
	// characters. About a paragraph: enough to tell what it is.
	PreviewBodyLength = 280
)

// previewRequest applies the limits a preview puts on what may be ASKED for,
// before the handler runs. It reports false, having answered, when the
// request is for more than a preview gives.
func previewRequest(c echo.Context) bool {
	query := c.Request().URL.Query()

	// Past the first page. Both spellings the services use.
	for _, name := range []string{"start", "offset"} {
		if n, err := strconv.ParseUint(query.Get(name), 10, 64); err == nil && n > 0 {
			_ = refuseMembersOnly(c)
			return false
		}
	}

	// No more than a preview's worth, whatever was asked for.
	limit, err := strconv.ParseUint(query.Get("limit"), 10, 64)
	if err != nil || limit == 0 || limit > PreviewPageSize {
		query.Set("limit", strconv.Itoa(PreviewPageSize))
		c.Request().URL.RawQuery = query.Encode()
	}

	return true
}

func refuseMembersOnly(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, map[string]string{
		"error":       ErrMembersOnly,
		"description": "sign in to read this site",
	})
}

// MembersInPreview is for the routes a preview leaves out altogether:
// comments, and the lists of who follows whom. Chained AFTER SetViewer, it
// answers 401 to somebody who is not signed in on an installation set to
// preview, and does nothing anywhere else.
func MembersInPreview() echo.MiddlewareFunc {
	return func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			viewer, _ := c.Get(constants.ViewerContextKey).(*entities.Viewer)
			if policy.PreviewOnly() && (viewer == nil || viewer.ID == 0) {
				return refuseMembersOnly(c)
			}
			return h(c)
		}
	}
}

// previewResponse runs the handler, holding back what it writes, and sends
// the preview of it.
func previewResponse(c echo.Context, h echo.HandlerFunc) error {
	original := c.Response().Writer
	held := &heldResponse{header: original.Header(), status: http.StatusOK}
	c.Response().Writer = held

	err := h(c)

	c.Response().Writer = original
	if err != nil {
		return err
	}

	body := held.body.Bytes()

	// Only what is certainly a successful JSON answer is rewritten. Anything
	// else, an error or a redirect, goes out as the handler wrote it.
	isJSON := strings.Contains(original.Header().Get(echo.HeaderContentType), "json")
	if held.status == http.StatusOK && isJSON && len(body) > 0 {
		if rewritten, ok := previewJSON(body); ok {
			body = rewritten
		}
	}

	original.Header().Del(echo.HeaderContentLength)
	original.WriteHeader(held.status)
	_, err = original.Write(body)

	return err
}

// heldResponse collects a response instead of sending it.
type heldResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *heldResponse) Header() http.Header         { return w.header }
func (w *heldResponse) WriteHeader(status int)      { w.status = status }
func (w *heldResponse) Write(b []byte) (int, error) { return w.body.Write(b) }

// previewJSON returns the preview of a JSON document, or false if it is not
// one.
func previewJSON(document []byte) ([]byte, bool) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	// Numbers are kept as written. An id read into a float64 and written
	// back would come out changed once it is large enough.
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}

	rewritten, err := json.Marshal(previewValue(value))
	if err != nil {
		return nil, false
	}

	return rewritten, true
}

// previewValue applies the preview to one decoded value, and everything
// inside it.
//
// It knows responses by the names of their fields, which are the same across
// the services:
//
//	content, body  a post's words or a profile's description, cut to
//	               PreviewBodyLength, and the object marked "truncated": true
//	portrait_urls  a photo's sizes: only the smallest is kept
//	files          a photo post's images: only the first is kept, at its
//	               smallest size, and the object says how many were held
//	               back in "files_held_back"
//	data with      a list: marked "more": true when there is more of it than
//	  pagination   was sent, and its total lowered to what was sent, so that a
//	               client which pages by the total stops
//
// Avatars and covers are left as they are: they are a profile's face, and
// shown at every size.
func previewValue(value any) any {
	switch typed := value.(type) {
	case []any:
		for i, item := range typed {
			typed[i] = previewValue(item)
		}
		return typed

	case map[string]any:
		for key, item := range typed {
			typed[key] = previewValue(item)
		}

		for _, key := range []string{"content", "body"} {
			if words, ok := typed[key].(string); ok {
				if cut, wasCut := previewText(words); wasCut {
					typed[key] = cut
					typed["truncated"] = true
				}
			}
		}

		if sizes, ok := typed["portrait_urls"].(map[string]any); ok {
			typed["portrait_urls"] = smallestSize(sizes)
		}

		// A post with several images shows a visitor one, like any photo.
		// Without this the list would hand over every image at every size,
		// beside a portrait_urls cut down to a thumbnail.
		if files, ok := typed["files"].([]any); ok && len(files) > 0 {
			if first, ok := files[0].(map[string]any); ok {
				if sizes, ok := first["urls"].(map[string]any); ok {
					first["urls"] = smallestSize(sizes)
				}
			}
			if held := len(files) - 1; held > 0 {
				typed["files_held_back"] = held
			}
			typed["files"] = files[:1]
		}

		previewList(typed)

		return typed

	default:
		return value
	}
}

// previewList marks a list that has more than a preview shows.
func previewList(response map[string]any) {
	items, isList := response["data"].([]any)
	pagination, hasPagination := response["pagination"].(map[string]any)
	if !isList || !hasPagination {
		return
	}

	total, ok := pagination["total"].(json.Number)
	if !ok {
		return
	}

	if n, err := total.Int64(); err == nil && n > int64(len(items)) {
		pagination["more"] = true
		pagination["total"] = len(items)
	}
}

// The sizes a photo is stored in, smallest first, as far as a preview cares.
var previewSizes = []string{"small", "square", "medium"}

func smallestSize(sizes map[string]any) map[string]any {
	for _, name := range previewSizes {
		if size, ok := sizes[name]; ok {
			return map[string]any{name: size}
		}
	}

	// None of the names known here. Send nothing rather than guess which of
	// the others is small.
	return map[string]any{}
}

var (
	markdownImage  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	markdownLink   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	markdownMarks  = regexp.MustCompile("(?m)^\\s{0,3}(#{1,6}|>|[-*+]|\\d+\\.)\\s+|[*_`~]{1,3}")
	htmlTag        = regexp.MustCompile(`<[^>]+>`)
	whitespaceRuns = regexp.MustCompile(`\s+`)
)

// previewText returns the start of a text, as plain words, and whether
// anything was left out.
//
// A text that fits is returned exactly as it was, formatting and all: a
// preview is not a reason to flatten a two-line note. One that does not fit
// is cut, and the cut piece has its Markdown taken out, because half a link
// or an unclosed emphasis would otherwise be sent as the last thing in it.
func previewText(text string) (string, bool) {
	if utf8.RuneCountInString(text) <= PreviewBodyLength {
		return text, false
	}

	plain := markdownImage.ReplaceAllString(text, "")
	plain = markdownLink.ReplaceAllString(plain, "$1")
	plain = htmlTag.ReplaceAllString(plain, "")
	plain = markdownMarks.ReplaceAllString(plain, "")
	plain = strings.TrimSpace(whitespaceRuns.ReplaceAllString(plain, " "))

	runes := []rune(plain)
	if len(runes) <= PreviewBodyLength {
		// Short once the formatting is gone. Still marked: what is sent is
		// not what was written.
		return plain, true
	}

	cut := string(runes[:PreviewBodyLength])

	// Back to the end of the last whole word, unless that loses most of it.
	if space := strings.LastIndex(cut, " "); space > len(cut)/2 {
		cut = cut[:space]
	}

	return strings.TrimRight(cut, " ,;:.-") + "…", true
}
