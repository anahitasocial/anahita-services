# Architecture

[Documentation](README.md) › Architecture

How the services, the gateway and the shared library fit together.

## Contents

- [Components](#components)
- [Services](#services)
- [Routing](#routing)
- [One service for every kind of actor](#one-service-for-every-kind-of-actor)
- [Naming images](#naming-images)
- [anahita-libs](#anahita-libs)
- [What a node is called elsewhere](#what-a-node-is-called-elsewhere)
- [Naming dates and times](#naming-dates-and-times)
- [The database](#the-database)

## Components

Three kinds of components:

**HTTP services** (JSON, behind nginx) own an API resource family. They do
not talk to each other over HTTP.

**gRPC services** are shared infrastructure the HTTP services call
directly: `graph-grpc-service` owns the node/edge graph and every read
against it, `file-grpc-service` owns file storage, `mail-grpc-service`
owns outbound email. These calls use mutual TLS: each side presents a
certificate from the installation's own authority, so a gRPC service answers
only the other services (`anahita-libs/grpctls`; see
[Internal calls](configuration.md#internal-calls)).

**RabbitMQ** carries asynchronous fan-out. Services publish domain events
to the `anahita_events` exchange; consumers (notifications, feed, search)
react. Event names are versioned constants in `anahita-libs/event`, shaped
`node.<type>.<service>.<entity>.<action>.v1` and
`edge.<type>.<service>.<action>.v1`.

## Services

| Service | HTTP port | gRPC port | Responsibility |
| --- | --- | --- | --- |
| `nginx` | 80 | — | Gateway, routing, CORS, token validation subrequest |
| `auth-service` | 8081 | — | OAuth 2.0 / OIDC provider, server-rendered sign-in and sign-up, email verification, registration applications, the administration list of accounts, agreements, password, TOTP, WebAuthn, invites, clients, NodeInfo |
| `actor-service` | 8082 | — | The profiles of people, of groups and of events. One service for every kind of actor; see [below](#one-service-for-every-kind-of-actor) |
| `text-service` | 8084 | — | Text media: articles, notes, topics |
| `photo-service` | 8085 | — | Photos |
| `hashtag-service` | 8086 | — | Hashtags |
| `location-service` | 8087 | — | Locations and geocoding |
| `notification-service` | 8091 | 50057 | Notifications |
| `socialgraph-service` | 8092 | — | Follow, block, subscribe relationships |
| `search-service` | 8093 | — | Search |
| `like-service` | 8095 | 50058 | Votes / likes |
| `identity-image-service` | 8096 | — | The avatars and covers of profiles and posts; see [Naming images](#naming-images) |
| `feed-service` | 8098 | 50059 | Feeds and reposts |
| `recommendation-service` | 8100 | | Suggestions of who to follow. Optional |
| `link-preview-service` | 8101 | — | Fetches the page a link points to and keeps its title and picture, for the card on a note. The only service that connects to addresses people type. Optional. See [Link previews](link-previews.md) |
| `syndication-service` | 8107 | — | Connecting a website: finds its feed and checks its domain's proof; takes WebSub hubs' calls. Goes only to public addresses. Optional. See [Websites](websites.md) |
| `syndication-worker` | — | — | Reads every website's feed and posts what is new; looks at every proof again daily. Answers no requests |
| `video-service` | 8103 | — | Videos: uploads sent straight to storage, and the posts made of them. Optional. See [Videos](videos.md) |
| `video-worker` | — | — | Makes the sizes of each uploaded video with ffmpeg, then the post. Answers no requests |
| `export-service` | 8104 | — | A person's copy of their own data: asking for one and fetching it. Optional. See [Download your data](data-export.md) |
| `export-worker` | — | — | Makes each archive asked for, and deletes the ones kept their time. Answers no requests |
| `web-shell` | 8102 | — | Sends the web app, with each page's own title, description and picture written into `index.html`, so a shared link shows as itself. On the web app's host, not the API's. Optional. See [The web shell](web-shell.md) |
| `atproto-service` | 8106 | — | Bluesky: accounts there are looked up and followed from here, and their posts kept as they are written; for whoever connects their own Bluesky account, their public posts, replies, likes, reposts and follows are written there as them. The one service that holds a connection open to the outside. Optional. See [Bluesky](federation.md#bluesky) |
| `activitypub-service` | 8105 | — | Makes the installation a server of the fediverse: people who choose to be found, and their posts, replies, likes, reposts and follows both ways; also the list of servers the site refuses. The one service that reads what other servers send. Optional. See [Federation](federation.md) |
| `abuse-report-service` | 8099 | — | Reports of people and posts, and the cases administrators decide. See [Abuse reports](abuse-reports.md) |
| `graph-grpc-service` | — | 50054 | Node/edge graph: actors, media, edges, traversal, trending |
| `file-grpc-service` | — | 50055 | File storage on any S3-compatible object store; checks what each upload really is, and signs the addresses large files are sent to directly |
| `mail-grpc-service` | — | 50056 | Email delivery over SMTP (50051 inside its container) |
| `migration-job` | — | — | One-shot database migrations |
| `account-sweeper` | — | — | What is done by the clock, hourly: purges accounts past their 30-day deletion grace period, and finishes purges a super administrator asked for; removes images uploaded for a post that was never made, and invitations nobody answered; reminds people of an event a day before, and archives one thirty days after it ended. Not started by `up`; run with `docker compose run --rm account-sweeper`. See [Account lifecycle](account-lifecycle.md) |

Backing services in `docker-compose.yml`:

- **MySQL** (3306) — all persistent data
- **Redis** (6379) — sessions
- **RabbitMQ** (5672) — the `anahita_events` exchange
- **Garage** (3900) — object storage, S3-compatible, for local development

## Routing

nginx maps public paths to services. The full map lives in
`nginx/nginx.conf`, and the same gateway runs in docker-compose and on
Kubernetes, where the Ingress sends everything to it
(see [Deploying](deploying.md)):

| Path prefix | Service |
| --- | --- |
| `/oauth/`, `/auth`, `/signup`, `/signup-requests`, `/accounts`, `/agreements`, `/onboarding`, `/password`, `/email`, `/username`, `/reauth`, `/totp`, `/webauthn`, `/invites`, `/clients`, `/auth-logs/`, `/oauth-signing-keys`, `/.well-known/`, `/nodeinfo/`, `/static/` | `auth-service` |
| `/articles/`, `/notes/`, `/topics/`, `/bookmarks/` | `text-service` |
| `/people/`, `/groups/`, `/events/` | `actor-service` |
| `/photos/` | `photo-service` |
| `/videos/` | `video-service` |
| `/.well-known/webfinger`, `/.well-known/host-meta`, `/inbox`, `/ap/`, `/activitypub/` | `activitypub-service` |
| `/atproto/` | `atproto-service` |
| `/exports` | `export-service` |
| `/avatars/`, `/covers/` | `identity-image-service` |
| `/likes/` | `like-service` |
| `/hashtags/` | `hashtag-service` |
| `/locations/` | `location-service` |
| `/feeds/`, `/reposts/` | `feed-service` |
| `/recommendations/` | `recommendation-service` |
| `/link-previews` | `link-preview-service` |
| `/websites/_connect`, `/websites/_websub/` | `syndication-service` (`/websites/_websub/` not signed in) |
| `/websites/` | `actor-service` |
| `/notifications/` | `notification-service` |
| `/abuse-reports` | `abuse-report-service` |
| `/socialgraph/` | `socialgraph-service` |
| `/search/` | `search-service` |

`/internal/oauth/validate` is an internal-only subrequest target used by
nginx to validate bearer tokens against `auth-service`. The web app keeps its
tokens server-side behind a session cookie; for the locations that need it,
nginx asks auth-service to translate that cookie into a `Bearer` token before
proxying, so the services only ever see bearer tokens.

A list of posts is always one profile's: `GET /notes/<owner id>/`, and the same
for `/articles/`, `/topics/` and `/photos/`. There is no list of every post of
a type across the site, and a request without an owner is refused. Posts are
discovered through feeds, search, hashtags and profiles.

### No stories

There is no story service and there are no stories, and nothing records who
looked at what. A story service existed as a stub and was removed; so, after
being built, was a row of "who posted lately" on the home page, which a new
installation with few people on it has no use for yet. What may come later is
in the web app's roadmap, under Kept for later.

### Custom feeds

A custom feed is a node of type `node.feed.feed-service.generator.v1`, owned
by whoever made it, with what it shows as JSON in the node's `meta`. Keeping
one on the home page is an edge, `edge.feed.pin.v1`. feed-service serves them
(`POST /feeds/`, `GET /feeds/<id>/items` and the rest), and reads a feed's
posts from the graph over `FeedCustomService.Browse`, which builds its query
from the definition and gates it for whoever is reading. The definition and
its rules are in `anahita-libs/feeds`, used on both sides. See
[Custom feeds](feeds.md).

### Pinned posts

`nodes.pinned` says a post is pinned to the profile it is on, and
`nodes.pinned_at` when, which is the order pins are shown in. A profile's
posts (`GET /feeds/actor/<id>/`) and its lists of one kind
(`GET /notes/<owner id>/` and the like) are led by what is pinned: one
ordering over the whole list, so a pinned post is on the first page and on no
other. A profile has `constants.MaxPinnedPosts`, one. The rules are in
[Permissions](permissions.md#pinned-posts).

An ordinary edit of a post neither pins nor unpins it. That is done through
the pin routes alone, which keep the limit and the date.

### Quote posts

A quote is a note with `quote_id`, the post it quotes. Nothing else about it
is different from a note: it is made with `POST /notes/<owner id>/`, listed in
feeds and on profiles as a note, and gated as one. The rules are in
[Permissions](permissions.md#quotes).

| On `nodes` | Holds |
| --- | --- |
| `quote_id` | On the quoting note: the post it quotes. Set once, when the note is made |
| `quote_detached_at` | On the quoting note: when the quoted post's author took their post out |
| `quote_policy` | Who may quote: on a post, that post; on a person, their posts when a post does not say |
| `quote_count` | On the quoted post: the notes that still quote it. Counted over the rows, on every quote, removal and detach, and after a purge |

- **A note that quotes is sent `quote`**: `{ "post": … }` when the reader may
  see the quoted post, or `{ "state": "unavailable" | "detached" }`. Each
  service that sends notes fills it in with `handlers.SetQuotes`, which first
  asks the graph which of the quoted posts this reader may see
  (`FilterVisible`, the gate every listing uses) and reads only those.
  Reading one post through the graph checks nothing: it is the filter that
  decides. A response whose handler
  did not look the post up carries `quote_id` and no `quote` at all.
- **`authorized.quote`** on a post says whether the viewer may quote it. In a
  list it is exact for a post on its author's own profile; for a post
  somebody wrote in a group it does not know the author's own setting, and
  making the quote asks again with everything.
- **Routes**: `PATCH /{articles,notes,topics,photos}/<id>/quote-policy`,
  `PATCH /people/<id>/quote-policy`, `DELETE /notes/<id>/quote`, and
  `GET /notes/<id>/quotes` for the notes that quote a post.

### Embeds and citations

An article or a topic shows what an address on a line of its own is to: a
post here as itself, a page elsewhere by its card. An address inside a
sentence stays a link. The text stays plain Markdown, so other servers are
sent the link it is. How a line is read is `anahita-libs/embeds`, and the
web app reads it the same way.

- **A post here** is not copied into the article. The reader's browser asks
  for it as for any post, so somebody who may not see it gets the link and
  nothing more.
- **A citation** is the edge from the article to each post here it shows
  (`edge.medium.citation.v1`). text-service writes the citations when the
  article or topic is added or edited, and takes away those whose line is
  gone. An address that could not be read, or is not a post, is not cited.
- **A mention** is a post naming a profile, a person, a group or an event:
  by its `@handle`, or by its address on a line of its own, which shows its
  card. It is the edge from the profile to the post (`edge.tag.mention.v1`),
  kept by actor-service; see [Permissions](permissions.md#mentions).
- **Who cites what.** Each post and profile keeps how many people cite it,
  a profile counting the citations of its posts,
  `nodes.citation_count`, which [Search](search.md) and
  [Recommendations](recommendations.md) weigh. `GET /notes/:id/cited_by`
  lists the posts citing a post of any kind, newest first, as the viewer
  may see them; the web app shows them in a post's Cited by tab, beside
  Replies and Locations. What an article or a topic cites is listed at the
  end of its own text.
- **A page elsewhere** is shown by its card, kept in the post's `meta` under
  `link_previews` and sent as `link_previews`: see
  [Link previews](link-previews.md#cards-in-long-writing).

### The media library

A photo or video post can be in its author's media library only
(`nodes.library_only`): put up from inside the article editor, to be shown
in an article. It is read by its address with the access any post has, so
the access gates do not know of it; the listings leave it out, each by
`listedQuery` (graph-grpc-service `data/repositories/listed.go`): feeds, a
profile's lists, search, a hashtag's and a location's posts. It cannot be
pinned. Nothing announces it: the graph publishes no event for it, so no
notification, hashtag or mention is made of it, and neither federation
service sends it, its sweeper included.

- **Made** with `library_only: true` on `POST /photos/<own id>/` or when a
  video's upload begins (`video_uploads.library_only`, carried to the post
  by the worker). Only on one's own profile; never a reply. Set once.
- **Listed** by `GET /photos/<id>/?library=1` and `/videos/<id>/?library=1`:
  all of the viewer's own photos or videos on their own profile, these
  with them. Nobody else's.
- A post saved before it was in a library, or quoted, is still found
  where it was: those lists are the viewer's own doing.

### Saved posts

Somebody can save a post, of any kind, to find it again: `PUT` and
`DELETE /bookmarks/<id>`, and `GET /bookmarks/` for their own list, most
recently saved first. A post they have saved carries `is_saved_by_viewer`.

- **Private.** A saved post is an edge from the person to the post
  (`edge.base.bookmark.v1`), and every question about it is asked with the
  asker's own id. Nobody can ask what somebody else saved or who saved a
  post; nothing is counted on the post, no event is published, and its author
  is not told.
- **Saving does not keep a post open.** The list goes through the gate a post
  is read through. A post saved while public and since made private, or whose
  author has since blocked the person, is left out, and is back if it is
  opened up again. It can always be taken out of the list.
- **No folders.** One list.

### A post's activity

Who liked a post, reposted it, quoted it and replied to it is four lists
that already exist or follow from what does:

| List | From |
| --- | --- |
| Likes | `GET /likes/<id>/` |
| Reposts | `GET /reposts/<id>/` |
| Quotes | `GET /notes/<id>/quotes` |
| Replies | `GET /notes/<id>/thread` |

Each is for anybody who can see the post, and each lists only the people and
notes that reader may see, so a list can be shorter than the count on the
post. There is no count of views and no list of who looked: nothing records
that. The plan that had both (`plans/post-insights.md`) was set aside for
this.

### A profile's three lists

`GET /feeds/actor/<actor id>/` is everything a profile shows, in one of three
lists chosen with `filter`. Replies and reposts are a person's: only people
reply and repost, so for a group or any other kind of actor the last two are
empty.

| `filter` | Holds |
| --- | --- |
| `posts` (the default) | What was posted on the profile. Also what its owner reposted, when they have asked for that. Not replies |
| `replies` | The replies its owner wrote, wherever the threads are. Each is sent with what it answers as `parent`, and with `root`: the post at the top of its thread and, as `root.owner`, the profile that thread is on |
| `reposts` | What its owner reposted, and nothing else. Each is sent with the post it reposts as `parent`, with that post's author, owner and date |

- **Reposts are not among the posts unless the profile asks for it.** The
  choice is kept on the actor as `show_reposts_on_profile`, off until somebody
  turns it on, and changed with `PATCH /people/<id>/reposts-on-profile` by
  whoever may edit the profile. There is no such route for a group. A
  caller cannot ask for them: the feed reads the profile and decides.
  The choice is about the profile only. Followers are sent reposts in their
  own feeds either way, and the list of reposts always has them.
- **A reply is listed only to somebody who can see the post at the top of its
  thread**, like everywhere else a reply turns up. Not one that was removed,
  and not one that was hidden from its thread.
- **On a site that shows visitors a preview**, the list of replies answers a
  visitor 401 `members_only`: replies are not part of a preview.

A reply read on its own (`GET /notes/<id>`) carries `root` too, with its
owner, so a page can say what it is a reply to and whose profile it is on. The
thread is the root's: `GET /notes/<root id>/thread`.

**A thread written in one go** is up to ten notes made together, in order,
all or none (roadmap row 101): `POST /notes/<owner id>/thread` begins a post,
`POST /notes/<id>/replies/thread` answers one. Each note after the first is a
reply to the one before. graph-grpc-service writes them in one transaction
(`Medium.AddThread`) and announces them in order once all are made, each
with its place in the thread (`part` in the event).

A photo post holds up to twenty images (`PHOTO_MAX_FILES`), each uploaded in a request of its own
and kept in `photo_files`, a table beside the post. How that works, and what
is stored for each image, is in [Photo posts](photos.md).

## One service for every kind of actor

People and groups are both actors: each has a profile, an avatar and a cover,
followers, an audience, permissions, and the same life from enabled to
archived to deleted to purged. They were served by two services,
`person-service` and `group-service`, that were the same service written
twice, and a fix made in one was regularly missing from the other. They are
now one, `actor-service`.

- **Each kind keeps its own address.** `/people/` and `/groups/` are
  unchanged, and the gateway sends both to `actor-service`. A client cannot
  tell that anything moved.
- **The old names are still in the data, as names.** A person's type is
  `node.actor.person-service.person.v1` and a group's
  `node.actor.group-service.group.v1`. Those strings are in every node and
  edge ever stored and in where avatars and covers are kept, so they stay.
  They no longer say where anything runs.
- **What is shared is written once:** signing in the viewer, step-up before a
  deletion, presigning images, the answers for a refused change, the
  configuration, the start-up. **What differs stays apart:** a person is read
  by username and cannot be created here (that is signing up, in
  `auth-service`); a group can be created and restored and has
  administrators.
- **It has no database address.** It reads and writes through
  `graph-grpc-service` and owns no tables, so it is given no database
  credentials.
- **A new kind of actor** is a handler and its routes in this service, not a
  service of its own. The website, a website's feed as a profile, is built
  that way ([plans/done/website.md](plans/done/website.md)).
- **An event is one.** Its type is `node.actor.actor-service.event.v1`, it
  is served at `/events/` by the handler a group has, told a different kind,
  and what only an event has is beside it: answering going or maybe, calling
  it off, and its calendar file. Its start and end are on the node; its time
  zone, how many may go, where to join online and when it was called off are
  in the `events` table. A group hosts one through an
  `edge.actor.host.v1` edge from the group to the event, and
  `GET /groups/<id>/events/` lists them. Who may do what is in
  [Permissions](permissions.md#events).
- **An invitation is an edge,** `edge.actor.invite.v1`, from the person
  invited to the group or event, so every kind of actor can have them. They
  are made and answered through `socialgraph-service`
  (`/socialgraph/<actor>/invites` and its `accept` and `decline`, and
  `/socialgraph/invites/` for the ones waiting for the viewer), and each is
  announced on the bus so that the notification for it can be answered where
  it is read. The rules are in [Permissions](permissions.md#invitations).

## Naming images

There are three kinds of image, and the names for them have not always
lined up. This is what each is, and what it is called where.

| What it is | Who has one | Column | In the API | Served by |
| --- | --- | --- | --- | --- |
| **Avatar**: the picture that stands for an actor | People, groups | `nodes.avatar_filename` | `avatar_urls` | `identity-image-service`, at `/avatars/` |
| **Cover**: the wide picture across the top | People, groups, and posts such as articles | `nodes.cover_filename` | `cover_urls` | `identity-image-service`, at `/covers/` |
| **A photo post's images**: the photo itself | Photo posts | `photo_files`, with the first copied to `nodes.filename` | `files`, and `portrait_urls` for the first | `photo-service`; see [Photo posts](photos.md) |

- **Avatars and covers are identity images:** what a profile or a post is
  recognised by. A photo post's images are content. That is the line between
  the two services.
- **`nodes.filename` is the file a node *is*:** a photo post's photo today,
  a document's file if file sharing is built. An actor has none. Until
  2026-10-05 the same column also held an actor's avatar, so its meaning
  depended on the kind of node; the avatar now has `avatar_filename`, beside
  `cover_filename`, and each of the three names one thing.
- **"Portrait"** in the code and in `portrait_urls` means a photo post's
  image, not a picture of a person. It is not to be used for anything new,
  and is to be retired. That is a change to the API and the web app, not
  to the schema, and was left out when the schema was put in order
  ([plans/done/migration-baseline.md](plans/done/migration-baseline.md)).
- **"Avatar"** stays, in the code and the API. It is the word Mastodon and
  Bluesky use for the same field.
- **Where they are stored** is named for the service that looks after
  them: `assets/v1/identity-image-service/avatars/person/<id>/…` and
  `…/covers/…`. `v1` is the layout of the first release. The PHP
  application's (`assets/public`) is v0 in all but name, and while this
  platform was being built its files were under `assets/v2`, with avatars
  and covers in folders named `avatar-service` and `cover-service`.
  `scripts/s3-migration` moves a bucket from either.

Every uploaded image, of any of the three kinds, goes through one check
(`images.Accept` in `anahita-libs/utils`): it is judged from its bytes and
not its name, limited in bytes and in pixels before it is opened, and turned
the right way up. Nothing is stored as it arrived; each stored size is
written out again, never enlarged, and a square is cut from the middle and
not squeezed.

## anahita-libs

The shared module every service imports:

| Package | Contents |
| --- | --- |
| `constants` | OAuth scopes, shared enums |
| `proto` | Generated gRPC stubs: `graphpb`, `filepb`, `mailpb`, `typespb` |
| `event` | RabbitMQ connect/publish/consume, event name constants |
| `sessions` | Redis-backed sessions and cookies |
| `crypt` | Encryption with pluggable local / AWS KMS backends |
| `grpctls` | Mutual TLS for the calls between services |
| `ratelimit` | A counter in Redis: so many of something in a window, for each person |
| `safehttp` | An HTTP client for going where somebody else said to: public addresses only, decided at the moment of connecting. For link previews and for speaking to other servers |
| `policy` | The installation's settings on who may see what, which several services must agree on |
| `federation` | What the services that speak to other servers share: the text of a post there and back (Markdown to HTML that is safe to send, HTML that arrived to Markdown that is safe to keep); the queue of what is to be sent; copying pictures, avatars and covers in, removing them after a while and copying them back when asked (`media.go`); the cards of links; the limits on what is kept from elsewhere (`retention.go`); custom emoji; and the servers the site refuses |
| `site` | Where the installation is, as the internet sees it: its domain, and the addresses of its web app and API. See [What a node is called elsewhere](#what-a-node-is-called-elsewhere) |
| `language` | The language a post is written in |
| `feeds` | What a custom feed is made of, and the rules a definition keeps |
| `linkpreviews` | What a link's preview is, and the copy of one a note keeps |
| `videos` | What a video post is: its files and where they are |
| `data`, `requests`, `responses`, `validators` | Request/response shapes and validation |
| `middleware`, `cors` | HTTP middleware |
| `geocoding`, `utils`, `cmd` | Supporting helpers |

**The calls between services** are made by the clients in
`data/services` (`GraphNodeGRPC`, `MailGRPC` and the rest). A process
keeps one gRPC connection to each service it calls, made the first time
and shared by every client and every call (`data/services/connections.go`):
a connection is safe to share and reconnects by itself. Before roadmap row
98, each call opened a connection of its own, with a new handshake, and
left it open.

## What a node is called elsewhere

Inside the installation a node is its number, and a person is also their
username. Neither will do for another server. A number says how many came
before, and means nothing anywhere else; a username can be changed. What
another server is told a person or a post is called is kept there for good,
by servers this one may never hear from again.

- **Every node has a UUID,** `nodes.uuid`: random (version 4), unique, and
  never changed. The database gives it when the row is added, so nothing that
  adds a node can forget to, and the nodes there were before it were given
  one each when the column was added.
- **It is read with the node** (`Node.uuid` in `graphpb`, `UUID` on
  `entities.AnahitaNode`), and a node is found by it with the graph's
  `ReadByUUID`, which applies no access rule, as `Read` does not. It is not
  in what the API sends a browser: the web app has no use for it.
- **The addresses it goes into are made in one place,** `anahita-libs/site`,
  from one reading of the settings (`PUBLIC_DOMAIN`, `PUBLIC_WEB_URL`,
  `PUBLIC_API_URL`; see [Configuration](configuration.md#site-identity)). Two
  services that disagreed about the host would make one person two.
  `CanFederate` says whether other servers could reach those addresses at
  all: names on the internet, over https. A site on a developer's machine is
  neither.

- **What came from another server is a node too,** so that feeds, follows
  and blocks need nothing new. It has a `uri`, what it is called where it
  lives, and an `origin_domain`; a node without a `uri` was made here. A
  person elsewhere has a type of their own, one for each protocol
  (`node.actor.activitypub-service.person.v1`,
  `node.actor.atproto-service.person.v1`), so whatever lists people here
  leaves them out; a post of theirs is an ordinary note. Such nodes are
  written only by the graph's `UpsertRemote`, which refuses anything that is
  not a person elsewhere or a note of theirs, and anything under one of this
  installation's own addresses (`graph-grpc-service/data/repositories/nodes_remote.go`).

- **A public profile's pictures have an address that stays,**
  `/avatars/<uuid>/<size>` and `/covers/<uuid>/<size>` on the API, which
  sends whoever asks on to a freshly signed one. Another server keeps the
  address of an avatar for weeks; the signed one lasts a quarter of an hour.
  Only what somebody not signed in may see is answered for
  (`identity-image-service/cmd/api/handlers/public.go`).
- **What is to be sent to another server waits in a table** until it has
  been sent, or tried for two days (`anahita-libs/federation`, `queue.go`).
  The bus cannot promise that a message arrives, and another server may be
  down for an afternoon. Each protocol's service has a table of its own and
  any number of workers on it; a row is handed to one at a time.
- **The web app's host answers what other servers ask it under
  `/.well-known`** by sending them on to the API
  ([The web shell](web-shell.md)).

Federation is built on this; see [Federation](federation.md), and to try
it on one machine, [The federation lab](federation-lab.md).

## Naming dates and times

One rule, in the database, in the API and in the web app:

- A date **with a time** ends in `_at`: `created_at`, `updated_at`,
  `starts_at`, `ends_at`, `last_visited_at`, `deleted_at`.
- A calendar date **with no time** ends in `_on`. Nothing uses one yet.

The same name is used all the way through: the column `created_at` is the
field `created_at` in a response and `createdAt` in the web app, and a list is
sorted with `ordering=created_at`. The last change to a row is `updated_at`,
never `modified_at`.

A test, `TestTimestampColumnsEndInAt` in graph-grpc-service, reads
`migration-job/schema/init.sql` and fails on a date or time column that does
not follow.

## The database

### How it is made

`migration-job` runs once each time the stack starts, and does one of three
things:

| The database has | It runs | Then |
| --- | --- | --- |
| No tables | `schema/init.sql` | records goose's baseline |
| The PHP application's tables (`<prefix>_nodes`, `<prefix>_edges`) | `legacy/upgrade.sql` | records goose's baseline |
| This schema | `goose up`, the migrations in `migrations/` | |

`schema/init.sql` and `legacy/upgrade.sql` each make the whole schema, and
`make schema_test` checks that they make the same one: column by column and
index by index, on a throwaway MySQL. Until 2026-10-17 there were 70 goose
migrations as well; they were folded into the two files (roadmap row 27),
and `migrations/` holds only what is written after. How to change the schema
is in [`migration-job/migrations/README.md`](../migration-job/migrations/README.md).

What a site's own data needs beyond the upgrade, as anahita.io does on its
way to banu.io, is a file of its own in `legacy/sites/`, run by hand.

### NULL only where it means something

A column says what it holds, in every table. NULL is kept for five things,
and for nothing else:

| NULL means | Columns |
| --- | --- |
| No other row: a reference with nothing at its end | `owner_id`, `parent_id`, `root_id`, `quote_id`, `created_by`, `modified_by`; a notification's `subject_id`, `object_id`, `target_id` |
| Made here: no address on another server | `uri`, `origin_domain` |
| No point | `geo_latitude`, `geo_longitude`, NULL together |
| Not happened, or does not apply | `deleted_at`, `archived_at`, `pinned_at` and the like; `starts_at` and `ends_at`, which only an event has |
| None, where a unique index would make `''` collide | an account's `email`; the delivery queues' `dedupe_key` |

Everything else is `NOT NULL`: `''` for no text, `0` for no count, false for
a switch not set, and `created_at` and `updated_at` default to the time of
insert. A reference is never 0 for none, and a point never 0,0.

So the code reads a column as it is, and uses `COALESCE` only for:

- what may be NULL by the table above, read as `''` or `0`;
- a row from a `LEFT JOIN`, which may not be there;
- an aggregate over no rows (`SUM` gives NULL, not 0);
- a choice between two values (`COALESCE(root.access, note.access)`, the
  first date a domain was proved).

A date NULL for "never" is read as the zero time, not as now, so a check of
how recent it is fails closed (`optionalTime` in the graph and auth
services).

