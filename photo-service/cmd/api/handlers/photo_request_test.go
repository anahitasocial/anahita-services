package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"anahita-libs/requests"

	"github.com/labstack/echo/v4"
)

func bindPhotoAdd(t *testing.T, req *http.Request) *photoAddRequest {
	t.Helper()

	e := echo.New()
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetParamNames("owner_id")
	c.SetParamValues("8")

	bound := &photoAddRequest{MediumAddRequest: *requests.NewMediumRequest().Add}
	if err := c.Bind(bound); err != nil {
		t.Fatalf("bind: %v", err)
	}
	return bound
}

// The way a post with several images is sent: JSON naming the uploads. The
// fields a photo shares with every other post are in a struct this one
// embeds, and they have to arrive through it.
func TestAPhotoPostSentAsJSONNamesItsUploads(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/photos/8/", strings.NewReader(`{
		"name": "Herons",
		"body": "at the lagoon",
		"access": "followers",
		"language": "en",
		"uploads": [
			{"upload_id": 31, "alt_text": "a heron standing"},
			{"upload_id": 30}
		]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	bound := bindPhotoAdd(t, req)

	if bound.OwnerID != 8 || bound.Name != "Herons" || bound.Body != "at the lagoon" ||
		bound.Access != "followers" || bound.Language != "en" {
		t.Errorf("the shared fields did not arrive: %+v", bound.MediumAddRequest)
	}
	if len(bound.Uploads) != 2 || bound.Uploads[0].UploadID != 31 ||
		bound.Uploads[0].AltText != "a heron standing" || bound.Uploads[1].UploadID != 30 {
		t.Errorf("uploads = %+v", bound.Uploads)
	}
}

// The way a photo was posted before it could hold more than one image, which
// older clients still use: a form carrying the file.
func TestAPhotoPostSentAsAFormCarriesOneImage(t *testing.T) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for field, value := range map[string]string{
		"name": "Heron", "body": "one", "access": "public", "alt_text": "a heron",
	} {
		if err := form.WriteField(field, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := form.CreateFormFile("file", "heron.jpg")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("not really an image"))
	form.Close()

	req := httptest.NewRequest(http.MethodPost, "/photos/8/", &body)
	req.Header.Set(echo.HeaderContentType, form.FormDataContentType())

	bound := bindPhotoAdd(t, req)

	if bound.OwnerID != 8 || bound.Name != "Heron" || bound.Access != "public" || bound.AltText != "a heron" {
		t.Errorf("the form's fields did not arrive: %+v, alt %q", bound.MediumAddRequest, bound.AltText)
	}
	if len(bound.Uploads) != 0 {
		t.Errorf("a form post named uploads: %+v", bound.Uploads)
	}
}

// When neither says, the audience is the installation's default, as for any
// post.
func TestAPhotoPostKeepsTheDefaultAudience(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/photos/8/", strings.NewReader(`{"name": "x", "body": "y", "uploads": [{"upload_id": 1}]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	bound := bindPhotoAdd(t, req)

	if want := requests.NewMediumRequest().Add.Access; bound.Access != want {
		t.Errorf("audience = %q, want the default %q", bound.Access, want)
	}
}

func TestTheImagesOfAnEditArriveInOrder(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/photos/40/files", strings.NewReader(`{
		"files": [{"id": "b.jpg", "alt_text": "second, now first"}, {"upload_id": 9}, {"id": "a.jpg"}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetParamNames("id")
	c.SetParamValues("40")

	bound := &photoFilesRequest{}
	if err := c.Bind(bound); err != nil {
		t.Fatalf("bind: %v", err)
	}

	if bound.ID != 40 || len(bound.Files) != 3 || bound.Files[0].ID != "b.jpg" ||
		bound.Files[0].AltText != "second, now first" || bound.Files[1].UploadID != 9 {
		t.Errorf("bound = %+v", bound)
	}
}
