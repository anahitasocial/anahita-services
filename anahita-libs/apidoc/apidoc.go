// Package apidoc describes a service's routes where they are registered, and
// makes an OpenAPI 3.1 document of them.
//
// A route is registered through a Router, which registers it with echo as
// before and keeps it, so that its path is written once:
//
//	api := doc.On(e)
//	hashtags := api.Group("/hashtags")
//	hashtags.GET("/:alias", r.handler.Hashtag.Read, r.setViewer).
//		Summary("One hashtag").
//		Viewer(apidoc.Anyone).
//		Takes(requests.NewHashtagRequest().Read).
//		Sends(200, responses.HashtagNodeItem{}).
//		Refuses(404, "there is no such hashtag")
//
// What a route takes and sends is read from the structs its handler binds and
// answers with: their tags say where each value comes from and what it may
// be, and the comment on each field, read from the source, is its
// description. Nothing is written twice.
//
// Check (check.go) is what keeps it true: every route registered has a
// description, and the document committed is the one the code makes now.
package apidoc

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/labstack/echo/v4"
)

// Access is who may call a route, before the route's own rules about what
// they may see or do.
type Access string

const (
	// Anyone, signed in or not. A token is read when there is one, and the
	// viewer still decides what is seen.
	Anyone Access = "anyone"
	// Somebody signed in.
	SignedIn Access = "signed_in"
	// A site administrator, or a super administrator.
	Admin Access = "admin"
	// A super administrator.
	SuperAdmin Access = "super_admin"
)

// Service is the description of one service's routes.
type Service struct {
	// The service's name, as its directory is named: "hashtag-service".
	Name string
	// What it is called, and what it is for, in a sentence or two.
	Title       string
	Description string

	routes []*Route
}

// NewService begins the description of a service.
func NewService(name, title, description string) *Service {
	return &Service{Name: name, Title: title, Description: description}
}

// Routes is every route described, in the order they were registered.
func (s *Service) Routes() []*Route {
	return s.routes
}

// Router registers routes with echo and keeps their descriptions.
type Router struct {
	service *Service
	echo    *echo.Echo
	group   *echo.Group
	prefix  string
}

// On is a Router over an echo instance.
func (s *Service) On(e *echo.Echo) *Router {
	return &Router{service: s, echo: e}
}

// Group is a Router for the routes under a prefix, as echo.Group is.
func (r *Router) Group(prefix string, m ...echo.MiddlewareFunc) *Router {
	group := &Router{service: r.service, echo: r.echo, prefix: r.prefix + prefix}
	if r.group != nil {
		group.group = r.group.Group(prefix, m...)
	} else {
		group.group = r.echo.Group(prefix, m...)
	}
	return group
}

func (r *Router) add(method, path string, h echo.HandlerFunc, m []echo.MiddlewareFunc) *Route {
	if r.group != nil {
		r.group.Add(method, path, h, m...)
	} else {
		r.echo.Add(method, path, h, m...)
	}
	route := &Route{Method: method, Path: normalizePath(r.prefix + path)}
	r.service.routes = append(r.service.routes, route)
	return route
}

func (r *Router) GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *Route {
	return r.add(http.MethodGet, path, h, m)
}

func (r *Router) POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *Route {
	return r.add(http.MethodPost, path, h, m)
}

func (r *Router) PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *Route {
	return r.add(http.MethodPut, path, h, m)
}

func (r *Router) PATCH(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *Route {
	return r.add(http.MethodPatch, path, h, m)
}

func (r *Router) DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *Route {
	return r.add(http.MethodDelete, path, h, m)
}

// normalizePath is a path as echo keeps it: beginning with a slash.
func normalizePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

// File is a file sent in a form, for a field of a struct that describes an
// upload, tagged `form:"avatar"` with the field's name. Echo binds no file
// into a struct, so such a struct is a description only; the handler reads
// the file with c.FormFile by the same name.
type File struct{}

// Route is the description of one route.
type Route struct {
	Method string
	// As echo has it: "/hashtags/:alias".
	Path string

	summary     string
	description string
	access      Access
	scope       string
	stepUp      bool
	takes       reflect.Value
	sends       []answer
	refuses     []answer
	notBuilt    string
	internal    string
	unbound     bool
	media       string
	body        reflect.Type
}

type answer struct {
	status int
	// What it says, when it is not a body: why it is sent.
	reason string
	body   reflect.Type
}

// With gives a route a description shared with others: two routes for one
// handler, or the same action on each kind of thing. The route is still
// registered on a line of its own, which the members-only guard
// (anahita-libs/middleware) reads.
func (r *Route) With(describe func(*Route)) *Route {
	describe(r)
	return r
}

// Summary is what the route does, in a few words: "One hashtag".
func (r *Route) Summary(text string) *Route {
	r.summary = text
	return r
}

// Describe says more than the summary can: what the route is for, and
// anything a caller would not guess.
func (r *Route) Describe(text string) *Route {
	r.description = text
	return r
}

// Viewer is who may call it.
func (r *Route) Viewer(access Access) *Route {
	r.access = access
	return r
}

// Scope is the OAuth scope the caller's token must carry (docs/oauth.md).
func (r *Route) Scope(scope string) *Route {
	r.scope = scope
	return r
}

// StepUp says the person must just have proved who they are again.
func (r *Route) StepUp() *Route {
	r.stepUp = true
	return r
}

// Takes is the struct the handler binds the request into, as the handler
// makes it: a value it sets before binding is the default.
func (r *Route) Takes(request any) *Route {
	r.takes = reflect.ValueOf(request)
	return r
}

// Sends is an answer with a body: what the handler answers with on
// `status`, and why, when it is not plain. A nil body is an answer with
// nothing in it.
func (r *Route) Sends(status int, body any, why ...string) *Route {
	a := answer{status: status, reason: strings.Join(why, "; ")}
	if body != nil {
		a.body = reflect.TypeOf(body)
	}
	r.sends = append(r.sends, a)
	return r
}

// SendsNothing is an answer with no body, and what it means.
func (r *Route) SendsNothing(status int, meaning string) *Route {
	r.sends = append(r.sends, answer{status: status, reason: meaning})
	return r
}

// Refuses is a refusal: its status, and why, in words.
func (r *Route) Refuses(status int, reason string) *Route {
	r.refuses = append(r.refuses, answer{status: status, reason: reason})
	return r
}

// Media is the type of the route's bodies, taken and sent, where it is not
// application/json: what other servers speak, `application/activity+json`.
func (r *Route) Media(mediaType string) *Route {
	r.media = mediaType
	return r
}

// TakesBody is a request whose whole body is one value, not the fields of
// a struct: what another server sends to an inbox.
func (r *Route) TakesBody(body any) *Route {
	r.body = reflect.TypeOf(body)
	return r
}

// Unbound says the struct given to Takes describes the request and is not
// bound: the handler reads each value itself, and refuses what it does not
// take as it says, so no 400 is assumed.
func (r *Route) Unbound() *Route {
	r.unbound = true
	return r
}

// Internal marks a route for the other services of the installation, which
// the gateway does not send anybody outside to, with what asks it.
func (r *Route) Internal(why string) *Route {
	r.internal = why
	return r
}

// NotBuilt marks a route that is registered and answers 501, with why.
func (r *Route) NotBuilt(why string) *Route {
	r.notBuilt = why
	return r
}
