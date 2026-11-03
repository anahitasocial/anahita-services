# Profiles

A profile is how an **actor** is shown: a person, a group, an event or a
website. Every actor has an id, a name and a description, and a place in the
social graph: who follows it, who has blocked it, and, for a person, whom
they follow. Most of all, an actor owns things. The posts and updates on it
are its own, and following it is subscribing to them: what it posts comes to
its followers' home pages, and what its followers may post on it is its own
to choose.

The code and the API say **actor**; people using Anahita are only ever told
**profile**. This page is what every kind has alike. What only one kind has
is on its own page:

| Kind | What only it has | Its page | Its API |
| --- | --- | --- | --- |
| A person | An account, an account type, pronouns, tags in photos, who may quote them | [People](people.md) | `/people/` |
| A group | Made by members, rules to agree to, events it hosts | [Groups](groups.md) | `/groups/` |
| An event | A time, a place, going or maybe, a calendar file, being called off | [Events](events.md) | `/events/` |
| A website | A site's feed, its proved domain, replies as the site | [Websites](websites.md) | `/websites/` |

All four are served by actor-service, and kept in the graph as nodes of the
types in [The graph](the-graph.md).

## Contents

- [What every profile has](#what-every-profile-has)
- [Its address](#its-address)
- [Its admins](#its-admins)
- [The API](#the-api)
- [Where the code is](#where-the-code-is)

## What every profile has

Each is described once, on the page named.

| What | Said in |
| --- | --- |
| A name, a handle, a description and one link; a profile photo and a cover photo | [Its address](#its-address); the pictures are identity-image-service's ([Photos](photos.md)) |
| Who may see it: an access level, and whether those it is not open to may ask to follow it | [Privacy](privacy.md#who-can-see-a-profile-or-a-group) |
| What may be posted on it, and by whom | [Permissions](permissions.md#posting) |
| Followers, follow requests, invitations and blocks | [Permissions](permissions.md#following), [Invitations](permissions.md#invitations) |
| Whether its new photos and videos may be downloaded | [Permissions](permissions.md#downloading-photos-and-videos) |
| Being turned off, archived, deleted for thirty days, restored and purged | [Account lifecycle](account-lifecycle.md) |
| Being featured, for onboarding to offer | [Registration](registration.md#onboarding) |
| Being reported, and removed as spam | [Abuse reports](abuse-reports.md), [Account lifecycle](account-lifecycle.md#removing-as-spam) |
| Being suggested to follow | [Recommendations](recommendations.md) |

## Its address

Every profile has a handle, unique among every kind: `ana`, `ten-forward`,
`warp-theory-seminar`. Its page in the web app is `/@handle`, whatever its
kind, and `GET /actors/:alias` says which kind a handle is and its id. A
person chooses theirs when they sign up. A group's is made from its name,
and changes when it is renamed. An event's is made from its name when it is
made and stays, since invitations and calendars link to it. A website's is
chosen when it is connected.

## Its admins

A group, an event and a website are looked after by **admins**: people who
change it, choose who may see it and post on it, and add or remove other
admins. Whoever makes one is its first admin and its first follower. In the
web app an event's admins are called its **hosts**. A person looks after
their own profile and has no admins.

A group left without an admin is listed for the site's administrators to
appoint one ([Account lifecycle](account-lifecycle.md#groups-left-without-an-administrator)).

## The API

Each kind keeps its own address for what every kind has: changing who may
see a group is `PATCH /groups/:id/access`, an event's is
`PATCH /events/:id/access`. The [API reference](api/index.html) lists them
under each kind, after what is that kind's own, as **every profile's**, and
says what each does under Profiles.

| What every profile has | Route, under each kind's address |
| --- | --- |
| Who may see it | `PATCH /:id/access` |
| What may be posted on it | `PATCH /:id/features` |
| Downloading its photos and videos | `PATCH /:id/download-default` |
| Its admins | `GET`, `POST /:id/admins/`, `DELETE /:id/admins/:admin_id` |
| Turned off or on | `PATCH /:id/enabled` |
| Featured | `PATCH /:id/featured` |
| Archived | `POST /:id/archive` |
| Deleted, and what that would take with it | `DELETE /:id`, `GET /:id/deletion-counts` |
| Restored | `POST /:id/restore` |
| Purged | `DELETE /:id/purge` |

Not every kind has every one: a person has no admins and restores their
account by signing in; a website has nothing posted on it but its own feed,
so no features and no access to choose; only people and groups can be
removed as spam. Each kind's page in the reference has exactly its own.

What each is sent as: a person as `actor.PersonNode`, a group as
`actor.GroupNode`, an event as `actor.EventNode` and a website as
`actor.WebsiteNode`, each what every profile is sent (`ActorNode`) without
what only another kind has. `GET /actors/:alias` answers with
`actor.HandleResponse`.

## Where the code is

| What | Where |
| --- | --- |
| The routes, and which are every profile's | `actor-service/cmd/api/routes.go` (`Section`, `Shared`) |
| Groups, events and websites | `actor-service/cmd/api/handlers/group*.go`, `event*.go`, `website*.go` |
| People | `actor-service/cmd/api/handlers/person*.go` |
| The shapes sent | `anahita-libs/responses/actor.go`, `actor-service/responses/` |
| Who may do what | `anahita-libs/permissions/actor.go` |
