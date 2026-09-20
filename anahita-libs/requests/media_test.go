package requests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
