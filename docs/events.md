# Events

An event is a group with a time and a place: an art performance, a block
party, a meeting, a class. It is followed, posted on and looked after by its
admins, whom the web app calls its **hosts**. What every profile has, an
event has too ([Profiles](profiles.md)), and everything said of a group in
[Permissions](permissions.md) is true of an event: its access, what may be
posted on it, who can invite to it. This page is what only an event has.

## Contents

1. [Who may do what](#who-may-do-what)
2. [What is on](#what-is-on)
3. [Reminders and when it is over](#reminders-and-when-it-is-over)
4. [The API](#the-api)
5. [Where the code is](#where-the-code-is)

## Who may do what

Going to an event is following it. The answer, **going** or **maybe**, is on
the follow. Following an event the ordinary way, with nothing said, counts as
maybe.

| Action | Who |
| --- | --- |
| Make an event of your own | Anybody signed in, unless the installation sets `EVENTS_FROM` higher. They look after it, and are going |
| Make an event for a group | That group's admins, and nobody else, whatever `EVENTS_FROM` says. The group's admins become the event's |
| Answer going or maybe | Somebody who can see the event and whom it has not blocked, or somebody invited to it |
| Take an answer back | Always yours to do |
| See where to join online, and the address it is held at | Whoever is going, and whoever looks after the event. Not maybe, not invited |
| Change it, call it off | Whoever looks after it |

1. **Places.** An event can limit how many are going. Going is refused when
   every place is taken (409 `event_full`); maybe is not counted. The check and
   the answer are made under a lock on the event, so the last place is given
   once however many ask at the same moment.
2. **Over or called off.** No answers are taken (410 `event_closed`), and
   nobody can be invited. Leaving still works. An event that is called off is
   kept, saying so.
3. **Invitations.** Accepting one is going, or maybe when every place is
   taken. An event with a limit on places may have twice that many invitations
   waiting, and never fewer than 50; with no limit, 500 as for a group.
4. **For the followers of the group hosting it.** An event hosted by a group
   can be opened to everybody who follows that group
   (`events.open_to_host_followers`). They are then treated as invited, all at
   once: shown the event's name, time and description where they could not
   otherwise see it, in the group's list of events and on the event's own
   page, and able to answer going or maybe. It is not an access level. What is
   posted on the event stays for the people who answered, and a follower of
   the group sees that only once they have. The relation says so as
   `via_host`.
5. **Where it is held** is an address kept on the event's own node
   (`geo_address` and the fields beside it), not a place on the site's map.
   Places are public and shared, and an event at somebody's home should not
   put their home among them. Nothing searches for an event by it. A public
   venue can be tagged on the event like a place on any post.
6. **Its place on a map** is kept only when whoever saves the event asks for
   a map (`show_map`), since finding it sends the address to a geocoder. The
   point is sent with the address and to nobody who is not sent the address.
7. **Its address on the site** is set when it is made and does not change when it is
   renamed, since invitations and calendars link to it.

## What is on

**What is on** (`GET /events/`) lists every event still to come or on now
that the viewer may see, by the same gate as any list of profiles, whoever
hosts it, and not called off: soonest first, never ranked. A visitor gets
the public ones. A site administrator is gated as anybody is: it is a list
of things to go to. It can be narrowed:

| Parameter | Narrows it to |
| --- | --- |
| `from`, `to` | The ones on at some time between two RFC 3339 instants. The web app sends a day, a weekend, a week or a month on the reader's own calendar |
| `online=true` | The ones with an address to join online |
| `following=true` | The ones somebody the viewer follows hosts, looks after, or said going or maybe to. Nothing, to a visitor |

The web app's **For you** is not this list: it is the recommendation
service's events, each with why ([Recommendations](recommendations.md)).

## Reminders and when it is over

Everybody who answered is told when an event moves or is called off, and
reminded the day before ([Notifications](notifications.md)). Thirty days
after it ends it is archived, by the account sweeper
([Account lifecycle](account-lifecycle.md#the-sweeper)): kept, with
everything posted on it, and closed to anything new.

An event, and what is posted on it, stays here: it is not on the fediverse or
Bluesky ([Federation](federation.md#groups-events-and-the-rest)).

## The API

Under `/events/`: what is on (`GET /events/`), the viewer's own
(`GET /events/mine`), the featured ones, reading one, making one (`POST
/events/`, for oneself or for a group one looks after), changing it, going
or maybe (`PUT` and `DELETE /events/:id/rsvp`), who is going
(`GET /events/:id/attendees/`), a file for a calendar (`GET
/events/:id/ics`), calling it off (`POST /events/:id/cancel`), and its rules.
What every profile has is there too, at `/events/:id/…`
([Profiles](profiles.md#the-api)). Each is sent as `actor.EventNode`, whose
`event` says when it is, how many may go and are going, and what the viewer
answered.

## Where the code is

| What | Where |
| --- | --- |
| The routes | `actor-service/cmd/api/routes.go`, under `events` |
| The handlers | `actor-service/cmd/api/handlers/event*.go` |
| Who may do what | `anahita-libs/permissions/event.go` (`CanRsvp`, `CanSeeOnlineURL`); setting `EVENTS_FROM` |
| What is on, and each person's lists | `graph-grpc-service/data/repositories/events.go` (`BrowseOpen`, `BrowseFor`, `BrowseHostedBy`) |
| Reminders and archiving | `account-sweeper` |
