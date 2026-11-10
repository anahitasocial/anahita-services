# What a site offers (roadmap row 139, its first part)

Asked for by the user on 2026-11-09, for the recipes: "We should be able to
switch off articles, topics, events, and websites", and then, of where to
say so: "An Anahita instance is basically one big fancy actor." Each kind
of profile already has a file saying which kinds of post it has
(`graph-grpc-defaults/actor_features/`); the instance gets one too,
`instance.json`, and every profile has only what the instance has. JSON, as
the other files are (the user, 2026-11-09). The rest of row 139, how lists
are laid out and the tutorials, comes later.

## Contents

1. [Why](#why)
2. [The file](#the-file)
3. [What follows from it](#what-follows-from-it)
4. [Video and sound](#video-and-sound)
5. [Out of scope](#out-of-scope)
6. [Phases](#phases)

## Why

Anahita is small services, each doing one thing, where Mastodon and
Pixelfed are each one application (the user, 2026-11-09). A site that
offers only notes and photos, the `coffeehouse` recipe, should neither run
what it does not offer nor show it: no Articles in a composer, no Events
in a menu, and the server refusing them all the same.

## The file

`graph-grpc-defaults/actor_features/instance.json`, beside `person.json`,
`group.json`, `event.json` and `website.json`, in the same form: a list of
services, each with the kinds it offers.

```json
[
  { "service": "actor-service", "node_types": ["person", "group", "event", "website"] },
  { "service": "text-service",  "node_types": ["note", "article", "topic"] },
  { "service": "photo-service", "node_types": ["photo"] },
  { "service": "video-service", "node_types": ["video", "audio"] }
]
```

That is everything. The image carries no file, which offers the same: a
kind added to a profile's file later is not left out by a list that
forgot it. A coffeehouse's:

```json
[
  { "service": "actor-service", "node_types": ["person", "group"] },
  { "service": "text-service",  "node_types": ["note"] },
  { "service": "photo-service", "node_types": ["photo"] },
  { "service": "video-service", "enabled": false }
]
```

1. **A service it lists with `node_types`** is offered with those kinds
   alone. **One it lists with `"enabled": false`** is not offered at all.
   **One it does not list** is as each profile's file says.
2. **No file** is everything, as before: a site that has never heard of it
   changes nothing.
3. **`actor-service`'s kinds** are the kinds of profile that may be made:
   `group`, `event`, and `website`, which syndication-service makes. A
   person is always offered: it is an account.
4. graph-grpc-service reads it, as it reads the others; a site changes it
   by mounting its own over the image's, as a ConfigMap on Kubernetes. The
   other services ask graph-grpc-service (`ReadInstance`), so there is one
   file in one place.

## What follows from it

1. **Every profile has only what the instance offers.** After a profile's
   features are merged (its kind's file, then its own choices), a service
   the instance does not offer is taken out, and of one it offers, the
   kinds, composers and add permissions of a kind it does not. As video is
   taken out now where `VIDEO_UPLOADS` is not on, so nothing a profile has
   saved brings one back. Posting is checked against these features
   (`CanPostTo`), and the composers the web app is sent come from the same
   check: a kind that is off is neither offered nor taken.
2. **A kind of profile that is off is not made:** actor-service refuses a
   group or an event, and syndication-service a website, with 403
   `{"error": "off"}`, whatever `GROUPS_FROM`, `EVENTS_FROM` or
   `WEBSITES_FROM` say.
3. **What is already there stays,** readable at its address: a site that
   turns articles off keeps those written before. Nothing is deleted.
4. **NodeInfo says what is offered,** `metadata.offers`: `{ "profiles":
   ["person", "group"], "posts": ["note", "photo"] }`. The web app follows
   it: the menu, the routes, the search and feed tabs, and the buttons
   that make a group, an event or a website.
5. **Posts from other servers** are shown as they are: an article from
   elsewhere still shows in a feed. This is about what is made here.

## Video and sound

`VIDEO_UPLOADS` and `AUDIO_UPLOADS` stay, and go together with the file:
video is offered where the file offers it **and** `VIDEO_UPLOADS` is on,
sound the same with `AUDIO_UPLOADS`. A site that set them changes nothing.
Moving them into the file alone is for a later minor release.

## Out of scope

1. Titles and descriptions of what a site offers, for an About page: when
   something shows them.
2. Who may make what, site-wide (`add_permissions` on the instance): the
   `*_FROM` settings say it for profiles.
3. How lists are laid out, a site's home page and themes: the rest of row
   139.

## Phases

1. **The services:** the file, the cut after the merge, `ReadInstance`,
   the refusals, NodeInfo, docs. About a day.
2. **The web app** following NodeInfo. Half a day to a day.
3. **The `coffeehouse` recipe,** with its `instance.json` and its diagram.
   Half a session.

## Status

Plan written on 2026-11-09, and its first two phases, the services and the web app, built the same night; the coffeehouse recipe follows.
