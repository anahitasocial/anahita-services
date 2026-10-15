package shell

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"anahita-libs/policy"
)

// Signer gives the address a stored picture can be read from for a while.
type Signer interface {
	Sign(key string) (string, error)
}

// How long what a page says of itself is kept before it is read again.
// Long enough that a post everybody is linking to is read once, short
// enough that one made private stops being described.
const keptFor = time.Minute

// How many pages are kept at once.
const keptPages = 2000

// Server sends the web app: its files as they are, and index.html, with
// the page's own tags in it, for every other address.
type Server struct {
	Template *Template
	Reader   Reader
	Signer   Signer
	// The folder the build is in.
	Dir string
	// "https://www.example.org", or "" to use the address asked for.
	PublicURL string
	// "https://api.example.org": where what other servers look for under
	// /.well-known is answered. "" to answer for none of it.
	APIURL string

	mu   sync.Mutex
	kept map[string]keptSubject
	now  func() time.Time
}

type keptSubject struct {
	subject *Subject
	until   time.Time
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clean := path.Clean("/" + r.URL.Path)

	if wellKnown[clean] {
		s.wellKnown(w, r, clean)
		return
	}

	if strings.HasPrefix(clean, ImagePath) {
		s.image(w, r, strings.TrimPrefix(clean, ImagePath))
		return
	}
	// The shell's own addresses are not pages of the app.
	if clean == "/_shell" || strings.HasPrefix(clean, "/_shell/") {
		http.NotFound(w, r)
		return
	}
	if s.file(w, r, clean) {
		return
	}

	s.page(w, r, clean)
}

// What another server asks of a site's own host to find things out about
// it, and which the API answers.
//
// Somebody here is known elsewhere as ana@example.org, so a server that
// wants to find her asks example.org, the host the web app is on, where
// she is. This shell is what answers on that host, and it knows nothing of
// people. The API does; so the question is sent on to it, as asked. A server
// asking these follows where it is sent.
var wellKnown = map[string]bool{
	"/.well-known/webfinger": true,
	"/.well-known/host-meta": true,
	"/.well-known/nodeinfo":  true,
}

func (s *Server) wellKnown(w http.ResponseWriter, r *http.Request, clean string) {
	if s.APIURL == "" {
		http.NotFound(w, r)
		return
	}

	to := s.APIURL + clean
	if r.URL.RawQuery != "" {
		to += "?" + r.URL.RawQuery
	}

	// Any site may ask, from a page in a browser too.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.Redirect(w, r, to, http.StatusFound)
}

// file sends one of the build's own files, and says whether there was one.
func (s *Server) file(w http.ResponseWriter, r *http.Request, clean string) bool {
	if clean == "/" || clean == "/index.html" || s.Dir == "" {
		return false
	}

	// Cleaned above, so it cannot climb out of the folder.
	name := filepath.Join(s.Dir, filepath.FromSlash(clean))
	info, err := os.Stat(name)
	if err != nil || info.IsDir() {
		// A file that is asked for by name and is not there is not a page
		// of the app: a missing script answered with index.html is run as
		// a script and fails in a way nobody can read.
		if strings.HasPrefix(clean, "/static/") {
			http.NotFound(w, r)
			return true
		}
		return false
	}

	if strings.HasPrefix(clean, "/static/") {
		// Named by what is in them: a changed file has another name.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.ServeFile(w, r, name)
	return true
}

func (s *Server) page(w http.ResponseWriter, r *http.Request, clean string) {
	tags := Tags{Type: "website"}
	base := s.base(r)
	address := clean

	if subject := s.subject(clean); subject != nil {
		tags = subject.Tags
		if subject.Path != "" {
			// A profile's own address, however it was reached.
			address = subject.Path
		}
		if subject.ImageKey != "" {
			tags.Image = base + ImagePath + subject.ImageName
		}
	}
	if tags.Image == "" {
		tags.Image = base + "/statics/media/ogimage.jpg"
		tags.ImageAlt = ""
		tags.LargeImage = true
	}
	tags.URL = base + address

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Never kept long: a new build has to be picked up at once, and what a
	// page says of itself follows who may see it.
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write([]byte(s.Template.Render(tags)))
}

// image sends whoever asks to a page's picture. The address asked for is
// the same for as long as the page is public; where it leads is signed
// afresh each time.
func (s *Server) image(w http.ResponseWriter, r *http.Request, name string) {
	if policy.MembersOnly() || s.Reader == nil || s.Signer == nil {
		http.NotFound(w, r)
		return
	}

	subject := ResolveImage(s.Reader, name)
	if subject == nil || subject.ImageKey == "" {
		http.NotFound(w, r)
		return
	}

	address, err := s.Signer.Sign(subject.ImageKey)
	if err != nil || address == "" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=300")
	http.Redirect(w, r, address, http.StatusFound)
}

// subject is what the page at a path is about, read at most once a minute.
func (s *Server) subject(clean string) *Subject {
	// A site for members describes none of its pages to anybody else.
	if policy.MembersOnly() || s.Reader == nil {
		return nil
	}

	now := time.Now
	if s.now != nil {
		now = s.now
	}

	s.mu.Lock()
	if found, ok := s.kept[clean]; ok && now().Before(found.until) {
		s.mu.Unlock()
		return found.subject
	}
	s.mu.Unlock()

	subject := Resolve(s.Reader, clean)

	s.mu.Lock()
	if s.kept == nil || len(s.kept) >= keptPages {
		s.kept = map[string]keptSubject{}
	}
	s.kept[clean] = keptSubject{subject: subject, until: now().Add(keptFor)}
	s.mu.Unlock()

	return subject
}

// base is the address of the site: the one it was told, or the one the
// request came to.
func (s *Server) base(r *http.Request) string {
	if s.PublicURL != "" {
		return s.PublicURL
	}

	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
