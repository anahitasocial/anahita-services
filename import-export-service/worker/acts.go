package worker

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/permissions"
	"anahita-libs/servicecalls"
	"anahita-libs/site"

	"importexport/data/repositories"
)

// GraphActs do what a line asks through the graph, as its person, by the
// rules socialgraph-service and text-service apply to the same thing done
// by hand (anahita-libs/permissions); who or what lives on another server
// is found and kept by activitypub-service or atproto-service, asked on
// their internal port (anahita-libs/servicecalls).
type GraphActs struct {
	Site      site.Addresses
	Calls     *servicecalls.Client
	Nodes     services.GraphNodeGRPCInterface
	Actors    services.ActorsGRPCInterface
	Relations services.SocialgraphGRPCInterface
	Follows   services.SocialgraphFollowGRPCInterface
	Blocks    services.SocialgraphBlockGRPCInterface
	Mutes     services.SocialgraphMuteGRPCInterface
	Media     services.MediaGRPCInterface
	// The hosts of the two services, as dialled: "activitypub-service" and
	// "atproto-service".
	ActivityPub string
	ATProto     string
}

// reasonOf is why a call to another service found nothing.
func reasonOf(err error) string {
	switch {
	case errors.Is(err, servicecalls.ErrNotFound):
		return ReasonNotFound
	case errors.Is(err, servicecalls.ErrRefused):
		return ReasonRefused
	case errors.Is(err, servicecalls.ErrNotFederating):
		return ReasonNotFederating
	}
	log.Println("import worker: finding:", err)
	return ReasonUnreachable
}

func (g *GraphActs) FindPerson(ctx context.Context, person *entities.Viewer, line *repositories.Line) (*entities.AnahitaNode, string) {
	user, domain := handleOf(line.Address)

	var (
		id  int64
		err error
	)
	switch {
	// Somebody here: by their username.
	case user != "" && domain == strings.ToLower(g.Site.Domain):
		node, readErr := g.Nodes.ReadByAlias(&services.GraphNodeReadByAlias{Alias: user, NodeType: constants.NodeActorPersonEntityType})
		if readErr != nil || node == nil || node.ID == 0 {
			return nil, ReasonNotFound
		}
		id = node.ID
	// On the fediverse, by their handle.
	case user != "":
		id, err = g.Calls.Person(ctx, g.ActivityPub, user+"@"+domain)
	// On Bluesky, by their handle.
	case domain != "":
		id, err = g.Calls.Person(ctx, g.ATProto, domain)
	// From a download that named only their ActivityPub id.
	case strings.HasPrefix(line.URI, "https://"):
		id, err = g.Calls.Person(ctx, g.ActivityPub, line.URI)
	default:
		return nil, ReasonNotFound
	}
	if err != nil {
		return nil, reasonOf(err)
	}

	node, err := g.Actors.Read(id, person)
	if err != nil || node == nil || node.ID == 0 {
		return nil, ReasonNotAllowed
	}
	return node, ""
}

func (g *GraphActs) FindPost(ctx context.Context, person *entities.Viewer, line *repositories.Line) (*entities.AnahitaNode, string) {
	uri := strings.TrimSpace(line.URI)

	var (
		id  int64
		err error
	)
	switch {
	// A post of this site's, by the id it has on the fediverse.
	case g.Site.IsOurs(uri):
		_, uuid, ok := strings.Cut(uri, "/ap/notes/")
		if !ok || uuid == "" {
			return nil, ReasonNotFound
		}
		node, readErr := g.Nodes.ReadByUUID(strings.Trim(uuid, "/"))
		if readErr != nil || node == nil || node.ID == 0 {
			return nil, ReasonNotFound
		}
		id = node.ID
	case strings.HasPrefix(uri, "at://"):
		id, err = g.Calls.Post(ctx, g.ATProto, uri)
	case strings.HasPrefix(uri, "https://"):
		id, err = g.Calls.Post(ctx, g.ActivityPub, uri)
	default:
		return nil, ReasonNotFound
	}
	if err != nil {
		return nil, reasonOf(err)
	}

	node, err := g.Nodes.Read(id)
	if err != nil || node == nil || node.ID == 0 {
		return nil, ReasonNotFound
	}
	return node, ""
}

// self is the person's own profile.
func (g *GraphActs) self(person *entities.Viewer) (*entities.AnahitaNode, error) {
	return g.Actors.ReadOfType(person.ID, constants.NodeActorPersonEntityType, person)
}

