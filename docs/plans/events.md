# Events: a minimalist event actor

*Roadmap item 12, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
Events are "groups with an expiry date plus organiser features" (`docs/plans/done/dissolve-people-people.md:828-831`). The groundwork is already in place:
- `nodes` has `starts_at` / `ends_at` (indexed), and the `geo_*` columns;
- actor types automatically share the alias namespace, profiles, covers, avatars, media tabs, follows and access levels;
- item 11 adds invitations as an edge that any actor type can use.

The goal is the smallest event feature that's genuinely useful, not a ticketing system.

## What a minimalist event needs (the recommendation)
1. **When:** start and end, stored in UTC in `starts_at` / `ends_at`, plus a new **`timezone_name`** column (IANA, e.g. `America/Toronto`), so times display correctly across daylight-saving changes. The legacy integer `timezone` column couldn't do that, and was dropped on 2026-10-02. No all-day or recurring events in v1.
2. **Where:** either
   - a **place**, reusing location-service (a location tag edge plus the `geo_*` columns, and the map from `Map.jsx`); or
   - **online**, a meeting URL that **only people who are going** (and hosts) can see; or both.
3. **Who hosts:** the creator is an admin (the existing `edge.actor.admin.v1`). Optionally **hosted by a group** the creator administers (decided 2026-09-28):
   - A new edge `edge.actor.host.v1` runs from the group to the event.
   - The event then appears on the group's **Events** tab, and the group's admins count as event admins.
   - One host group in v1.
4. **RSVP:** **Going / Maybe** (decided 2026-09-28), stored on the membership follow edge as `meta.rsvp`.
   - Leaving removes the edge; there's no explicit "not going".
   - Accepting an invitation (item 11) creates the edge with `rsvp = going`, or `maybe` when the event is full.
5. **Capacity:** an optional `capacity` counting **Going only**. When full, Going is disabled and Maybe still works (no waitlist). The invitation cap: **pending invites ≤ max(2 × capacity, 50)**, or 500 with no capacity. That replaces item 11's per-actor limit for events.
6. **Invite edge states:**
   - Use item 11's `pending` / `declined` (with cooldown).
   - Accepting becomes the RSVP.
   - After the event ends, the sweeper deletes pending invitations, and the invite endpoint rejects new ones.
7. **Visibility:** the existing access levels.
   - `public`, `registered`, or **invite-only**: `followers`, plus item 11's rule that a pending invitation grants read access to the profile.
   - A group-hosted event can also be limited to the group's members. That's a new access keyword `host_followers`, checked through the host edge.
8. **Discussion:** the same media tabs as groups (notes and photos), chosen by `TAB_COMPONENTS`. Hosts can post announcements, which are ordinary notes pinned first using item 15's `pinned_at` (newest first if item 15 isn't done yet).
9. **Before, during and after:**
   - **Upcoming** until `starts_at`, **happening** until `ends_at`, then **past**.
   - Past events can't take RSVPs or invitations, but stay readable.
   - The sweeper **archives** an event 30 days after it ends, using the existing permanent, read-only archive state (`done/actor-lifecycle-states.md`).
