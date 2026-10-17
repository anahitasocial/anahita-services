package domains

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestASiteIsReadFromItsAddress(t *testing.T) {
	for _, c := range []struct {
		address, host, domain string
	}{
		{"https://example.com/blog/", "example.com", "example.com"},
		{"www.Example.com", "www.example.com", "example.com"},
		{"http://blog.example.org.", "blog.example.org", "blog.example.org"},
	} {
		site, err := SiteOf(c.address)
		if err != nil || site.Host != c.host || site.Domain != c.domain {
			t.Errorf("%q: %+v, %v; want %s and %s", c.address, site, err, c.host, c.domain)
		}
	}

	for _, address := range []string{"", "localhost", "http://192.0.2.1/", "intranet", "ftp://example.com", "a.localhost"} {
		if _, err := SiteOf(address); !errors.Is(err, ErrNotADomain) {
			t.Errorf("%q was taken as a domain", address)
		}
	}
}

func TestAFeedMustBeOnTheDomain(t *testing.T) {
	site, _ := SiteOf("https://www.example.com")
	for address, want := range map[string]bool{
		"https://example.com/feed":            true,
		"https://www.example.com/rss.xml":     true,
		"https://blog.example.com/atom":       true,
		"https://notexample.com/feed":         false,
		"https://feeds.example.net/x":         false,
		"https://example.com.evil.example/rs": false,
	} {
		if got := site.Covers(address); got != want {
			t.Errorf("%s: covered %v, want %v", address, got, want)
		}
	}
}

type records map[string][]string

func (r records) LookupTXT(_ context.Context, name string) ([]string, error) {
	if found, ok := r[name]; ok {
		return found, nil
	}
	return nil, errors.New("no such host")
}

// files answers every fetch from what it holds by address.
type files map[string]string

func (f files) RoundTrip(req *http.Request) (*http.Response, error) {
	body, ok := f[req.URL.String()]
	status := http.StatusOK
	if !ok {
		status = http.StatusNotFound
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
}

func TestTheTokenIsFoundInEitherPlace(t *testing.T) {
	site, _ := SiteOf("https://www.example.com")
	const token = "anahita-0123456789abcdef0123456789abcdef"

	for _, c := range []struct {
		name    string
		records records
		files   files
		want    string
		wantErr error
	}{
		{"a record", records{"_anahita.example.com": {"v=spf1", `"` + token + `"`}}, files{}, MethodDNS, nil},
		{"the file", records{}, files{"https://www.example.com/.well-known/anahita-verification.txt": "# Anahita\n" + token + "\n"}, MethodFile, nil},
		{"another token", records{"_anahita.example.com": {"anahita-other"}}, files{}, "", ErrNotFound},
		{"nowhere", records{}, files{}, "", ErrNotFound},
	} {
		checker := &Checker{Resolver: c.records, Client: &http.Client{Transport: c.files}}
		got, err := checker.Check(context.Background(), site, token)
		if got != c.want || !errors.Is(err, c.wantErr) {
			t.Errorf("%s: %q, %v; want %q, %v", c.name, got, err, c.want, c.wantErr)
		}
	}
}

func TestATokenSaysWhatItIs(t *testing.T) {
	token, err := NewToken()
	if err != nil || !strings.HasPrefix(token, "anahita-") || len(token) != len("anahita-")+32 {
		t.Fatalf("token %q, %v", token, err)
	}
	if other, _ := NewToken(); other == token {
		t.Error("two tokens were the same")
	}
}
