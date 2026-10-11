package fetch

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Page is what a page says of itself in its head.
type Page struct {
	Title       string
	Description string
	SiteName    string
	// The picture's address, made whole against the page's own.
	Image string
}

// parse reads a page's head for what it is called, what it is about and its
// picture. Open Graph first, then what Twitter's cards used, then the
// page's own title and description. Reading stops where the head does.
func parse(body io.Reader, base *url.URL) *Page {
	found := map[string]string{}
	tokens := html.NewTokenizer(body)
	inTitle := false

	keep := func(key, value string) {
		value = strings.TrimSpace(value)
		if _, have := found[key]; !have && value != "" {
			found[key] = value
		}
	}

reading:
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			break reading

		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokens.Token()

			switch token.Data {
			case "title":
				inTitle = true
			case "body":
				break reading
			case "meta":
				var name, content string
				for _, attr := range token.Attr {
					switch strings.ToLower(attr.Key) {
					case "property", "name":
						if name == "" {
							name = strings.ToLower(strings.TrimSpace(attr.Val))
						}
					case "content":
						content = attr.Val
					}
				}
				if name != "" {
					keep(name, content)
				}
			}

		case html.TextToken:
			if inTitle {
				keep("title", string(tokens.Text()))
			}

		case html.EndTagToken:
			token := tokens.Token()
			if token.Data == "title" {
				inTitle = false
			}
			if token.Data == "head" {
				break reading
			}
		}
	}

	first := func(keys ...string) string {
		for _, key := range keys {
			if value := found[key]; value != "" {
				return value
			}
		}
		return ""
	}

	page := &Page{
		Title:       first("og:title", "twitter:title", "title"),
		Description: first("og:description", "twitter:description", "description"),
		SiteName:    first("og:site_name", "application-name"),
		Image:       first("og:image:secure_url", "og:image", "twitter:image", "twitter:image:src"),
	}

	if page.SiteName == "" && base != nil {
		page.SiteName = strings.TrimPrefix(base.Hostname(), "www.")
	}

	// The picture's address may be given relative to the page. Only a web
	// address is one.
	if page.Image != "" && base != nil {
		resolved, err := base.Parse(page.Image)
		if err != nil || (resolved.Scheme != "http" && resolved.Scheme != "https") {
			page.Image = ""
		} else {
			page.Image = resolved.String()
		}
	}

	return page
}
