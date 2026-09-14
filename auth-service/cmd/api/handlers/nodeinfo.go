package handlers

import (
	"log"
	"net/http"
	"strings"

	"auth/data/repositories"

	"github.com/labstack/echo/v4"
)

// NodeInfo — how this installation describes itself to the rest of the
// fediverse, and to its own web app.
//
// http://nodeinfo.diaspora.software/protocol.html
//
// Two endpoints, and the two-step is the protocol rather than a design choice:
// a server asks /.well-known/nodeinfo, which is only a list of links, and
// follows the one whose `rel` it understands. That indirection is what lets a
// server publish 2.0 and 2.1 side by side, or move the document, without
// anybody having to agree on a path.
//
// The field that matters here is `openRegistrations`. It replaces
// REACT_APP_SIGNUP_CLOSED, which was an opinion held by the client: it hid a
// tab while POST /signups stayed open to anybody who looked. This is the
// server's own answer, it is the same answer the permission check gives, and
// every fediverse crawler already knows how to read it.
//
// Served by auth-service because registration is auth-service's business and
// because the other /.well-known documents are already here.

const (
	nodeInfoSchema21 = "http://nodeinfo.diaspora.software/ns/schema/2.1"

	// softwareName is what this installation runs. Lowercase and without
	// spaces by convention — crawlers group instances by this string.
	softwareName = "anahita"
)

// softwareVersion is reported to anybody who asks.
//
// A constant for now, and deliberately not derived from a build stamp: there
// is no release pipeline setting one, and a version that says "dev" on every
// production instance is worse than a version that is merely coarse.
var softwareVersion = "2.0.0"

type nodeInfoHandler struct {
	*Env
}

func newNodeInfoHandler(env *Env) *nodeInfoHandler {
	return &nodeInfoHandler{env}
}

type nodeInfoLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

type nodeInfoLinks struct {
	Links []nodeInfoLink `json:"links"`
}

type nodeInfoSoftware struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository,omitempty"`
}

type nodeInfoServices struct {
	// Both are required by the schema and both are empty: this installation
	// does not bridge to or from anything. They are `[]string{}` rather than
	// nil because the schema says array, and a JSON null there makes strict
	// consumers reject the whole document.
	Inbound  []string `json:"inbound"`
	Outbound []string `json:"outbound"`
}

type nodeInfoUsers struct {
	Total          int `json:"total"`
	ActiveMonth    int `json:"activeMonth"`
	ActiveHalfyear int `json:"activeHalfyear"`
}

type nodeInfoUsage struct {
	Users      nodeInfoUsers `json:"users"`
	LocalPosts int           `json:"localPosts"`
}

type nodeInfoDocument struct {
	Version           string           `json:"version"`
	Software          nodeInfoSoftware `json:"software"`
	Protocols         []string         `json:"protocols"`
	Services          nodeInfoServices `json:"services"`
	OpenRegistrations bool             `json:"openRegistrations"`
	Usage             nodeInfoUsage    `json:"usage"`
	Metadata          map[string]any   `json:"metadata"`
}

// Discovery returns the link document.
//
//	GET /.well-known/nodeinfo
func (h *nodeInfoHandler) Discovery(c echo.Context) error {
	base := strings.TrimRight(h.Config.GetAuthURL(), "/")

	return c.JSON(http.StatusOK, nodeInfoLinks{
		Links: []nodeInfoLink{
			{Rel: nodeInfoSchema21, Href: base + "/nodeinfo/2.1"},
		},
	})
}

// Read returns the NodeInfo 2.1 document.
//
//	GET /nodeinfo/2.1
func (h *nodeInfoHandler) Read(c echo.Context) error {
	// Usage is best-effort. A database that cannot answer should not take the
	// document down with it — a crawler that gets no NodeInfo at all may
	// conclude this is not a fediverse server, which is a worse and stickier
	// outcome than reporting zero members for one request.
	usage, err := h.Repos.Users.CountInstanceUsage()
	if err != nil {
		log.Println("nodeinfo: could not count usage:", err)
		usage = &repositories.InstanceUsage{}
	}

	return c.JSON(http.StatusOK, nodeInfoDocument{
		Version: "2.1",
		Software: nodeInfoSoftware{
			Name:       softwareName,
			Version:    softwareVersion,
			Repository: "https://github.com/anahitasocial/anahita",
		},
		// ActivityPub is where this is going and the protocol the plan is
		// designed against, but nothing federates yet. Claiming it before it
		// works would have remote servers attempting deliveries that fail.
		Protocols: []string{},
		Services: nodeInfoServices{
			Inbound:  []string{},
			Outbound: []string{},
		},
		// The whole reason this endpoint exists. `approval` reports TRUE:
		// registrations are open, they simply do not complete immediately.
		// Mastodon reports the same, and it is the honest answer to "can
		// somebody join?" — which is the question being asked.
		OpenRegistrations: h.Config.AllowsSelfServeRegistration(),
		Usage: nodeInfoUsage{
			Users: nodeInfoUsers{
				Total:          usage.Total,
				ActiveMonth:    usage.ActiveMonth,
				ActiveHalfyear: usage.ActiveHalfYear,
			},
			LocalPosts: usage.LocalPosts,
		},
		// Metadata is the schema's extension point, and where the detail
		// `openRegistrations` cannot carry belongs — a stranger seeing
		// `false` cannot tell a locked-down site from one that takes
		// invitations, and the web app needs to know which.
		Metadata: map[string]any{
			"nodeName":              h.Config.GetSiteName(),
			"registrationMode":      h.Config.GetRegistrationMode(),
			"approvalRequired":      h.Config.RequiresRegistrationApproval(),
			"invitationsAccepted":   h.Config.AllowsInvitations(),
			"termsOfServiceUrl":     h.Config.GetTosURL(),
			"privacyPolicyUrl":      h.Config.GetPrivacyURL(),
			"termsOfServiceVersion": h.Config.GetTosVersion(),
			"privacyPolicyVersion":  h.Config.GetPrivacyVersion(),

			// How to reach a person. Published here, rather than baked into
			// the web app as REACT_APP_ variables, for two reasons:
			//
			//   - a REACT_APP_ variable is compiled into the bundle, so
			//     changing the support address would mean rebuilding and
			//     redeploying the frontend. Here it is a restart;
			//   - SUPPORT_EMAIL is already the address the transactional
			//     emails use, and one setting cannot disagree with itself.
			//
			// Empty values are sent as empty rather than omitted, so a
			// client can tell "not configured" from "field unknown".
			"supportEmail":   h.Config.GetSupportEmail(),
			"supportPhone":   h.Config.GetSupportPhone(),
			"supportWebsite": h.Config.GetSupportWebsite(),
		},
	})
}
