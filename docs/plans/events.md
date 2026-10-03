# Events: a minimalist event actor

*Roadmap item 12, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
Events are "groups with an expiry date plus organiser features" (`docs/plans/done/dissolve-people-people.md:828-831`). The groundwork is already in place:
- `nodes` has `start_date` / `end_date` (indexed), and the `geo_*` columns;
- actor types automatically share the alias namespace, profiles, covers, avatars, media tabs, follows and access levels;
- item 11 adds invitations as an edge that any actor type can use.

The goal is the smallest event feature that's genuinely useful, not a ticketing system.

## What a minimalist event needs (the recommendation)
1. **When:** start and end, stored in UTC in `start_date` / `end_date`, plus a new **`timezone_name`** column (IANA, e.g. `America/Toronto`), so times display correctly across daylight-saving changes. The legacy integer `timezone` column couldn't do that, and was dropped on 2026-10-02. No all-day or recurring events in v1.
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
   - **Upcoming** until `start_date`, **happening** until `end_date`, then **past**.
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
- **group-service serves both `/groups` and `/events`,** the way text-service serves articles, notes and topics. The type is `node.actor.group-service.event.v1`.
- Most handlers (CRUD, access, features, admins, archive, delete, purge) are shared and parameterised by type. Event-specific handlers are RSVP, cancel and ics.
- A separate event-service would duplicate most of group-service for no benefit.
- Add the type to `feed-defaults` `actor_entity_types` and to `graph-grpc-defaults/actor_features/event.json`, which layers on `default.json` and sets the `invite` feature (item 11) to `admins` by default.

## Data changes (one migration, plus `init.sql` and `legacy/upgrade.sql`)
- **On `nodes`:** `timezone_name varchar(64) NULL`, `capacity int NULL`, `online_url varchar(512) NULL` (returned only to people going and to hosts), `cancelled_at datetime NULL`, `reminded_at datetime NULL`.
- **Or** keep the event-only fields in a side table `events(node_id PK, timezone_name, capacity, online_url, cancelled_at, reminded_at)`. That's cleaner, since `nodes` is already wide. **Recommended: the side table**, owned by group-service.
- **New edge type:** `edge.actor.host.v1` (group → event).
- **Follow edge `meta.rsvp`** (going | maybe). The graph needs a count of Going for the capacity check: a transactional check-and-insert under a row lock on the event, so two people can't take the last spot at the same moment.

## Endpoints (group-service, plus graph-grpc RPCs)

| Endpoint | Purpose |
| --- | --- |
| `POST /events` (optional `host_group_id`), `GET /events/:id`, `PATCH /events/:id` | CRUD. Changing the time or place publishes `event.edited` with a `changed` field, which drives the notification. |
| `PUT /events/:id/rsvp` `{ rsvp: going \| maybe }`, `DELETE /events/:id/rsvp` | RSVP. Returns 409 `event_full` for Going when full, and 410 when the event is past or cancelled. |
| `POST /events/:id/cancel` | Admins only |
| `GET /events/:id/attendees?rsvp=going\|maybe` | The existing followers browse, filtered by `meta.rsvp` |
| `GET /events/:id/ics` | Calendar file |
| `GET /events/?filter=upcoming\|invited\|hosting\|past` | The viewer's own lists, ordered by `start_date` |
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
| group-service: event type through the shared handlers, event-specific handlers (RSVP with capacity lock, cancel, ics, lists), `event.json` features, permissions (online URL visibility, `host_followers` access) | 7–9 h |
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
- Events live in group-service as `node.actor.group-service.event.v1`, with a side table for event-only fields.
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
- **services `architecture.md`**: events in group-service, the host edge, the `events` side table, the sweeper's event tasks.
- **web `architecture.md`**, *Routes*: `/events` and the event pages and tabs.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
