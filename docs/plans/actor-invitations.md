# Invite followers to an actor

*Roadmap item 11, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
A person can invite their followers to follow or join a group. The invitee gets an in-app notification with **Accept** and **Decline**. It should work for future actor types too (events are planned as "groups with an expiry date", `docs/plans/done/dissolve-people-people.md:828-831`; projects and others later).

**What exists:**
- Membership is a follow edge (person → group). Admins are `edge.actor.admin.v1`. Pending joins are `edge.actor.follow-request.v1`, which becomes a follow when accepted (`graph-grpc-service/cmd/server/handlers/socialgraph_follow.go:40-88`).
- **Admins can add members with no consent:** `POST /socialgraph/:target/followers/:source` passes `CanAddOthersAsFollowers` (`anahita-libs/cmd/api/permissions/capabilities.go:158-178`, feature `socialgraph-service/follower` default `admins`, `graph-grpc-defaults/actor_features/group.json:1-6`). The added person isn't notified; the group's admins get "X is following G".
- The auth-service `/invites` are sign-up invites by email. Nothing to reuse except the "pending cap + rate window" idea. Legacy PHP had no group invites.
- **Notifications** (`notifications` table, `notification-service`) have no status and no actions. `meta` exists but is unused. The web renders by type through i18n (`src/containers/notifications/Browse/NotificationMessage.jsx:31-52`), and `ListItem.jsx` has only mark-as-read and delete.
- **Existing bugs on this path, fixed as step 0:**
  1. The web "Add follower" picker crashes: `src/api/actor/addFollowers.js:3-5` reads `params.viewer.id` and `params.offset`, which aren't passed.
  2. Accepting a follow request in the web sends POST (Add) instead of PATCH (Edit) (`src/api/actor/followRequests.js:7-10` vs `socialgraph-service/cmd/api/routes.go:45`).
  3. Ignoring a follow request doesn't delete the request edge (`socialgraph_follow.go:194-195` only deletes follow edges).
  4. `allow_follow_request` can be turned on but never off (`graph-grpc-service/data/repositories/nodes_edit.go:119-121`).
  5. **Hole:** anyone with the `socialgraph:write` scope can make *someone else* follow a *person* (`socialgraph-service/cmd/api/permissions/followers.go:91-93`).
  6. On unfollow, notification-service deletes by `SubjectID` (the viewer), but notifications were created with `ObjectID` (the source) (`notification-service/cmd/api/handlers/event_edge_follow.go:21` vs `:98`), so an admin removal deletes the wrong row.
  7. Follow requests produce no notification at all: graph-grpc doesn't publish `edge.follow-request.*` and notification-service doesn't listen for it.
  8. Follower search ignores `q`: `GraphBrowseConnectionNodesRequest` has no search field (`anahita-libs/data/services/graph_request.go:69-81`). The invite picker needs this.

## Decisions (item 11)
- **Invites replace direct-add.** A group admin can no longer make someone a member without their consent. Site admins keep direct-add for moderation and migration.
- **Who can invite is a group setting with two choices:** admins only, or any follower (member) of the group. **Default: admins only**, matching today's `follower` permission default; each group can open it up to followers.
- **Invitees must be the inviter's own followers.**

## Design
**Storage: an edge, so it's generic for every actor type.**
- `edge.actor.invite.v1`: `node_a` = invitee (person), `node_b` = actor (group), `created_by` = inviter, `end_date` = expiry (30 days).
- `meta` holds `{ status: pending | declined }`.
- `uc_edge` gives one invitation per person per actor.
- **Declined** invitations keep the edge for a 90-day cooldown, so the same group can't re-invite the same person repeatedly. After that the sweeper deletes it. Accepting creates the follow edge and deletes the invite edge. Revoking or expiry deletes it.
- The group JSON exposes `authorized.invite`, next to the existing `authorized.add_follower`.

**Permissions** (`anahita-libs`, socialgraph-service)
- **A new feature permission,** `socialgraph-service/invite`, in `graph-grpc-defaults/actor_features/group.json` (default `admins`, allowed values `admins` | `followers`) and in `default.json` for future actor types. Future types (events, projects) get invitations by declaring the feature.
- **To invite, the inviter must:**
  - meet the actor's `invite` permission;
  - be able to see the actor;
  - have the actor active (not archived, disabled or deleted).
- **Each invitee must:**
  - follow the inviter;
  - not already be a member, or have a pending request;
  - have no block in either direction with the inviter or with the actor (two relation reads, since `IsBlocking` is one-way);
  - not be in a declined cooldown.
- **A pending invitation grants the invitee:**
  - **read access to the actor's profile** (name, avatar, description) even when it's followers-only, so they can see what they're invited to (but not its content);
  - the right to join on accept, even when the actor requires approval. The invitation is the group's consent.
- **Limits:**
  - at most 50 invitees per request;
  - at most 200 per inviter per day (the shared limiter from item 4b);
  - at most 500 pending per actor.
