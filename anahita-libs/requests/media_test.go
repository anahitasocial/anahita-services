package requests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// The web client sends status as "1"/"0", not "true"/"false".
func TestBindCommentStatus(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"status=1", true},
		{"status=0", false},
		{"status=true", true},
		{"status=false", false},
	}

	for _, tc := range cases {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPatch, "/photos/42/comment-status", strings.NewReader(tc.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath("/photos/:id/comment-status")
		c.SetParamNames("id")
		c.SetParamValues("42")

		got := NewMediumRequest().EditCommentStatus
		if err := c.Bind(got); err != nil {
			t.Fatalf("%s: bind failed: %v", tc.body, err)
		}
		if got.Status != tc.want {
			t.Errorf("%s: Status = %v, want %v", tc.body, got.Status, tc.want)
		}
		if got.ID != 42 {
			t.Errorf("%s: ID = %d, want 42", tc.body, got.ID)
		}
	}
}

// A media list is always one profile's. There is no list of every post on the
// site, so the owner has to be in the path and has to be a real id.
//
// It used to be optional, and readable from ?oid= as well, so leaving it out
// (or sending oid=0) returned every post there is.
func TestMediumBrowseNeedsAnOwnerInThePath(t *testing.T) {
	validate := validator.New()

	bind := func(target string, ownerParam string) (*MediumBrowseRequest, error) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		c := e.NewContext(req, httptest.NewRecorder())
		if ownerParam != "" {
			c.SetPath("/notes/:owner_id/")
			c.SetParamNames("owner_id")
			c.SetParamValues(ownerParam)
		}

		got := NewMediumRequest().Browse
		if err := c.Bind(got); err != nil {
			return got, err
		}
		return got, validate.Struct(got)
	}

	t.Run("an owner in the path is accepted", func(t *testing.T) {
		got, err := bind("/notes/5/?start=20&limit=10", "5")
		if err != nil {
			t.Fatalf("refused: %v", err)
		}
		if got.OwnerID != 5 || got.Offset != 20 || got.Limit != 10 {
			t.Errorf("owner=%d offset=%d limit=%d, want 5, 20, 10", got.OwnerID, got.Offset, got.Limit)
		}
	})

	refused := []struct {
		what   string
		target string
		owner  string
	}{
		{"no owner at all", "/notes/", ""},
		{"an owner in the query string only", "/notes/?oid=5", ""},
		{"owner zero", "/notes/0/", "0"},
		{"a negative owner", "/notes/-3/", "-3"},
	}

	for _, c := range refused {
		t.Run(c.what+" is refused", func(t *testing.T) {
			if got, err := bind(c.target, c.owner); err == nil {
				t.Errorf("accepted with owner %d", got.OwnerID)
			}
		})
	}

	t.Run("?oid= cannot override the owner in the path", func(t *testing.T) {
		got, err := bind("/notes/5/?oid=9", "5")
		if err != nil {
			t.Fatalf("refused: %v", err)
		}
		if got.OwnerID != 5 {
			t.Errorf("owner = %d, want the 5 from the path", got.OwnerID)
		}
	})
}
