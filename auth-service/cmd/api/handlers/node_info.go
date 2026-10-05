package handlers

import (
	"anahita-libs/policy"
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

	// softwareRepository is where the software this instance runs lives.
	//
	// Anahita Services, not the PHP Anahita it replaces: this said
	// anahitasocial/anahita, which sent anybody following it to a
	// different codebase — the same mistake the 2.0.0 version made, and
	// with the same cause, a NodeInfo document describing the project
	// rather than the software.
	//
	// Points at anahitasocial/ rather than the purplerat/ remote it is
	// pushed to today, because this is where it will live once it is
	// public and NodeInfo is read by strangers, for whom a private URL
	// and a wrong URL fail identically. Worth knowing that until that
	// move happens, this link 404s for everyone outside the org.
	//
	// A constant rather than config: every instance runs the same
	// software from the same place, so it is a property of the build
	// and not of the installation.
	softwareRepository = "https://github.com/anahitasocial/anahita-services"
)

// softwareVersion is reported to anybody who asks.
//
// A constant for now, and deliberately not derived from a build stamp: there
// is no release pipeline setting one, and a version that says "dev" on every
// production instance is worse than a version that is merely coarse.
//
// 1.0.0 because Anahita Services is a rebuild rather than a continuation.
// It said 2.0.0, read as the next major after the PHP implementation — but
// that numbering claims a lineage the code does not have: nothing here is
// descended from that codebase, the storage is different, and an instance
// running this shares no upgrade path with one running that. A crawler
// comparing versions across instances of "anahita" would otherwise be
// ordering two unrelated things on one scale.
//
// Note this is NOT the only version in the repository: every service
// carries its own in metadata.yml. Those are per-service numbers and this
// is what the installation tells the fediverse, so they are allowed to
// differ — but they should not differ by accident, which is why they were
// brought to 1.0.0 alongside this rather than left at 0.9.0.
//
// The k8s manifests pin image TAGS, which are a third thing again: they
// name what has actually been pushed to a registry, so they follow a
// release rather than an edit here.
var softwareVersion = "1.0.0"

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
			Repository: softwareRepository,
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
			// nodeName and nodeDescription keep the ecosystem's spelling
			// even though the settings behind them are ANAHITA_INSTANCE_*.
			// These two keys are what other fediverse software reads, so
			// the wire format follows the convention while the variable
			// name follows whatever is clearest for an operator here.
			// They do not have to agree, and making them agree would
			// mean picking one audience to confuse.
			"nodeName":            h.Config.GetInstanceName(),
			"nodeDescription":     h.Config.GetInstanceDescription(),
			"registrationMode":    h.Config.GetRegistrationMode(),
			"approvalRequired":    h.Config.RequiresRegistrationApproval(),
			"invitationsAccepted": h.Config.AllowsInvitations(),

			// The minimum role that may issue an invitation:
			// registered, administrators or super-administrators.
			//
			// Published because the web app cannot work it out. Whether
			// to offer somebody the invitations screen depends on a
			// server setting, and without this the client's only
			// options are to guess from the person's role — which is
			// what it did, assuming administrator — or to show
			// everybody a page most of them cannot use.
			//
			// Normalised, so the deprecated `members` spelling never
			// reaches a client. Discloses nothing: it is a policy of
			// the site, the same kind of fact as registrationMode
			// sitting two lines above.
			"invitesFrom": h.Config.GetInvitesFrom(),

			// The minimum role that may create a group, for the same reason
			// invitesFrom is here: the web app decides whether to offer a +
			// on the groups page, and cannot work that out from the person
			// alone. GROUP-SERVICE enforces it; this document only reports
			// it, so a client reading it is reading a policy, not a gate.
			"groupsFrom": h.Config.GetGroupsFrom(),

			// The widest audience anything new may be given: "public", or
			// "registered" on an installation that allows no new public
			// content (MIN_CONTENT_ACCESS). The services refuse a public
			// post or profile there; this is so the web app does not
			// offer one. Each profile and post also says which audiences
			// it may have, which already leaves "public" out, so a
			// client needs this only for menus that have no such list.
			"minContentAccess": policy.MinContentAccess(),

			// What somebody who is not signed in may read (SITE_READ_ACCESS):
			// "public"; "preview", the start of what is public; or
			// "registered", nothing, on a members-only installation. There
			// every content route answers 401 to them, and this document
			// is one of the few things that still answers, so the web app
			// can show a way in instead of a page of errors.
			"readAccess": policy.SiteReadAccess(),

			"termsOfServiceUrl": h.Config.GetTosURL(),
			"privacyPolicyUrl":  h.Config.GetPrivacyURL(),

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