- **Direct-add:** `CanAddOthersAsFollowers` becomes site-admin only. The group "Add follower" UI is removed from group admins' menus, and Invite takes its place.

**Endpoints** (socialgraph-service, calling new graph-grpc RPCs)

| Endpoint | Who |
| --- | --- |
| `POST /socialgraph/:actor_id/invites` `{ person_ids: [...] }` | Returns per-person results: `invited` \| `already_member` \| `not_follower` \| `blocked` \| `cooldown` \| `pending` |
| `GET /socialgraph/:actor_id/invites` | Actor admins: pending invitations, with who invited whom |
| `GET /socialgraph/invites` | The viewer's received pending invitations |
| `POST /socialgraph/invites/:id/accept` | The invitee |
| `POST /socialgraph/invites/:id/decline` | The invitee |
| `DELETE /socialgraph/invites/:id` | Revoke: the inviter or the actor's admins |

**Events and notifications**
- graph-grpc publishes `edge.invite.socialgraph.added|accepted|declined|deleted.v1`. Also add the missing `edge.follow-request.*` events (bug 7).
- **notification-service:**
  - `actor_invite`: to the invitee, with subject = inviter, target = actor and `meta.invite_id`.
  - `actor_invite_accept`: to the inviter, "X accepted your invitation to G".
  - `actor_follow_request`: to the actor's admins (bug 7).
- **Action state is computed live:** the notification response adds `actions: ["accept", "decline"]` only while the invitation is still pending. It's checked by `invite_id` when the list is read, so stale notifications never show buttons.
- On accept, decline, revoke or expiry, update the notification's `meta.status` so it reads "You joined G" or "Invitation expired".
- **Expiry:** the hourly sweeper (`graph-grpc-service/sweeper`) deletes invitations past `end_date` and declined ones past the cooldown, and publishes `deleted`.

**Web (anahita-web)**
- **Group page:** an **Invite** button (shown when `authorized.invite`) opens a dialog.
  - It has a searchable, paged list of *your followers*, using `/socialgraph/:viewer/followers/?q=` once bug 8 is fixed. Replace the broken `Socialgraph/Add/Select.jsx` picker with it.
  - Checkboxes, with people who are already members or already invited shown greyed out.
  - Send, then a summary of the results.
- **Group settings, Permissions tab:** "Who can invite people?" with Admins or Followers (`src/containers/actors/Settings/featurePermissions.js`).
- **Group admins:** a "Pending invitations" list with Revoke, next to Follow requests.
- **Notifications:** `ListItem.jsx` renders **Accept** and **Decline** buttons when `actions` is present, copying the button pair in `FollowRequests.jsx:75-101`. They update in place. Add i18n for `actorInvite`, `actorInviteAccept` and `actorFollowRequest` (en-GB, fr-FR).
- Fix the web bugs 1 and 2.

## Estimate

| Part | Time |
| --- | --- |
| Step 0 bug fixes (8 items above) | 3–4 h |
| Invite edge, graph RPCs, socialgraph endpoints, `invite` feature permission, read-and-join grant, limits, events, sweeper expiry, tests | 7–9 h |
| notification-service: invite, accept and follow-request notifications, live `actions`, state updates | 3–4 h |
| Web: invite dialog + follower search, permission setting, pending list, notification buttons, i18n | 5–6 h |
| **Total** | **about 18–23 h, 5–6 sessions** |

## Verification
- **Service tests:**
  - An admins-only group refuses an invite from a member; a followers group allows it.
  - Inviting someone who doesn't follow you gives `not_follower`.
  - A blocked person gives `blocked`, in either direction.
  - A second invite while one is pending gives `pending`, and one after declining gives `cooldown`.
  - The 51st invitee in one request gives 400, and the 201st in a day gives 429.
  - An invitee can read a followers-only group's profile but not its posts, and can join on accept.
  - Accepting after revoke or expiry gives 404 or 410.
  - A group admin can no longer direct-add; a site admin can.
- **Manual:**
  - A invites B and C to a group. B accepts from the notification and becomes a member, and A is notified. C declines, and A can't re-invite C for 90 days.
  - The admin sees and revokes a pending invite, and the invitee's notification loses its buttons.
  - An invitation 30 days old expires through the sweeper.

## Dependencies
- The shared rate limiter from item 4b (whichever is built first creates it).
- Item 3 (recommendations) can later suggest "Invite people who might like this group", using the same edges.
- Item 8 phase 4 (groups as ActivityPub `Group` actors) can later map invitations to `Invite` activities.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `permissions.md`**: replace *Adding followers to a group* with an *Invitations* section (who may invite, who may be invited, limits, cooldown, what a pending invitation grants); site admins keep direct add.
- **services `architecture.md`**: the invite edge, endpoints and events.
- **web `architecture.md`**: the Invite dialog, notification actions (Accept/Decline) and pending invitations.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
