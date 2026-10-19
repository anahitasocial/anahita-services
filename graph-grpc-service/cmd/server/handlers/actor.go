package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/proto/graphpb"
	"context"
	"errors"
	"fmt"
	"graph/requests"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	features "graph/data/actor_features"
	"graph/data/repositories"
	"graph/data/validators"
	"graph/sweeper"

	"github.com/google/uuid"
)

type actorHandler struct {
	graphpb.ActorServiceServer
	env *Env
}

func NewActorHandler(env *Env) *actorHandler {
	return &actorHandler{env: env}
}

// Count return the total number of actors
func (h *actorHandler) Count(ctx context.Context, req *graphpb.ActorCountRequest) (*graphpb.ActorCountResponse, error) {
	log.Printf("Received Count request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().Count
	rReq.Viewer = viewer
	rReq.Types = req.Types
	rReq.Search = req.Search
	rReq.AccessCheck = req.AccessCheck
	rReq.Featured = req.Featured
	rReq.IncludeAskable = req.IncludeAskable

	total, err := h.env.Repos.Nodes.Count(rReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.ActorCountResponse{
		Total: total,
	}, nil
}

// Browse return a list of actors
func (h *actorHandler) Browse(ctx context.Context, req *graphpb.ActorBrowseRequest) (*graphpb.ActorBrowseResponse, error) {
	log.Printf("Received Browse request: %v", req)
	var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().Browse
	rReq.Viewer = viewer
	rReq.Types = req.Types
	rReq.Search = req.Search
	rReq.Offset = req.Offset
	rReq.Limit = req.Limit
	rReq.Ordering = req.Ordering
	rReq.OrderingDir = req.Direction
	rReq.AccessCheck = req.AccessCheck
	rReq.Featured = req.Featured
	rReq.IncludeAskable = req.IncludeAskable

	actors, err := h.env.Repos.Nodes.Browse(rReq)
	if err != nil {
		return nil, err
	}

	// The list was widened to the actors the viewer may only ask to
	// follow. Those are cut down to a name and a picture here, before they
	// leave the graph, so no caller can send on what it should not.
	if req.IncludeAskable {
		if actors, err = h.restrictUnseen(actors, viewer); err != nil {
			return nil, err
		}
	}

	targetIds := extractNodeIDs(actors)
	subscriptionStatuses, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	for _, actor := range actors {
		actor.IsSubscribedByViewer = subscriptionStatuses[actor.ID].IsSubscribed
	}

	return &graphpb.ActorBrowseResponse{
		Actors: nodesToGrpc(actors),
	}, nil
}

// Read return a single actor
func (h *actorHandler) Read(ctx context.Context, req *graphpb.ActorReadRequest) (*graphpb.ActorReadResponse, error) {
	log.Printf("Received Get request: %v", req)

	loader := features.NewFeatureLoader(h.env.Config.GetActorFeaturesPath())

	// Actors only, and the caller's exact type when it supplied one.
	//
	// A bare id lookup returned whatever node held that id, so
	// GET /groups/<hashtag id> answered 200 with a hashtag body. The family
	// filter stops that. The exact type stops the subtler version: a person
	// and a group are both actors, so /groups/<a person id> still returned a
	// person until the route's own type travelled with the request.
	//
	// Falling back to the family when node_type is empty keeps the internal
	// callers that genuinely take any actor working unchanged.
	nodeType := req.NodeType
	if nodeType == "" {
		nodeType = constants.NodeActorPrefixEntityType
	}

	node, err := h.env.Repos.Nodes.ReadOfType(req.Id, nodeType)
	if err != nil {
		return nil, err
	}

	actor := nodeToGrpc(node)
	defaults, err := loader.Load(actor.Type)
	if err != nil {
		log.Printf("Failed to load default features for %s: %v", actor.Type, err)
	} else {
		_ = features.ApplyFeatures(actor, loader, defaults)
	}

	if req.Viewer != nil {
		actor.IsSubscribedByViewer, err = h.env.Repos.SubscriptionsRel.IsSubscribedToOne(req.Viewer.Id, actor.Id)
		if err != nil {
			return nil, err
		}
	}

	return &graphpb.ActorReadResponse{
		Actor: actor,
	}, nil
}

// BatchRead return a list of actors
func (h *actorHandler) BatchRead(ctx context.Context, req *graphpb.ActorBatchReadRequest) (*graphpb.ActorBatchReadResponse, error) {
	log.Printf("Received BatchRead request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().BatchRead
	rReq.IDs = req.Ids
	rReq.Ordering = req.Ordering
	rReq.OrderingDir = req.Direction

	result, err := h.env.Repos.Nodes.BatchRead(rReq)
	if err != nil {
		return nil, err
	}

	targetIds := extractNodeIDs(result)
	subscriptionStatuses, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	for _, actor := range result {
		actor.IsSubscribedByViewer = subscriptionStatuses[actor.ID].IsSubscribed
	}

	actors := nodesToGrpc(result)

	// The actor_features defaults, merged in exactly as Read does. Without
	// them an actor read in a batch carried only its raw features column —
	// NULL for most — and every permission read from it said nobody may add
	// anything. feed-service reads post owners this way to answer who may
	// comment and like.
	loader := features.NewFeatureLoader(h.env.Config.GetActorFeaturesPath())
	for _, actor := range actors {
		defaults, err := loader.Load(actor.Type)
		if err != nil {
			log.Printf("Failed to load default features for %s: %v", actor.Type, err)
			continue
		}
		_ = features.ApplyFeatures(actor, loader, defaults)
	}

	return &graphpb.ActorBatchReadResponse{
		Actors: actors,
	}, nil
}

// Edit update an actor
func (h *actorHandler) Edit(ctx context.Context, req *graphpb.ActorEditRequest) (*graphpb.ActorEditResponse, error) {
	log.Printf("Received Edit request: %v", req)

	node, err := h.env.Repos.Nodes.Read(req.Actor.Id)
	if err != nil {
		return nil, err
	}

	applyEditableActorFields(node, req.Actor)

	v := validators.NewActorValidator()
	if err := v.ValidateEdit(node); err != nil {
		return nil, err
	}

	result, err := h.env.Repos.Nodes.Edit(node)
	if err != nil {
		return nil, err
	}

	actor := nodeToGrpc(result)

	if actor.Type != "" {
		loader := features.NewFeatureLoader(h.env.Config.GetActorFeaturesPath())
		defaults, err := loader.Load(actor.Type)
		if err != nil {
			log.Printf("Failed to load default features for %s: %v", actor.Type, err)
		} else {
			_ = features.ApplyFeatures(actor, loader, defaults)
		}
	}

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	actor.IsSubscribedByViewer, err = h.env.Repos.SubscriptionsRel.IsSubscribedToOne(viewer.ID, actor.Id)
	if err != nil {
		return nil, err
	}

	// publish event
	go func() {
		entityName := strings.Split(actor.Type, ".")[3]
		srvName := strings.Split(actor.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatNodeActorEdited, srvName, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    actor.Service,
			SubjectID:        req.Viewer.Id,
			ObjectID:         actor.Id,
			TargetID:         actor.Id,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		log.Println("Publishing event", routing_key)
		if err = h.env.Publisher.Publish(payload, routing_key); err != nil {
			log.Println(err)
		}
	}()

	return &graphpb.ActorEditResponse{
		Actor: actor,
	}, nil
}

// Add create a new actor
func (h *actorHandler) Add(ctx context.Context, req *graphpb.ActorAddRequest) (*graphpb.ActorAddResponse, error) {
	log.Printf("Received Add request: %v", req)

	node := newActorFromRequest(req.Actor)

	v := validators.NewActorValidator()
	if err := v.ValidateAdd(node); err != nil {
		return nil, err
	}

	result, err := h.env.Repos.Nodes.Add(node)
	if err != nil {
		return nil, err
	}

	actor := nodeToGrpc(result)

	if actor.Type != "" {
		loader := features.NewFeatureLoader(h.env.Config.GetActorFeaturesPath())
		defaults, err := loader.Load(actor.Type)
		if err != nil {
			log.Printf("Failed to load default features for %s: %v", actor.Type, err)
		} else {
			_ = features.ApplyFeatures(actor, loader, defaults)
		}
	}

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType

	// publish event
	go func() {
		entityName := strings.Split(actor.Type, ".")[3]
		srvName := strings.Split(actor.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatNodeActorAdded, srvName, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    actor.Service,
			SubjectID:        req.Viewer.Id,
			ObjectID:         actor.Id,
			TargetID:         actor.Id,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.env.Publisher.Publish(payload, routing_key)
	}()

	return &graphpb.ActorAddResponse{
		Actor: actor,
	}, nil
}

// actorLifecycleRoutingKey builds the routing key for an actor transition.
//
// Separated out because it is the part that can be quietly wrong. A key with
// the wrong name does not fail — it publishes to a topic nothing is listening
// on, which looks exactly like a working publish from the emitting side and
// exactly like no event at all from the consuming side.
//
// The shape is node.<type>.<service>.<entity>.<action>.<version>, and both
// middle parts are short names: "node.actor.person-service.person.v1" gives
// the entity "person", and the service "person-service" gives "person". That
// is the convention Add already follows and the one person-service hardcoded
// before this took the job over.
//
// Reports false rather than guessing when the type is not shaped like an actor
// type. A malformed key is worse than none: no event is at least visible as an
// absence, while a wrong one is a message nobody receives and nobody misses.
func actorLifecycleRoutingKey(format, nodeType, service string) (string, bool) {
	parts := strings.Split(nodeType, ".")
	if len(parts) < 4 || parts[3] == "" || service == "" {
		return "", false
	}

	entityName := parts[3]
	srvName := strings.Split(service, "-")[0]
	if srvName == "" {
		return "", false
	}

	return fmt.Sprintf(format, srvName, entityName), true
}

// publishActorLifecycle emits a lifecycle transition for an actor.
//
// From HERE rather than from each calling service, because this is where the
// state actually changes. person-service published its own "deleted" event and
// group-service published nothing, so deleting a person notified consumers and
// deleting a group silently did not — an asymmetry nobody chose, and one that
// would have grown a third case with the next actor type.
//
// The routing key is derived from the node rather than passed in, for the same
// reason: a caller that hardcodes its own names is a caller that can disagree
// with the others.
//
// Best-effort and asynchronous. The transition has already committed, and a
// broker that is down is not a reason to tell the caller their group is still
// there.
func (h *actorHandler) publishActorLifecycle(format string, node *entities.AnahitaNode, viewerID int64, viewerType string) {
	routingKey, ok := actorLifecycleRoutingKey(format, node.Type, node.Service)
	if !ok {
		log.Printf("actor lifecycle: cannot derive a routing key from type %q "+
			"service %q, not publishing", node.Type, node.Service)
		return
	}

	payload := &event.EventPayload{
		Name:             routingKey,
		SourceService:    node.Service,
		SubjectID:        viewerID,
		ObjectID:         node.ID,
		TargetID:         node.ID,
		ViewerID:         viewerID,
		ViewerPersonType: viewerType,
	}

	go func() {
		if err := h.env.Publisher.Publish(payload, routingKey); err != nil {
			log.Println("actor lifecycle: publish failed:", err)
		}
	}()
}

// Delete remove a medium
// Delete SOFT-deletes an actor: it is marked and disabled, and nothing is
// destroyed.
//
// This used to call PurgeByOwner, which hard-deleted the actor, everything it
// owned, every descendant of those by parent_id — other people's comments on
// the actor's posts included — and every edge touching any of them. There was
// no way back from a misclick and no record that it had happened.
//
// PurgeByOwner still exists and still does exactly that, but it now runs from
// the sweeper once the grace period has expired, not from a request.
func (h *actorHandler) Delete(ctx context.Context, req *graphpb.ActorDeleteRequest) (*graphpb.ActorDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req.Id)
	actor, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	deleted, err := h.env.Repos.Nodes.SoftDeleteActor(actor.ID)
	if err != nil {
		return nil, err
	}

	// Already deleted. Not an error — a repeated request should not restart the
	// grace period, and the caller wanted the account gone, which it is. No
	// audit row either: nothing happened this time.
	if !deleted {
		log.Printf("actor %d was already marked deleted", actor.ID)
		return &graphpb.ActorDeleteResponse{}, nil
	}

	// Who deleted what, and when. Best-effort: the deletion has already
	// committed, and failing the request over the audit row would leave the
	// caller believing nothing happened when it did.
	//
	// req.ViewerId equals the actor for a self-service deletion and differs for
	// an administrator removing somebody else — which is the case this row
	// exists to answer for.
	var deletedBy int64
	if req.Viewer != nil {
		deletedBy = req.Viewer.Id
	}

	if err := h.env.Repos.NodeDeletionLogs.Add(
		actor.ID, actor.Alias, actor.Type, deletedBy,
		repositories.DeletionActionScheduled,
	); err != nil {
		log.Println("actor delete: audit write failed:", err)
	}

	// After the "already deleted" return above, so a repeated request does not
	// re-announce a deletion that happened days ago.
	var viewerType string
	if req.Viewer != nil {
		viewerType = req.Viewer.PersonType
	}
	h.publishActorLifecycle(
		event.EventFormatNodeActorDeleted, actor, deletedBy, viewerType)

	return &graphpb.ActorDeleteResponse{}, nil
}

// Restore reverses a Delete, within the grace period.
//
// People restore themselves by signing in — the password is the proof, and the
// login page offers it. A group cannot sign in, so its administrators restore
// it from the group's own settings page, and this is what that calls.
//
// The permission check lives in group-service, which knows who administers the
// group; this service knows only nodes. Restoring is gated on the same
// permission as deleting, on the reasoning that undoing an action should never
// need more authority than doing it.
func (h *actorHandler) Restore(ctx context.Context, req *graphpb.ActorRestoreRequest) (*graphpb.ActorRestoreResponse, error) {
	log.Printf("Received Restore request: %v", req.Id)

	restored, err := h.env.Repos.Nodes.RestoreActor(req.Id)
	if err != nil {
		return nil, err
	}

	if !restored {
		// Already purged, or never deleted. Not an error — the caller needs to
		// tell it from success, which the response field carries.
		log.Printf("actor %d had nothing to restore", req.Id)
		return &graphpb.ActorRestoreResponse{Restored: false}, nil
	}

	var restoredBy int64
	if req.Viewer != nil {
		restoredBy = req.Viewer.Id
	}

	node, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		// Restored, but we cannot describe what. Log rather than fail: the
		// group is back, which is what was asked for.
		log.Println("actor restore: audit read failed:", err)
		return &graphpb.ActorRestoreResponse{Restored: true}, nil
	}

	if err := h.env.Repos.NodeDeletionLogs.Add(
		node.ID, node.Alias, node.Type, restoredBy,
		repositories.DeletionActionRestored,
	); err != nil {
		log.Println("actor restore: audit write failed:", err)
	}

	// Only on a real restore — the "nothing to restore" branch returned above.
	// A consumer that revoked something on the way down needs this to put it
	// back; without it the only signal is a poll.
	var restoreViewerType string
	if req.Viewer != nil {
		restoreViewerType = req.Viewer.PersonType
	}
	h.publishActorLifecycle(
		event.EventFormatNodeActorRestored, node, restoredBy, restoreViewerType)

	return &graphpb.ActorRestoreResponse{Restored: true}, nil
}

// DeletionCounts reports what deleting this actor would cost.
//
// Read-only and gathered before the fact — the Danger zone shows these to
// somebody deciding, which is the only moment they are useful.
//
// A missing actor yields zeroes rather than an error. The dialog can render
// "0 posts" harmlessly, while an error would leave it with nothing to show at
// the moment it most needs to say something.
func (h *actorHandler) DeletionCounts(ctx context.Context, req *graphpb.ActorDeletionCountsRequest) (*graphpb.ActorDeletionCountsResponse, error) {
	counts, err := h.env.Repos.Nodes.ReadDeletionCounts(req.Id)
	if err != nil {
		return nil, err
	}

	resp := &graphpb.ActorDeletionCountsResponse{
		Posts:              counts.Posts,
		Replies:            counts.Replies,
		Followers:          counts.Followers,
		Following:          counts.Following,
		Admins:             counts.Admins,
		GroupsAdministered: counts.GroupsAdministered,
	}

	// Zero rather than the epoch when unknown, so the client can tell "we do
	// not know when they joined" from "they joined in 1970".
	if !counts.MemberSince.IsZero() {
		resp.MemberSinceUnix = counts.MemberSince.Unix()
	}

	return resp, nil
}

// The reasons RequestPurge gives for refusing. They cross to the client as
// they are, so they are part of the API.
const (
	purgeRefusedNotFound       = "not_found"
	purgeRefusedWrongType      = "wrong_type"
	purgeRefusedAdministrator  = "administrator"
	purgeRefusedYourself       = "yourself"
	purgeRefusedLastSuperAdmin = "last_super_administrator"
	// Removed as spam, but here longer than SPAM_REMOVAL_MAX_DAYS: disabled
	// or deleted instead, with the thirty days to bring it back.
	purgeRefusedTooOld = "too_old"
)

// RequestPurge queues actors to be destroyed, with everything they owned or
// wrote, and starts the work.
//
// The hard delete. No grace period, no restore, nothing to undo — which is why
// it is a separate RPC rather than a flag on Delete. A boolean on the ordinary
// deletion would mean the difference between "recoverable for thirty days" and
// "gone" was one field somebody could set by mistake, or a client could set by
// default.
//
// AUTHORITY IS THE CALLER'S TO CHECK. graph-grpc has no notion of who may do
// this, so person-service and group-service enforce super-administrator only,
// the same way they enforce every other permission before calling here.
//
// ALL OR NOTHING. Every actor is checked before any is touched, and one that
// cannot be purged refuses the whole request, with the reason for each. A
// cleanup of five hundred accounts that quietly left three behind would have
// to be found by reading the result; one that stops says so.
//
// IT ANSWERS BEFORE THE WORK IS DONE. Each actor is marked deleted, so it is
// gone from the site at once, and queued. The purge then runs in the
// background under the sweeper's lock, and ReadPurgeBatch says how far it has
// got. A purge used to run inside this call, so a large account could outlast
// the caller's patience and the gateway's.
//
// It notifies nobody. The purged event exists so auth-service removes the
// credentials, auth logs and pending tokens it still holds; it is plumbing,
// not a message to a person. The deleted event is NOT published either: it
// tells the person their account can be restored for thirty days, which here
// would be untrue.
//
// What survives: the tombstone, which reserves the handle so a later account
// cannot inherit this one's identity on a remote server, and the deletion log
// rows recording who did it. Both are deliberate — see docs/plans.
func (h *actorHandler) RequestPurge(ctx context.Context, req *graphpb.ActorPurgeBatchRequest) (*graphpb.ActorPurgeBatchResponse, error) {
	log.Printf("Received RequestPurge request for %d actor(s)", len(req.Ids))

	var requestedBy int64
	if req.Viewer != nil {
		requestedBy = req.Viewer.Id
	}

	response := &graphpb.ActorPurgeBatchResponse{}

	// Pass one: look, and touch nothing.
	var actors []*entities.AnahitaNode
	seen := map[int64]bool{}

	for _, id := range req.Ids {
		if seen[id] {
			continue
		}
		seen[id] = true

		actor, reason := h.purgeable(id, req, requestedBy)
		if reason != "" {
			response.Refused = append(response.Refused,
				&graphpb.ActorPurgeRefusal{Id: id, Reason: reason})
			continue
		}

		// Already a tombstone. The caller wanted it gone and it is.
		purged, err := h.env.Repos.Nodes.IsPurged(id)
		if err != nil {
			return nil, err
		}
		if purged {
			response.Skipped++
			continue
		}

		actors = append(actors, actor)
	}

	if len(response.Refused) > 0 {
		log.Printf("actor purge: refused, %d of %d cannot be purged",
			len(response.Refused), len(req.Ids))
		response.Skipped = 0
		return response, nil
	}

	// Pass two: hide and queue.
	batchID := uuid.NewString()

	for _, actor := range actors {
		// Marked deleted now, so the account is gone from the site before
		// this call returns, whatever the queue is doing.
		if _, err := h.env.Repos.Nodes.SoftDeleteActor(actor.ID); err != nil {
			if errors.Is(err, repositories.ErrLastSuperAdmin) {
				// Pass one let it through because another super administrator
				// was there then. Only possible when the request itself named
				// the others.
				return nil, status.Error(codes.FailedPrecondition, err.Error())
			}
			return nil, err
		}

		queued, err := h.env.Repos.PurgeRequests.Add(batchID, actor, requestedBy)
		if err != nil {
			return nil, err
		}
		// Removed as spam: why, and who decided, recorded now; the purge
		// writes its own rows when it is done.
		if queued && req.Reason == constants.PurgeReasonSpam {
			if err := h.env.Repos.NodeDeletionLogs.Add(
				actor.ID, actor.Alias, actor.Type, requestedBy, repositories.DeletionActionRemovedAsSpam,
			); err != nil {
				log.Printf("actor purge: recording %d as removed as spam failed: %v", actor.ID, err)
			}
		}
		if queued {
			response.Queued++
		} else {
			response.Skipped++
		}
	}

	if response.Queued > 0 {
		response.BatchId = batchID

		// One pass over the queue, in the background, under the sweeper's
		// lock. If this replica stops before it finishes, the rows are still
		// queued and the scheduled sweeper takes them.
		go func() {
			purged, err := sweeper.RunRequests(h.env.DB, h.env.Repos, h.env.Publisher, h.env.Files)
			if err != nil {
				log.Println("actor purge:", err)
				return
			}
			log.Printf("actor purge: %d account(s) purged on request", purged)
		}()
	}

	return response, nil
}

// purgeable reads one actor and says why it may not be purged, or returns it.
func (h *actorHandler) purgeable(id int64, req *graphpb.ActorPurgeBatchRequest, requestedBy int64) (*entities.AnahitaNode, string) {
	actor, err := h.env.Repos.Nodes.Read(id)
	if err != nil || actor == nil {
		return nil, purgeRefusedNotFound
	}

	// Actors only, and only the kind the route is for. A purge follows
	// owner_id and created_by from this node, so pointing it at a post would
	// destroy that post's comments and leave the actor untouched.
	if !strings.HasPrefix(actor.Type, constants.NodeActorPrefixEntityType) ||
		(req.Type != "" && actor.Type != req.Type) {
		return nil, purgeRefusedWrongType
	}

	if id == requestedBy {
		return nil, purgeRefusedYourself
	}

	administrator := actor.PersonType == constants.PersonTypeAdmin ||
		actor.PersonType == constants.PersonTypeSuperAdmin
	if administrator && !req.AllowAdministrators {
		return nil, purgeRefusedAdministrator
	}

	if err := h.env.Repos.Nodes.GuardLastSuperAdmin(id); err != nil {
		return nil, purgeRefusedLastSuperAdmin
	}

	// Spam is new (the user, 2026-10-19): an account here longer than the
	// installation allows is not removed this way, which has no way back.
	if req.Reason == constants.PurgeReasonSpam && tooOldForSpam(actor.CreatedAt, time.Now(), constants.SpamRemovalMaxDays()) {
		return nil, purgeRefusedTooOld
	}

	return actor, ""
}

// tooOldForSpam says whether an account made at `made` has been here longer
// than `days` at `now`, and so is not removed as spam. One with no date is
// not held to it.
func tooOldForSpam(made, now time.Time, days int) bool {
	return !made.IsZero() && now.Sub(made) > time.Duration(days)*24*time.Hour
}

// ReadPurgeBatch reports how far a RequestPurge has got.
//
// A batch id nobody was given comes back empty rather than as an error: the
// caller is polling, and "nothing here" is an answer it can show.
func (h *actorHandler) ReadPurgeBatch(ctx context.Context, req *graphpb.ActorPurgeBatchReadRequest) (*graphpb.ActorPurgeBatchReadResponse, error) {
	requests, err := h.env.Repos.PurgeRequests.BrowseBatch(req.BatchId)
	if err != nil {
		return nil, err
	}

	response := &graphpb.ActorPurgeBatchReadResponse{
		BatchId: req.BatchId,
		Total:   int64(len(requests)),
	}

	for _, request := range requests {
		switch request.Status {
		case repositories.PurgeRequestDone:
			response.Done++
		case repositories.PurgeRequestFailed:
			response.Failed++
		default:
			response.Queued++
		}

		response.Items = append(response.Items, &graphpb.ActorPurgeItem{
			Id:     request.ActorID,
			Alias:  request.ActorAlias,
			Status: request.Status,
			Error:  request.Error,
		})
	}

	return response, nil
}

// PublicAccessCounts says how much on the installation is public.
func (h *actorHandler) PublicAccessCounts(ctx context.Context, req *graphpb.PublicAccessCountsRequest) (*graphpb.PublicAccessCountsResponse, error) {
	counts, err := h.env.Repos.PublicAccess.Count()
	if err != nil {
		return nil, err
	}

	return &graphpb.PublicAccessCountsResponse{
		People: counts.People,
		Groups: counts.Groups,
		Posts:  counts.Posts,
	}, nil
}

// MakePublicRegistered makes everything public visible to signed-in people
// only, and returns what it changed.
//
// AUTHORITY IS THE CALLER'S TO CHECK, as for a purge: super administrators
// only, with a proof of identity, on an installation that no longer allows
// anything new to be public. This service knows none of the three.
func (h *actorHandler) MakePublicRegistered(ctx context.Context, req *graphpb.MakePublicRegisteredRequest) (*graphpb.PublicAccessCountsResponse, error) {
	var performedBy int64
	if req.Viewer != nil {
		performedBy = req.Viewer.Id
	}

	log.Printf("Received MakePublicRegistered request from %d", performedBy)

	counts, err := h.env.Repos.PublicAccess.MakeRegistered(performedBy)
	if err != nil {
		log.Println("make public registered:", err)
		return nil, err
	}

	log.Printf("made %d people, %d groups and %d posts registered-only",
		counts.People, counts.Groups, counts.Posts)

	return &graphpb.PublicAccessCountsResponse{
		People: counts.People,
		Groups: counts.Groups,
		Posts:  counts.Posts,
	}, nil
}

// Archive marks an actor archived. Permanent, and destroys nothing.
//
// The content keeps its URLs and stays readable forever; what stops is being
// pushed at people who did not ask for it, and — for a group — appearing in
// search. See SetArchiveQuery and SetArchiveSearchQuery.
//
// No Unarchive RPC, deliberately. Archiving is for an actor that is finished,
// and an undo would make it the gentle-sounding option somebody picks instead
// of deciding.
func (h *actorHandler) Archive(ctx context.Context, req *graphpb.ActorArchiveRequest) (*graphpb.ActorArchiveResponse, error) {
	log.Printf("Received Archive request: %v", req.Id)

	actor, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(actor.Type, constants.NodeActorPrefixEntityType) {
		return nil, status.Errorf(codes.InvalidArgument,
			"archive is for actors, not %s", actor.Type)
	}

	archived, err := h.env.Repos.Nodes.ArchiveActor(req.Id)
	if err != nil {
		return nil, err
	}

	if !archived {
		// Already archived, or deleted. Not an error — the caller wanted it
		// archived and it is, or it is on its way out instead.
		log.Printf("actor %d was not archived; already archived or deleted", req.Id)
		return &graphpb.ActorArchiveResponse{Archived: false}, nil
	}

	var archivedBy int64
	if req.Viewer != nil {
		archivedBy = req.Viewer.Id
	}

	if err := h.env.Repos.NodeDeletionLogs.Add(
		actor.ID, actor.Alias, actor.Type, archivedBy,
		repositories.DeletionActionArchived,
	); err != nil {
		log.Println("actor archive: audit write failed:", err)
	}

	h.publishActorLifecycle(
		event.EventFormatNodeActorArchived, actor, archivedBy, viewerPersonType(req.Viewer))

	return &graphpb.ActorArchiveResponse{Archived: true}, nil
}

// TouchLastVisit records that an actor just signed in.
//
// No viewer and no permission check: the caller has already authenticated the
// person, and there is nothing here that could be used against somebody else's
// account beyond moving a timestamp forward on it.
//
// Narrowed to people. Only a person signs in — a group or an event has no
// credentials — so a request naming any other actor type is a caller bug and
// says so rather than quietly stamping a group.
func (h *actorHandler) TouchLastVisit(ctx context.Context, req *graphpb.ActorTouchLastVisitRequest) (*graphpb.ActorTouchLastVisitResponse, error) {
	actor, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	if actor.Type != constants.NodeActorPersonEntityType {
		return nil, status.Errorf(codes.InvalidArgument,
			"only a person signs in, not %s", actor.Type)
	}

	changed, err := h.env.Repos.Nodes.TouchLastVisit(req.Id)
	if err != nil {
		return nil, err
	}

	// No audit entry and no event. A sign-in is already recorded in the auth
	// log, which carries the device and the address this does not; publishing
	// it again here would put a lifecycle event on the bus for every sign-in
	// on the site.
	return &graphpb.ActorTouchLastVisitResponse{Changed: changed}, nil
}

// SetEnabled turns an actor on or off.
//
// The reversible one. Its own RPC rather than a field on Edit because
// `enabled` had accumulated three meanings, and separating them is the point:
// this is a switch an administrator flips, not a by-product of deleting.
func (h *actorHandler) SetEnabled(ctx context.Context, req *graphpb.ActorSetEnabledRequest) (*graphpb.ActorSetEnabledResponse, error) {
	log.Printf("Received SetEnabled request: %v -> %v", req.Id, req.Enabled)

	actor, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(actor.Type, constants.NodeActorPrefixEntityType) {
		return nil, status.Errorf(codes.InvalidArgument,
			"enable/disable is for actors, not %s", actor.Type)
	}

	changed, err := h.env.Repos.Nodes.SetEnabled(req.Id, req.Enabled)
	if err != nil {
		return nil, err
	}

	if !changed {
		return &graphpb.ActorSetEnabledResponse{Changed: false}, nil
	}

	var changedBy int64
	if req.Viewer != nil {
		changedBy = req.Viewer.Id
	}

	action := repositories.DeletionActionDisabled
	format := event.EventFormatNodeActorDisabled
	if req.Enabled {
		action = repositories.DeletionActionEnabled
		format = event.EventFormatNodeActorEnabled
	}

	if err := h.env.Repos.NodeDeletionLogs.Add(
		actor.ID, actor.Alias, actor.Type, changedBy, action,
	); err != nil {
		log.Println("actor set-enabled: audit write failed:", err)
	}

	h.publishActorLifecycle(format, actor, changedBy, viewerPersonType(req.Viewer))

	return &graphpb.ActorSetEnabledResponse{Changed: true}, nil
}

// SetFeatured features or unfeatures an actor — the accounts onboarding offers
// a new member to follow.
//
// SUPER ADMINISTRATORS ONLY, and the caller enforces that: this service has no
// notion of a usertype, as with Purge.
//
// No audit entry and no event. Featuring is not a lifecycle change — nothing
// about the actor's own state moves, and nothing downstream indexes it — and
// the node_deletion_logs actions are all lifecycle.
func (h *actorHandler) SetFeatured(ctx context.Context, req *graphpb.ActorSetFeaturedRequest) (*graphpb.ActorSetFeaturedResponse, error) {
	log.Printf("Received SetFeatured request: %v -> %v", req.Id, req.Featured)

	actor, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(actor.Type, constants.NodeActorPrefixEntityType) {
		return nil, status.Errorf(codes.InvalidArgument,
			"featuring is for actors, not %s", actor.Type)
	}

	changed, err := h.env.Repos.Nodes.SetFeatured(req.Id, req.Featured)
	if errors.Is(err, repositories.ErrNotFeaturable) {
		// FailedPrecondition, and the sentinel's text in the message, which is
		// what the HTTP services match on to answer 409.
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	if err != nil {
		return nil, err
	}

	return &graphpb.ActorSetFeaturedResponse{Changed: changed}, nil
}

// viewerPersonType reads the type off a possibly-absent viewer.
func viewerPersonType(v *graphpb.Viewer) string {
	if v == nil {
		return ""
	}
	return v.PersonType
}

// Ping return a pong message
func (h *actorHandler) Ping(ctx context.Context, req *graphpb.ActorPingRequest) (*graphpb.ActorPingResponse, error) {
	log.Printf("Received Ping request: %v", req.Message)
	return &graphpb.ActorPingResponse{
		Message: "Pong",
	}, nil
}
