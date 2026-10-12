package fetch

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// THE RULE THIS GUARDS: the service goes only to public addresses, on the
// web's two ports. Everything else in the package is reading what comes
// back.

func TestOnlyPublicAddressesOnTheWebsPorts(t *testing.T) {
	refused := []string{
		"127.0.0.1", "127.8.9.10", "::1", "0.0.0.0", "::",
		"10.0.0.5", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254",    // the cloud metadata address
		"fe80::1",            // link-local
		"fc00::1", "fd12::1", // unique local
		"100.64.0.1",           // carrier-grade NAT
		"198.18.0.1",           // benchmarking
		"192.0.2.1",            // documentation
		"224.0.0.1", "ff02::1", // multicast
		"255.255.255.255",  // broadcast
		"::ffff:127.0.0.1", // loopback, written as IPv6
		"::ffff:10.0.0.1",  // private, written as IPv6
		"::ffff:169.254.169.254",
		"64:ff9b::7f00:1", // loopback, through NAT64
	}
	for _, address := range refused {
		if PublicAddress(net.ParseIP(address), "443") {
			t.Errorf("%s was allowed", address)
		}
	}

	for _, address := range []string{"93.184.216.34", "1.1.1.1", "2606:4700:4700::1111"} {
		if !PublicAddress(net.ParseIP(address), "443") || !PublicAddress(net.ParseIP(address), "80") {
			t.Errorf("%s was refused", address)
		}
		for _, port := range []string{"22", "3306", "6379", "8080", "50051", ""} {
			if PublicAddress(net.ParseIP(address), port) {
				t.Errorf("%s was allowed on port %q", address, port)
			}
		}
	}

	if PublicAddress(nil, "443") {
		t.Error("no address at all was allowed")
	}
}

// The fetcher as it is built for use refuses a server on this machine, by
// the address it connects to and whatever name it was given for it.
func TestTheFetcherRefusesWhatIsNotPublic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a request reached a server on this machine")
	}))
	defer server.Close()

	fetcher := New()

	for _, address := range []string{
		server.URL,
		strings.Replace(server.URL, "127.0.0.1", "localhost", 1),
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"http://10.0.0.5/",
	} {
		_, _, err := fetcher.Page(address)
		if err == nil || !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: got %v", address, err)
		}
	}

	for _, address := range []string{"ftp://example.org/", "file:///etc/passwd", "gopher://example.org/"} {
		if _, _, err := fetcher.Page(address); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: got %v", address, err)
		}
	}
}

// local is a fetcher let through to servers on this machine, for testing
// what is read. Whether a connection may be made is tested above.
func local() *Fetcher {
	return newFetcher(func(ip net.IP, port string) bool {
		return ip != nil && ip.IsLoopback()
	})
}

func serve(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

func TestAPageIsReadFromItsHead(t *testing.T) {
	address := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != UserAgent {
			t.Errorf("fetched as %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head>
			<title>The page's own title</title>
			<meta name="description" content="Its own description">
			<meta property="og:title" content=" Compost &amp; you ">
			<meta property="og:description" content="How to make it">
			<meta property="og:site_name" content="The Allotment">
			<meta property="og:image" content="/pictures/heap.jpg">
			<meta name="twitter:title" content="Not this one">
			</head><body><meta property="og:title" content="Nor this, in the body"></body></html>`)
	})

	page, final, err := local().Page(address + "/post")
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Compost & you" || page.Description != "How to make it" || page.SiteName != "The Allotment" {
		t.Errorf("read %+v", page)
	}
	if page.Image != address+"/pictures/heap.jpg" {
		t.Errorf("picture at %q", page.Image)
	}
	if final != address+"/post" {
		t.Errorf("led to %q", final)
	}
}

func TestAPageWithLittleToSayStillHasAName(t *testing.T) {
	address := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Plain</title><meta property="og:image" content="javascript:alert(1)"></head></html>`)
	})

	page, _, err := local().Page(address)
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Plain" || page.Description != "" || page.SiteName != "127.0.0.1" {
		t.Errorf("read %+v", page)
	}
	if page.Image != "" {
		t.Errorf("a picture that is not a web address was kept: %q", page.Image)
	}
}

func TestWhatIsNotAPageIsRefused(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"another kind of file": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, "%PDF-1.4")
		},
		"an error": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusNotFound)
		},
		"a redirect that never ends": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, r.URL.Path+"x", http.StatusFound)
		},
	}
	for name, handler := range cases {
		if _, _, err := local().Page(serve(t, handler)); err == nil {
			t.Errorf("%s was read as a page", name)
		}
	}

	// A redirect to somewhere the service does not go is not followed.
	address := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/", http.StatusFound)
	})
	if _, _, err := New().Page(address); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("a redirect to the metadata address: %v", err)
	}
}

// No more than the top of a page is read, however much there is.
func TestOnlyTheTopOfAPageIsRead(t *testing.T) {
	sent := 0
	address := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><head><title>Long</title>")
		filler := strings.Repeat("<!-- padding -->", 4096)
		for i := 0; i < 400; i++ {
			n, err := fmt.Fprint(w, filler)
			sent += n
			if err != nil {
				return
			}
		}
	})

	page, _, err := local().Page(address)
	if err != nil || page.Title != "Long" {
		t.Fatalf("read %+v (%v)", page, err)
	}
	if sent > 8*MaxPageBytes {
		t.Errorf("the server was let send %d bytes", sent)
	}
}

func picture(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}

func TestAPictureIsMadeSmallAndWrittenOutAgain(t *testing.T) {
	original := picture(1600, 900)
	address := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(original)
	})

	small, name, mimeType, err := local().Thumbnail(address)
	if err != nil {
		t.Fatal(err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(small))
	if err != nil || config.Width != ThumbnailWidth {
		t.Errorf("kept at %d wide (%v)", config.Width, err)
	}
	if bytes.Equal(small, original) {
		t.Error("the file was kept as it came")
	}
	if !strings.HasSuffix(name, ".png") || len(name) != 64+4 || mimeType != "image/png" {
		t.Errorf("named %q, of type %q", name, mimeType)
	}

	// The same picture is the same name.
	_, again, _, _ := local().Thumbnail(address)
	if again != name {
		t.Errorf("the same picture was named %q and %q", name, again)
	}
}

func TestWhatIsNotAPictureIsRefused(t *testing.T) {
	cases := map[string][]byte{
		"a page called a picture": []byte("<html><script>alert(1)</script></html>"),
		"an SVG":                  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"nothing":                 {},
		"a picture cut off early": picture(400, 300)[:200],
	}
	for name, content := range cases {
		address := serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(content)
		})
		if _, _, _, err := local().Thumbnail(address); err == nil {
			t.Errorf("%s was kept", name)
		}
	}
}