// Follow follows, or asks to follow somebody who approves their
// followers, as socialgraph-service's two routes do.
func (g *GraphActs) Follow(person *entities.Viewer, target *entities.AnahitaNode) (string, string) {
	source, err := g.self(person)
	if err != nil || source == nil {
		return repositories.LineFailed, ReasonFailed
	}
	theirs, err := g.Relations.Read(target.ID, person.ID)
	if err != nil || theirs == nil {
		return repositories.LineFailed, ReasonFailed
	}
	if theirs.IsLeading || theirs.HasRequested {
		return repositories.LineAlready, ""
	}
	mine, err := g.Relations.Read(person.ID, target.ID)
	if err != nil || mine == nil {
		return repositories.LineFailed, ReasonFailed
	}
	if theirs.IsBlocking || mine.IsBlocking {
		return repositories.LineFailed, ReasonBlocked
	}

	rules := &permissions.FollowContext{
		Viewer:          person,
		Source:          source,
		Target:          target,
		TargetViewerRel: theirs,
		TargetSourceRel: theirs,
	}
	asking := false
	switch {
	case permissions.CanFollow(rules):
	case permissions.CanRequestToFollow(rules):
		asking = true
	default:
		return repositories.LineFailed, ReasonNotAllowed
	}

	err = g.Follows.Add(&services.SocialgraphFollowAddRequest{
		Source:          source,
		Target:          target,
		Viewer:          person,
		IsFollowRequest: asking,
	})
	switch {
	case err == nil:
		return repositories.LineDone, ""
	case an_handlers.IsRulesNotAgreed(err):
		return repositories.LineFailed, ReasonRulesNotAgreed
	}
	log.Println("import worker: following:", err)
	return repositories.LineFailed, ReasonFailed
}

// Block blocks, as socialgraph-service does: not twice, and not somebody
// who blocks the person already.
func (g *GraphActs) Block(person *entities.Viewer, target *entities.AnahitaNode) (string, string) {
	source, err := g.self(person)
	if err != nil || source == nil {
		return repositories.LineFailed, ReasonFailed
	}
	mine, err := g.Relations.Read(person.ID, target.ID)
	if err != nil || mine == nil {
		return repositories.LineFailed, ReasonFailed
	}
	if mine.IsBlocking {
		return repositories.LineAlready, ""
	}
	theirs, err := g.Relations.Read(target.ID, person.ID)
	if err != nil || theirs == nil {
		return repositories.LineFailed, ReasonFailed
	}
	if theirs.IsBlocking {
		return repositories.LineFailed, ReasonBlocked
	}
	if err := g.Blocks.Add(source, target, person); err != nil {
		log.Println("import worker: blocking:", err)
		return repositories.LineFailed, ReasonFailed
	}
	return repositories.LineDone, ""
}

// Mute mutes a person, as socialgraph-service does: not somebody blocked
// either way, which hides more.
func (g *GraphActs) Mute(person *entities.Viewer, target *entities.AnahitaNode, hideNotifications bool, endsAt *time.Time) (string, string) {
	if !target.IsPerson() {
		return repositories.LineFailed, ReasonNotAllowed
	}
	source, err := g.self(person)
	if err != nil || source == nil {
		return repositories.LineFailed, ReasonFailed
	}
	mine, err := g.Relations.Read(person.ID, target.ID)
	if err != nil || mine == nil {
		return repositories.LineFailed, ReasonFailed
	}
	if mine.IsMuting {
		return repositories.LineAlready, ""
	}
	theirs, err := g.Relations.Read(target.ID, person.ID)
	if err != nil || theirs == nil {
		return repositories.LineFailed, ReasonFailed
	}
	if mine.IsBlocking || theirs.IsBlocking {
		return repositories.LineFailed, ReasonBlocked
	}
	var ends time.Time
	if endsAt != nil {
		ends = *endsAt
	}
	if err := g.Mutes.Add(source, target, person, hideNotifications, ends); err != nil {
		log.Println("import worker: muting:", err)
		return repositories.LineFailed, ReasonFailed
	}
	return repositories.LineDone, ""
}

// Save saves a post the person may see, as text-service does.
func (g *GraphActs) Save(person *entities.Viewer, post *entities.AnahitaNode) (string, string) {
	if !permissions.CanReadByID(g.Nodes, post.ID, person) {
		return repositories.LineFailed, ReasonNotAllowed
	}
	if err := g.Media.SetSaved(post.ID, true, person); err != nil {
		log.Println("import worker: saving:", err)
		return repositories.LineFailed, ReasonFailed
	}
	return repositories.LineDone, ""
}
