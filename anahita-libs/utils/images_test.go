package images

import (
	"bytes"
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
