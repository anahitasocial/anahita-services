package images

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func picture(t *testing.T, width, height int, asPNG bool) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 120, 255})
		}
	}

	var buf bytes.Buffer
	var err error
	if asPNG {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sizeOf(t *testing.T, content []byte) (int, int) {
	t.Helper()

	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("the result is not an image: %v", err)
	}
	return config.Width, config.Height
}

func TestFitImageNarrowsAWideImageAndKeepsItsShape(t *testing.T) {
	fitted, err := FitImage(picture(t, 2000, 1000, false), 640)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := sizeOf(t, fitted); w != 640 || h != 320 {
		t.Errorf("fitted to %dx%d, want 640x320", w, h)
	}
}

// A small image stored at a large size's name is still the small image.
func TestFitImageNeverEnlarges(t *testing.T) {
	fitted, err := FitImage(picture(t, 300, 200, true), 1920)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := sizeOf(t, fitted); w != 300 || h != 200 {
		t.Errorf("a 300x200 image came out %dx%d", w, h)
	}
}

func TestFitImageKeepsTheFormat(t *testing.T) {
	for _, asPNG := range []bool{true, false} {
		fitted, err := FitImage(picture(t, 800, 600, asPNG), 400)
		if err != nil {
			t.Fatal(err)
		}
		_, format, _ := image.DecodeConfig(bytes.NewReader(fitted))
		if want := map[bool]string{true: "png", false: "jpeg"}[asPNG]; format != want {
			t.Errorf("came out as %s, want %s", format, want)
		}
	}
}

// The square is the middle of the picture. Squeezing the whole picture into
// a square is what this replaces.
func TestSquareImageCutsTheMiddleAndDoesNotSqueeze(t *testing.T) {
	// Left third red, middle third green, right third blue.
	img := image.NewRGBA(image.Rect(0, 0, 900, 300))
	for x := 0; x < 900; x++ {
		shade := color.RGBA{255, 0, 0, 255}
		if x >= 300 {
			shade = color.RGBA{0, 255, 0, 255}
		}
		if x >= 600 {
			shade = color.RGBA{0, 0, 255, 255}
		}
		for y := 0; y < 300; y++ {
			img.Set(x, y, shade)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	square, err := SquareImage(buf.Bytes(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := sizeOf(t, square); w != 100 || h != 100 {
		t.Fatalf("square is %dx%d", w, h)
	}

	decoded, err := png.Decode(bytes.NewReader(square))
	if err != nil {
		t.Fatal(err)
	}
	// Squeezed, the left edge would be red and the right edge blue. Cut from
	// the middle, all of it is green.
	for _, x := range []int{2, 50, 97} {
		r, g, b, _ := decoded.At(x, 50).RGBA()
		if g>>8 < 200 || r>>8 > 60 || b>>8 > 60 {
			t.Errorf("at x=%d the square is not from the middle of the picture: r=%d g=%d b=%d", x, r>>8, g>>8, b>>8)
		}
	}
}

func TestSquareImageIsNoLargerThanTheImage(t *testing.T) {
	square, err := SquareImage(picture(t, 400, 120, false), 640)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := sizeOf(t, square); w != 120 || h != 120 {
		t.Errorf("square of a 400x120 image is %dx%d, want 120x120", w, h)
	}
}

func TestSomethingThatIsNotAnImageIsRefused(t *testing.T) {
	page := []byte("<html><body>not a photo</body></html>")

	if _, err := FitImage(page, 640); err == nil {
		t.Error("a page of HTML was fitted")
	}
	if _, err := SquareImage(page, 640); err == nil {
		t.Error("a page of HTML was squared")
	}
}

func TestAcceptTakesAnImageAndSaysWhatItIs(t *testing.T) {
	upright, mimeType, err := Accept(picture(t, 400, 300, false), 1<<20, 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/jpeg" {
		t.Errorf("kind = %q", mimeType)
	}
	if w, h, _ := Dimensions(upright); w != 400 || h != 300 {
		t.Errorf("size = %dx%d", w, h)
	}
}

// Judged from the bytes, whatever the file was called.
func TestAcceptRefusesWhatIsNotAnImage(t *testing.T) {
	if _, _, err := Accept([]byte("<html><script>alert(1)</script></html>"), 1<<20, 1_000_000); !errors.Is(err, ErrUnsupported) {
		t.Errorf("a page of HTML: %v", err)
	}
}

func TestAcceptRefusesTooManyBytesOrPixels(t *testing.T) {
	image := picture(t, 400, 300, true)

	if _, _, err := Accept(image, len(image)-1, 1_000_000); !errors.Is(err, ErrTooLarge) {
		t.Errorf("one byte over: %v", err)
	}
	// 400 x 300 is 120,000 pixels.
	if _, _, err := Accept(image, 1<<20, 119_999); !errors.Is(err, ErrTooLarge) {
		t.Errorf("one pixel over: %v", err)
	}
	if _, _, err := Accept(image, 1<<20, 120_000); err != nil {
		t.Errorf("exactly at the limit was refused: %v", err)
	}
}

// The start of a real PNG and then nothing: it says it is an image and
// cannot be read as one.
func TestAcceptRefusesABrokenImage(t *testing.T) {
	broken := picture(t, 400, 300, true)[:20]

	if _, _, err := Accept(broken, 1<<20, 1_000_000); !errors.Is(err, ErrUnreadable) {
		t.Errorf("a cut-off PNG: %v", err)
	}
}

func TestFitLongest(t *testing.T) {
	for _, c := range []struct {
		w, h, side, wantW, wantH int
	}{
		{3000, 2000, 2048, 2048, 1365},
		{2000, 3000, 2048, 1365, 2048},
		{800, 600, 2048, 800, 600},
	} {
		img := image.NewRGBA(image.Rect(0, 0, c.w, c.h))
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			t.Fatal(err)
		}
		out, err := FitLongest(buf.Bytes(), c.side)
		if err != nil {
			t.Fatal(err)
		}
		w, h, err := Dimensions(out)
		if err != nil {
			t.Fatal(err)
		}
		if w != c.wantW || h != c.wantH {
			t.Errorf("%dx%d fitted to %d: got %dx%d, want %dx%d", c.w, c.h, c.side, w, h, c.wantW, c.wantH)
		}
	}
}

func TestAsPhoto(t *testing.T) {
	opaque := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	for i := range opaque.Pix {
		opaque.Pix[i] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, opaque); err != nil {
		t.Fatal(err)
	}
	if _, kind, err := AsPhoto(buf.Bytes(), "image/png"); err != nil || kind != "image/jpeg" {
		t.Errorf("an opaque PNG: got %q, %v; want image/jpeg", kind, err)
	}

	clear := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	buf.Reset()
	if err := png.Encode(&buf, clear); err != nil {
		t.Fatal(err)
	}
	if _, kind, err := AsPhoto(buf.Bytes(), "image/png"); err != nil || kind != "image/png" {
		t.Errorf("a see-through PNG: got %q, %v; want image/png", kind, err)
	}
}
