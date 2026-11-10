# What a site offers

Anahita is small services, each doing one thing, and a site runs and shows
only what it offers. One file says what that is: `instance.json`, beside
the files that say what each kind of profile has. An instance is one big
profile, and every profile on it has only what it has. A site of notes and
photos alone, for example, offers no articles, topics, events, websites or
videos: they are in no composer, menu or tab, and the services refuse them.

Built for the recipes, roadmap row 139 ([the plan](plans/instance-features.md)).

## Contents

1. [The file](#the-file)
2. [What follows from it](#what-follows-from-it)
3. [Video and sound](#video-and-sound)
4. [Giving a site its own](#giving-a-site-its-own)
5. [Where the code is](#where-the-code-is)

## The file

`instance.json` in the directory `ACTOR_FEATURES_PATH` names,
`/app/defaults/actor_features` in graph-grpc-service's image, beside
`default.json`, `person.json`, `group.json`, `event.json` and
`website.json` ([Permissions](permissions.md#the-defaults)). It is in their
form: a list of services, each with the kinds it offers.

```json
[
  { "service": "actor-service", "node_types": ["person", "group"] },
  { "service": "text-service",  "node_types": ["note"] },
  { "service": "photo-service", "node_types": ["photo"] },
  { "service": "video-service", "enabled": false }
]
```

That is a site of people and groups, notes and photos.

| In the file | Means |
| --- | --- |
| A service with `node_types` | That service, with those kinds alone |
| A service with `"enabled": false` | Not that service at all |
| A service not listed | As each kind of profile's own file says |
| No file | Everything, as before it was read. The image carries none |

`actor-service`'s kinds are the kinds of profile that may be made: `group`,
`event`, and `website`, which syndication-service makes. A person is always
offered: it is an account. The kinds of post are `note`, `article` and
`topic` (text-service), `photo` (photo-service), and `video` and `audio`
(video-service).

A file that cannot be read is said in graph-grpc-service's log, and offers
everything, as no file does.

## What follows from it

1. **Every profile has only what the instance offers,** whatever its kind's
   file or its own saved choices say: its composers and tabs, and who may
   add each kind, come from those features, so a kind that is off is
   neither shown nor taken.
2. **The services refuse what is off,** with 403 `{"error": "off"}`:
   actor-service a group or an event, syndication-service a website,
   text-service a note, an article or a topic, photo-service a photo, and
   video-service a video or a sound. graph-grpc-service refuses to save a
   post of a kind that is off, whichever service asks, as a last check.
   `GROUPS_FROM`, `EVENTS_FROM` and `WEBSITES_FROM` still say who may make
   one where it is offered.
3. **What is already there stays,** readable at its address: a site that
   turns articles off keeps those written before.
4. **Posts from other servers are shown as they are:** an article from the
   fediverse still shows in a feed. This is about what is made here.
5. **NodeInfo says what is offered,** in `metadata.offers`:
   `{"profiles": ["person", "group"], "posts": ["note", "photo"]}`. The web
   app follows it: its menu (Groups, Events, Websites), a group's Events
   tab, the recommendations' tabs, and the buttons that make a group, an
   event or a website.

## Video and sound

`VIDEO_UPLOADS` and `AUDIO_UPLOADS` go together with the file: video is
offered where the file offers it **and** `VIDEO_UPLOADS` is `on`, sound the
same with `AUDIO_UPLOADS` ([Configuration](configuration.md#video)). A
site that set them changes nothing. NodeInfo's `videoUploads` and
`audioUploads` say both together.

## Giving a site its own

graph-grpc-service alone reads the file; every other service asks it
(`ReadInstance`, on the graph's gRPC service), so there is one file in one
place.

1. **docker-compose:** `graph-grpc-defaults/actor_features/` is mounted
   into graph-grpc-service; put `instance.json` there. It is read again
   for each request: no restart.
2. **Kubernetes:** mount a ConfigMap of the file over
   `/app/defaults/actor_features/instance.json` in graph-grpc-service's
   Deployment, with `subPath`, so that the image's other files stay. A
   recipe does this in its overlay.

## Where the code is

| What | Where |
| --- | --- |
| Reading the file, cutting a profile's features, what is offered | `graph-grpc-service/data/actor_features/instance.go`, `merger.go` (`ForSite`) |
| `ReadInstance`, and the last check on saving a post | `graph-grpc-service/cmd/server/handlers/instance.go` |
| Asking, and refusing with `off` | `anahita-libs/data/services/graph_grpc.go` (`ReadInstance`, `Offered`), `anahita-libs/handlers/offered.go` |
| NodeInfo's `offers` | `auth-service/cmd/api/handlers/node_info.go` |
| The web app | `src/utils/offers.js`, `src/permissions/actor.js` (anahita-web) |