10. **Notifications** (in-app; notification email doesn't exist yet):
    - an invitation (item 11);
    - an RSVP, to hosts (batched, one per hour);
    - **time or place changed**, to everyone going or maybe;
    - **cancelled**, to everyone going or maybe;
    - a **reminder 24 h before**, to people going. A new sweeper task, idempotent through a `reminded_at` column.
11. **Cancel:** a `cancelled_at` column. The event shows "Cancelled", RSVPs close, and attendees are notified. It's kept, not deleted, so people can see what happened.
12. **Calendar:** `GET /events/:id/ics`, an iCalendar file with the time zone, the place or URL and a link back. Also "Add to calendar" in the web app. It's tiny and very useful.
13. **Finding events** (no site-wide browse, per item 2):
    - an **"Events" page** with tabs: Upcoming (going or maybe), Invited, Hosting, Past;
    - a group's Events tab;
    - a person's profile Events tab showing only public events they host or attend (if their privacy allows);
    - nearby or suggested public events later, through item 3 (recommendations) or feed generators.

## Where it lives
- **actor-service serves `/events` beside `/people` and `/groups`.** (When this was planned it was to be group-service; people and groups were merged into actor-service on 2026-10-05, see [done/actor-service.md](done/actor-service.md).) The type is `node.actor.actor-service.event.v1`: a new kind is named for the service it was born in.
- Most handlers (CRUD, access, features, admins, archive, delete, purge) are shared and parameterised by type. Event-specific handlers are RSVP, cancel and ics.
- A separate event-service would duplicate most of what actor-service already has for no benefit.
- Add the type to `feed-defaults` `actor_entity_types` and to `graph-grpc-defaults/actor_features/event.json`, which layers on `default.json` and sets the `invite` feature (item 11) to `admins` by default.

## Data changes (one migration, plus `init.sql` and `legacy/upgrade.sql`)
- **On `nodes`:** `timezone_name varchar(64) NULL`, `capacity int NULL`, `online_url varchar(512) NULL` (returned only to people going and to hosts), `cancelled_at datetime NULL`, `reminded_at datetime NULL`.
- **Or** keep the event-only fields in a side table `events(node_id PK, timezone_name, capacity, online_url, cancelled_at, reminded_at)`. That's cleaner, since `nodes` is already wide. **Recommended: the side table**, owned by actor-service's events, through graph-grpc like every other table an actor's handlers use.
- **New edge type:** `edge.actor.host.v1` (group → event).
- **Follow edge `meta.rsvp`** (going | maybe). The graph needs a count of Going for the capacity check: a transactional check-and-insert under a row lock on the event, so two people can't take the last spot at the same moment.

## Endpoints (actor-service, plus graph-grpc RPCs)

| Endpoint | Purpose |
| --- | --- |
| `POST /events` (optional `host_group_id`), `GET /events/:id`, `PATCH /events/:id` | CRUD. Changing the time or place publishes `event.edited` with a `changed` field, which drives the notification. |
| `PUT /events/:id/rsvp` `{ rsvp: going \| maybe }`, `DELETE /events/:id/rsvp` | RSVP. Returns 409 `event_full` for Going when full, and 410 when the event is past or cancelled. |
| `POST /events/:id/cancel` | Admins only |
| `GET /events/:id/attendees?rsvp=going\|maybe` | The existing followers browse, filtered by `meta.rsvp` |
| `GET /events/:id/ics` | Calendar file |
| `GET /events/?filter=upcoming\|invited\|hosting\|past` | The viewer's own lists, ordered by `starts_at` |
| `GET /groups/:id/events` | A group's events |

## Web (anahita-web)
- **Create and edit form:** title, description, start and end (date-time pickers in the chosen zone, defaulting to the browser's), a zone selector, place (location autocomplete) and/or online URL, capacity, visibility, and the host group (from groups you administer).
- **Event page:** the actor Read layout with a date/time block (in the viewer's zone, with the event's zone noted), place with map or "Online", host(s), Going/Maybe counts with avatars, **RSVP buttons** (disabled when full, with "Event full"), "Add to calendar", Invite (item 11), and the Cancelled / Past banners. The media tabs sit below.
- **Events page** with its four tabs, a group Events tab, and a person profile Events tab.
- Notification strings for invitations, RSVPs, changes, cancellation and reminders (en-GB, fr-FR).

## Later (recorded, not built)
- a waitlist;
- recurring and all-day events;
- co-host groups;
- ticketing or questions on RSVP;
- notification email;
- nearby or suggested events (item 3);
- **federation:** ActivityPub `Event` objects compatible with Mobilizon and Gancio (`startTime`, `endTime`, `location`, `joinMode`, `Join` / `Accept`), and on AT Protocol the community calendar lexicons (`community.lexicon.calendar.event` and `rsvp`, used by Smoke Signal and OpenMeet). This belongs to item 8's later phases.

## Estimate

| Part | Time |
| --- | --- |
| Data: `events` side table, host edge, `rsvp` on follow meta, migration | 2–3 h |
| actor-service: event type through the shared handlers, event-specific handlers (RSVP with capacity lock, cancel, ics, lists), `event.json` features, permissions (online URL visibility, `host_followers` access) | 7–9 h |
| Sweeper: past, archive after 30 days, expire invites, 24 h reminders | 2–3 h |
| Notifications: RSVP (batched), changed, cancelled, reminder | 2–3 h |
| Web: create/edit form with time zones, event page, RSVP, events page + tabs, group/profile tabs, i18n | 8–10 h |
| **Total** | **about 21–28 h, 5–7 sessions** |

## Verification
- **Capacity:** with capacity 2, three people press Going at the same moment; two succeed and one gets `event_full`. Moving from Going to Maybe frees a spot.
- **Time zones:** an event created for 19:00 America/Toronto shows 01:00 next day to a viewer in Europe/Paris, with the zone noted. The `.ics` file imports correctly into Apple, Google and Thunderbird calendars, including across a DST change.
- **Privacy:** the online URL is absent from the API response for Maybe, invitees and strangers, and present for Going and hosts. An invite-only event's profile is visible to invitees but not its posts.
- **Lifecycle:** after the end, RSVP returns 410. After 30 days the event is archived, and the reminder is sent exactly once, 24 h before.
- **Group hosting:** a group admin creates an event hosted by the group, and it shows on the group's Events tab. Another admin of that group can edit it.

## Decisions (item 12)
- Events live in actor-service as `node.actor.actor-service.event.v1`, with a side table for event-only fields.
- Events can be hosted by people or groups (one host group).
- RSVP is Going / Maybe. Capacity counts Going. When full, there's no waitlist.
- Pending invitations are capped at max(2 × capacity, 50).

## Dependencies
- **Hard:** item 11 (invitations), including its sweeper expiry.
- **Uses:** location-service, the archive lifecycle and the sweeper.
- **Soft:** item 2 (no global browse, hence the personal Events page), item 3 (event suggestions later), item 8 (federation as AP `Event` later).

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `permissions.md`**: events (who can create, RSVP, see the online URL; `host_followers`; invite caps).
- **services `architecture.md`**: events in actor-service, the host edge, the `events` side table, the sweeper's event tasks.
- **web `architecture.md`**, *Routes*: `/events` and the event pages and tabs.

## What was built

### Phase 1, the event itself: 2026-10-10

Server only. An event can be made, read, changed, answered and called off through the API. Nothing in the web app yet.

- **The type** `node.actor.actor-service.event.v1` (`constants.NodeActorEventEntityType`), with `graph-grpc-defaults/actor_features/event.json` and an entry in `feed-defaults`.
- **One handler for both.** actor-service's group handler is told which kind it serves (`nodeType`), and serves events from the same code: read, list, access, features, administrators, archive, delete, purge. `handlers/event.go` adds what only an event has. `/events/` is routed by the gateway.
- **The `events` table** (migration `20261017000001_events`, `init.sql`, `legacy/upgrade.sql`): time zone, places, online address, called off, reminded. When it starts and ends are the node's `starts_at` and `ends_at`.
- **Answering** is following, with the answer in the follow edge's meta (`constants/events.go`). `EventsRepo.Rsvp` records it under a lock on the event's row.
- **Routes:** `POST /events/`, `GET /events/:id`, `PATCH /events/:id`, `PUT` and `DELETE /events/:id/rsvp`, `POST /events/:id/cancel`, and everything `/groups/:id/...` has.
- **The response** carries an `event` block: times in UTC, the zone, places, counts, `is_full`, the viewer's answer, and `state` (upcoming, happening, past, cancelled). `online_url` is in it only for whoever is going or looks after the event; `has_online_url` says there is one.
- **Invitations know events:** none to an event that is over or called off (`closed`), fewer waiting when places are limited (`constants.InviteMaxPendingFor`), and accepting is going.

Found and fixed on the way: **making a group made the group follow its maker**, not the maker follow the group. The two were the wrong way round in the call. No group had been made through this service on the development database, so no rows needed mending.

Differences from the design above:

- **Times are given as RFC 3339 with an offset** and stored in UTC, with the zone's name beside them.
- **An event's address does not change when it is renamed.** A group's does.
- **A follow with no answer counts as maybe,** so following an event from anywhere that offers Follow does something sensible.
- **`GET /events/` listed every event the viewer may see** in this phase, as `/groups/` does. It was removed in phase 2, when the viewer's own lists arrived.

Tests: `permissions/event_test.go`, `responses/event_test.go`, `constants/invites_test.go`; the graph's `TestEventTimesAreAnEventsAlone`; on a real database `TestEventAnswers` and `TestEventLastPlaceIsGivenOnce` (four people asking for one place at once).

### Phase 2, lists and the calendar file: 2026-10-10

Server only.

- **`GET /events/mine?filter=upcoming|past|hosting|invited`:** the viewer's own events. Upcoming and past are what they answered, split by whether it is over; hosting is what they look after; invited is what they were asked to and have not answered. Soonest first for what is to come, latest first for the rest. An event they were invited to and may not yet see comes as its name, picture and time, marked `restricted`.
- **`GET /events/:id/attendees/?rsvp=going|maybe`:** who is going, or said maybe, to anybody who can see the event, each person gated as a follower list is. Through the graph's neighbour query, which can now be narrowed by what an edge's meta says.
- **`GET /events/:id/ics`:** the event as a calendar file. Its times are written in UTC, which every calendar shows in its owner's zone, so no zone rules are in the file. `STATUS:CANCELLED` when it was called off. The address to join online is in it only for whoever is going or looks after the event. The link back needs `WEB_ORIGIN` set for actor-service.
- **`GET /events/` was removed.** There is no list of every event, as the plan says.
- **The limited view of an event says when it is,** to somebody invited.

Tests: `TestEventListsAndWhoIsGoing` on a real database; `handlers/event_ics_test.go`.

### Phase 3, hosting by a group: 2026-10-10

Server only.

- **`host_group_id` when an event is made,** for somebody who administers that group. The host is an edge from the group to the event (`edge.actor.host.v1`), set once.
- **The group's administrators are copied onto the event** as its own administrators when it is made (`carryOverAdmins`). Nothing is asked of the group afterwards: the user chose this on 2026-10-10, over working out an event's administrators from its host each time. Somebody who later becomes or stops being an administrator of the group is added to or removed from the event by hand.
- **`GET /groups/:id/events/?filter=upcoming|past`:** the events a group hosts, each only if the viewer may see it. Following the host is not following the event.
- **The event says who hosts it** (`event.host`), when the viewer may see that group.

Tests: `TestEventsHostedByAGroup` on a real database.

### Phase 4, the web app: 2026-10-10

In anahita-web; see *Events* in its `docs/architecture.md`.

- **Events** in the left menu: the viewer's own lists, with a + for whoever may make one.
- **The form:** name, description, start and end on the clocks of a chosen time zone, places, a link to join online, and who can see it. `utils/events.js` turns the chosen times into instants with `Intl`, tested across clock changes.
- **An event's page** is the actor page with a panel above its tabs: when (in the reader's zone, and the event's when that differs), who hosts it, how many are going, Going and Maybe, Add to calendar, and for its administrators Edit and Cancel.
- **A group's Events tab,** with Add event for its administrators, which makes the group the host.
- **Not built in the web app:** choosing a place for an event with the locations picker. An event's page has the same Locations tab a group's has.

### Still to do

1. **The sweep:** archive 30 days after the end, drop invitations to events that are over, remind whoever is going a day before.
2. **Notifications:** an answer (to hosts), a change of time or place, calling off, the reminder.
3. **`host_followers` access,** for an event open to its host group's followers.
4. **Who may make an event.** It follows `GROUPS_FROM`, which is administrators unless set otherwise. Asked of the user on 2026-10-10 and not answered: a setting of its own, `EVENTS_FROM`, was suggested.
5. **Start a group from an event,** raised by the user on 2026-10-10: a new group with the event's hosts as administrators, to which whoever went is invited. Proposed in place of turning the event itself into a group, which would make followers of people who agreed to one evening and would break the event's address. Not decided.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Phase 1, the event itself (server) | done 2026-10-10 |
| Phase 2, lists and the calendar file (server) | done 2026-10-10 |
| Phase 3, hosting by a group (server) | done 2026-10-10 |
| Phase 4, the web app | done 2026-10-10 |
| The rest, listed under Still to do | not started |
