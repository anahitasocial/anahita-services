# Groups

A group is a profile made by somebody signed in, looked after by its admins,
and followed by people: a class, a newsroom's desk, a reading group, a team.
A group has **followers**, never members. What every profile has, a group
has too ([Profiles](profiles.md)). This page is what only a group has, and
where each is described.

## Contents

1. [Making one](#making-one)
2. [What only a group has](#what-only-a-group-has)
3. [The API](#the-api)
4. [Where the code is](#where-the-code-is)

## Making one

Who may make a group is the installation's choice, `GROUPS_FROM`:
administrators and super administrators unless it says otherwise
([Configuration](configuration.md)). Whoever makes one is its first admin and
its first follower, and chooses who may see it as it is made: anyone, anyone
signed in, its followers, or only its admins for now, as a draft
([Privacy](privacy.md#who-can-see-a-profile-or-a-group)). Where the
installation allows nothing new to be public, a public group is refused.
Its handle is made from its name, and changes when it is renamed.

## What only a group has

| What | Said in |
| --- | --- |
| Rules, which each follower ticks one by one before following, and agrees to again when they change | [Permissions](permissions.md#following), [Abuse reports](abuse-reports.md#breaking-a-rule) |
| Reports against its rules, which its admins see without who made them | [Abuse reports](abuse-reports.md#a-groups-own-reports) |
| The events it hosts, made by its admins, who are then the event's | [Events](events.md) |
| Posts by its followers, as its admins allow | [Permissions](permissions.md#posting) |
| Inviting people to follow it, up to 500 waiting at once | [Permissions](permissions.md#invitations) |
| Being left without an admin, and a site administrator appointing one | [Account lifecycle](account-lifecycle.md#groups-left-without-an-administrator) |

A group, and what is posted on it, stays here: it is not on the fediverse or
Bluesky ([Federation](federation.md#groups-events-and-the-rest)).

## The API

Under `/groups/`: listing groups and reading one, making one (`POST
/groups/`, with `access`), changing it, its rules (`GET` and `PUT
/groups/:id/rules`, `POST /groups/:id/rules/agree`), the events it hosts
(`GET /groups/:id/events/`), removing one as spam, and purging many at once.
What every profile has is there too, at `/groups/:id/…`
([Profiles](profiles.md#the-api)). Each is sent as `actor.GroupNode`.

## Where the code is

| What | Where |
| --- | --- |
| The routes | `actor-service/cmd/api/routes.go`, under `groups` |
| The handlers | `actor-service/cmd/api/handlers/group*.go` |
| Who may do what | `anahita-libs/permissions/actor.go`, `follow.go` |
