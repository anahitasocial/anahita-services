package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"activitypub/ap"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// How another server finds somebody here from their handle.
//
// A person is written ana@example.org. A server that meets that asks
// example.org's /.well-known/webfinger who "acct:ana@example.org" is, and is
// told the address of their description. Only for somebody who has chosen
// to be on the fediverse: for anybody else the answer is that there is no
// such person, in the same words as for a name nobody has.

// WebFinger answers who a handle is.
func (h *Handlers) WebFinger(c echo.Context) error {
	// Any site's page may ask.
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")

	resource := c.QueryParam("resource")
	if resource == "" {
		return c.NoContent(http.StatusBadRequest)
	}

	// The installation itself, which signs what it asks of other servers
	// in nobody's name. A server that is shown its key asks who that is.
	if h.isInstance(resource) {
		if ok, _ := h.siteFederates(); !ok {
			return c.NoContent(http.StatusNotFound)
		}
		c.Response().Header().Set(echo.HeaderContentType, ap.JRDContentType)
		c.Response().Header().Set("Cache-Control", "public, max-age=300")
		return c.JSON(http.StatusOK, &ap.JRD{
			Subject: "acct:" + h.Site.Handle(h.Site.Domain),
			Aliases: []string{h.addresses.Instance()},
			Links: []ap.JRDLink{
				{Rel: "self", Type: "application/activity+json", Href: h.addresses.Instance()},
			},
		})
	}

	person := h.federating(h.resolve(resource))
	if person == nil {
		return c.NoContent(http.StatusNotFound)
	}

	actor := h.addresses.Actor(person.UUID)
	profile := h.addresses.Profile(person.Alias)

	c.Response().Header().Set(echo.HeaderContentType, ap.JRDContentType)
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return c.JSON(http.StatusOK, &ap.JRD{
		Subject: "acct:" + h.Site.Handle(person.Alias),
		Aliases: []string{actor, profile},
		Links: []ap.JRDLink{
			{Rel: "self", Type: "application/activity+json", Href: actor},
			{Rel: "http://webfinger.net/rel/profile-page", Type: "text/html", Href: profile},
		},
	})
}

// resolve finds who a WebFinger resource names: a handle, or one of the two
// addresses a person here has. Nil when it names nobody here.
func (h *Handlers) resolve(resource string) *entities.AnahitaNode {
	switch {
	case strings.HasPrefix(resource, "acct:"):
		handle := strings.TrimPrefix(strings.TrimPrefix(resource, "acct:"), "@")
		alias, domain, found := strings.Cut(handle, "@")
		if !found || !h.isOurDomain(domain) {
			return nil
		}
		return h.personByAlias(alias)

	case strings.HasPrefix(resource, "https://"):
		if uuid := h.addresses.UUIDOf(resource); uuid != "" {
			return h.personByUUID(uuid)
		}
		// Their page in the web app.
		parsed, err := url.Parse(resource)
		if err != nil || !h.Site.IsOurs(resource) {
			return nil
		}
		if alias, ok := strings.CutPrefix(strings.TrimSuffix(parsed.Path, "/"), "/people/"); ok && !strings.Contains(alias, "/") {
			return h.personByAlias(alias)
		}
	}
	return nil
}

// isOurDomain says whether the part of a handle after the @ is this
// installation: the domain people know it by, or the host its API is on.
//
// The second because of how a server checks somebody it met by their
// address and not by their handle: it asks the host in the address who
// "name@that-host" is, and expects to be told their real handle. Here that
// host is the API's, which is not the handle's.
func (h *Handlers) isOurDomain(domain string) bool {
	if strings.EqualFold(domain, h.Site.Domain) {
		return true
	}
	api, err := url.Parse(h.Site.APIURL)
	return err == nil && api.Host != "" && strings.EqualFold(domain, api.Host)
}

// isInstance says whether a WebFinger resource names the installation
// itself: by its address, or by the handle it goes by, which is its domain
// twice over (example.org@example.org), as Mastodon's own is.
func (h *Handlers) isInstance(resource string) bool {
	if resource == h.addresses.Instance() {
		return true
	}
	handle, ok := strings.CutPrefix(resource, "acct:")
	if !ok {
		return false
	}
	name, domain, found := strings.Cut(strings.TrimPrefix(handle, "@"), "@")
	return found && strings.EqualFold(name, h.Site.Domain) && h.isOurDomain(domain)
}

func (h *Handlers) personByAlias(alias string) *entities.AnahitaNode {
	// What a username here can be, and nothing else is looked up.
	if alias == "" || len(alias) > 100 || strings.ContainsAny(alias, "@./ %") {
		return nil
	}

	req := services.NewGraphNodeRequest().ReadByAlias
	req.Alias = alias
	req.NodeType = constants.NodeActorPersonEntityType
	person, err := h.Nodes.ReadByAlias(req)
	if err != nil || person == nil || person.ID == 0 {
		return nil
	}
	return person
}

func (h *Handlers) personByUUID(uuid string) *entities.AnahitaNode {
	if !uuidShape.MatchString(uuid) {
		return nil
	}
	person, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || person == nil || person.ID == 0 {
		return nil
	}
	return person
}

// HostMeta is the older way of being asked where WebFinger is. Some servers
// still ask it first.
func (h *Handlers) HostMeta(c echo.Context) error {
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")

	template := h.Site.API("/.well-known/webfinger") + "?resource={uri}"
	return c.Blob(http.StatusOK, "application/xrd+xml; charset=utf-8", []byte(
		`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
			`<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0">`+"\n"+
			`  <Link rel="lrdd" template="`+xmlEscape(template)+`"/>`+"\n"+
			`</XRD>`+"\n",
	))
}

func xmlEscape(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(value)
}
