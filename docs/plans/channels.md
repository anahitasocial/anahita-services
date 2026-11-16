# Websites become channels (roadmap row 147)

Asked for by the user on 2026-11-15: a profile for an organization (a
business, a non-profit, a newsroom), a brand or a project (research, an
experiment, open source), which only it posts on. "I want to post Anahita
updates and pictures, and I don't want those pictures on my personal photo
feed." Websites (row 38) are most of it already: an `Organization` on the
fediverse, admins who reply as it, who wrote each reply kept. A channel is
a website whose admins post as it, with the feed one thing it may have.

## Contents

1. [The name](#the-name)
2. [What a channel is](#what-a-channel-is)
3. [A channel beside a group](#a-channel-beside-a-group)
4. [What it posts](#what-it-posts)
5. [Who wrote it](#who-wrote-it)
6. [Making one, and its feed](#making-one-and-its-feed)
7. [Privacy, and the networks](#privacy-and-the-networks)
8. [Events](#events)
9. [The rename](#the-rename)
10. [What the code does today](#what-the-code-does-today)
11. [Out of scope](#out-of-scope)
12. [Documentation](#documentation)
13. [Phases](#phases)

## The name

**Channel**, settled by the user on 2026-11-15: a common word, as person,
group and event are, and already the word people know for a profile that
only its owner posts on and that others follow, like, reply to and share.
A plain word for the thing can be anybody's (the reason Website was chosen
over Pulse on 2026-10-16). Weighed and left: Pulse, Space, Node and Hub,
then Page and Voice. Node means a server on the fediverse, Hub a WebSub
server, and Space and Hub a place where everybody posts. syndication-service
keeps its name: it reads feeds.

## What a channel is

1. **A profile that only it posts on.** Its followers reply, like, repost
   and quote; they post nothing on it.
2. **Its admins write as it.** The channel is the owner and the author
   shown of everything on it, and its profile photo is the one shown.
3. **Who wrote each post is kept,** for its admins and the site's
   administrators, never sent elsewhere.
4. **A feed is one thing it may have:** a website's RSS, Atom or JSON feed,
   from a domain its admins prove is theirs, whose items become the
   channel's notes as a website's do now.
5. **It may be private,** as a group may.

## A channel beside a group

| | Group | Channel |
| --- | --- | --- |
| Who posts | Its followers, as the group allows | The channel alone, written by its admins |
| Author shown on a post | The follower who wrote it, "in" the group | The channel; no line naming a group above it in a feed |
| Admins reply under its posts | As themselves | As the channel |
| A feed from a website | No | If its admins connect one |
| On the fediverse and Bluesky | No (row 73, put off) | Once its admins turn it on, public channels only |
| Rules to agree to before following | Yes (row 102) | No: followers only reply |
| Composer | For its followers | For its admins |
| Its admins look after | Its followers' posts and the replies | The replies under its posts |
| Privacy, follow requests, invitations, admins, About, events | Yes | The same |

## What it posts

`channel.json` in `graph-grpc-defaults/actor_features/`, in place of
`website.json`: nothing for followers to add; for admins:

| Kind | A new channel | Its admins |
| --- | --- | --- |
| Notes, photos, videos | On | May turn off |
| Articles | Off | May turn on, for long pieces |
| Topics | Off | May turn on, to start a discussion its followers reply in |
| Sounds | As videos, where a site offers them | — |

A site trims this in its own `channel.json`, or switches a kind off for
everything in `instance.json` (row 139). Phase 2 checks how a kind is
offered but off until a profile's admins turn it on, through the features
a profile saves (`nodes.features`).

## Who wrote it

1. Every post and reply made as the channel is owned and created by it;
   the admin who wrote it is `written_by` in the post's `meta`, as replies
   as a website are now (`anahita-libs/handlers/reply_owner.go`), and a
   feed's notes keep the first admin as now.
2. The channel's admins see "Written by Naomi Nagata" under its posts and
   replies; nobody else does. Site administrators see it in a report's
   case, as now.
3. Nothing of it is federated, exported to anyone else, or counted.

## Making one, and its feed

1. **Add channel** (`/channels/add`), as Add group: a name, a handle, a
   profile photo, and who may see it. `CHANNELS_FROM` says who may make
   one, as `WEBSITES_FROM` does now.
2. **A feed** is connected in the channel's settings, in a section of its
   own, with the domain proof as now (a DNS TXT record or a file): found,
   proved, then read, its ten newest items and then each new one. One
   feed a channel; disconnected there too, keeping what it brought.
3. **A website already on a site** becomes a channel with its feed
   connected; nothing it has changes.

## Privacy, and the networks

1. **As a group's:** anyone, anyone signed in, or followers only; follow
   requests if its admins allow them; a stranger shown its name, handle
   and profile photo only, to ask from. A feed's items take the channel's
   access, though the site they come from is public.
2. **Only what is public goes out.** A channel that is not public has no
   fediverse and no Bluesky; its Fediverse settings say why. A public
   channel that becomes private goes as a person who becomes private
   does: its posts are taken back elsewhere.

## Events

A channel hosts events as a group does: `hostGroup` takes a channel too,
its admins carried onto the event, and `GET /channels/:id/events/`; in the
web app, the event form offers the channels one looks after, and a
channel has an Events tab.

## The rename

| Today | Becomes |
| --- | --- |
| `node.actor.actor-service.website.v1` | `node.actor.actor-service.channel.v1`, in `nodes` and `edges` by a migration rehearsed on a copy first |
| `/websites/…` (API and web app) | `/channels/…`; the web app's old addresses send to the new |
| `websites`, `website_items` | `channel_feeds`, `channel_feed_items`; `verified_domains` stays |
| `WEBSITES_FROM`, NodeInfo `websitesFrom` | `CHANNELS_FROM`, `channelsFrom`; the old setting read for one release |
| `website` in `instance.json`, a custom feed's kind, recommendations | `channel`; the old word read for one release in `instance.json`, stored feed definitions migrated |
| `suggest_websites` | `suggest_channels` |
| `AccountKindWebsite` (auth-service) | `AccountKindChannel` |
| `src/languages/*/websites.js` and website keys elsewhere | `channels.js`, "Channel" in every base |

The fediverse sees nothing change: actors are `/users/:uuid`, and a
channel's Bluesky account is tied to the profile, not its kind.

## What the code does today

From the check of both repositories on 2026-11-15:

1. A website is served by actor-service, its handler wrapping the
   group's, with no `/access`, `/features`, `/archive` or add route; it is
   made only by syndication-service's connect, public, with a feed.
2. Its admins can already post any kind on it through the API: a
   permission a profile's file does not set admits admins
   (`permissions/capabilities.go`), and such a post would be taken for a
   feed item and could not be edited (`IsWebsiteItem`). Channels make the
   posting meant; a feed item is told by its `channel_feed_items` row.
3. A connect whose feed is taken already leaves a website without a feed
   (`connect.go`, `ErrFeedTaken`). Making and connecting apart ends it.
4. Hosting an event takes a group only (`event.go`, `hostGroup`); the SQL
   takes any profile.
5. The docs and the code disagree in ten places (`docs/websites.md`'s
   migrations, its "not suggested", articles in comments where notes are
   made, Website-service names, `docs/profiles.md`'s "no features", who
   sees `written_by`, archiving, the endpoints table, the web app's
   settings tabs, the feedless website). The rename rewrites those pages.

## Out of scope

1. Rules for a channel, until somebody asks.
2. More than one feed a channel.
3. A channel's admins shown on its posts to anybody but each other.

## Documentation

1. anahita-services: `docs/websites.md` becomes `docs/channels.md`;
   `profiles.md`, `permissions.md`, `instance-features.md`,
   `federation.md`, `configuration.md`, `recommendations.md`, `events.md`,
   `notifications.md`, `the-graph.md`, `docs/README.md`, and the API
   reference; every link to the old page fixed.
2. anahita-web: `docs/architecture.md`, `docs/README.md`, the roadmap's
   wording where it names websites as they are now.
3. anahita.io, once channels are built (the user, 2026-11-15): the home
   page's features (`src/data/home.js`, "Websites"), the services page
   (`src/data/services.js`, syndication-service and its doc link to
   `websites`), the API's Profiles page (`src/app/api/actor/page.jsx`),
   and the docs it carries from this repository. Channels written as what
   they are for, posting as an organization or a project, with the feed
   one thing a channel may have.
4. anahita-recipes, once channels are built (the user, 2026-11-15): every
   recipe's README and `instance.json` that names websites (all seven, and
   `commons`' table of services), `docs/what-runs.py` (`PROFILES` and the
   descriptions) and the diagrams it draws. syndication-service then runs
   for channels' feeds only: a recipe may offer channels without feeds,
   and not run it. `theatre`'s README calls each person's videos a
   channel ("channels, not groups"), which the new word makes wrong.
5. The dev seed, where it names websites; the FediDB listing once channels
   are released.
6. `CHANGELOG.md` of both repositories, with what operators rename.

## Phases

| Phase | What | Status |
| --- | --- | --- |
| 1 | The rename, nothing else changed: types, tables, addresses, settings, words; the migration rehearsed on a copy | ✅ 2026-11-16 |
| 2 | Add channel without a feed; privacy, follow requests, archiving and features as a group's; `channel.json` | ✅ 2026-11-16, `channel.json`'s kinds of post left for phase 3, with posting as the channel |
| 3 | Admins post as the channel; Written by, for its admins; a feed item told by its row | ✅ 2026-11-16: a feed's notes marked `from_feed` in their meta as their row is written (migration `20261122000022` for those before); notes, photos and videos stay on, since a profile's saved kinds are added to its defaults, never taken from them; a post's `editor` no longer sent for a channel's |
| 4 | The feed connected and disconnected in settings | ⬜ |
| 5 | Events hosted by a channel | ⬜ |
| 6 | The docs of both repositories; anahita.io's pages; every recipe and its diagram; tried at both sizes; changelogs | ⬜ |
