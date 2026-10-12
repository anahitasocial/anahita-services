# Federation: ActivityPub and AT Protocol

*Roadmap item 8, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
We want local people to follow Mastodon and Bluesky accounts, those accounts to follow back, local posts to go out, and remote posts to show up in the leaders feed.

**Today:**
- There's no federation code. The one exception is NodeInfo 2.1 in auth-service (`auth-service/cmd/api/handlers/node_info.go`), which deliberately reports `protocols: []`.
- The design groundwork is decided but not built:
  - nodes get a **UUID**, used for federated ids and the DID (`docs/plans/actor-profile-management.md:498-545, 673-697`);
  - purged actors keep a tombstone and should answer with `410 Gone` (`nodes_purge.go:18-34`);
  - lifecycle states map to protocol behaviour (`done/actor-lifecycle-states.md:401-437`);
  - there's one alias namespace across all actor types, because of WebFinger.

**What fits well already:**
- **The leaders feed works as is.** It selects media whose `owner_id` is followed by the viewer (`graph-grpc-service/data/repositories/feed_leaders.go:130-200`). A remote note stored as a `note.v1` node owned by a remote actor node, with a local follow edge to that actor, appears in feeds with **no feed changes**.
- **Follow requests match the protocols.** Accepting rewrites a follow-request edge into a follow and publishes an event (`socialgraph_follow.go:40-88`), which maps directly onto ActivityPub `Follow` / `Accept`.
- **Anonymous reads of public profiles and notes already work** (`SetViewer` with viewer 0), so actor and object JSON can be served through graph-grpc.
- **Reusable pieces:**
  - encryption at rest: `anahita-libs/crypt` (the TOTP secrets precedent);
  - a key table to copy: the `oauth_signing_keys` shape;
  - the job pattern: the sweeper's advisory lock (`graph-grpc-service/sweeper/sweeper.go`).

**Decisions:**
- **Native on both protocols.** ActivityPub is built in Go. AT Protocol uses the official Bluesky PDS container plus Jetstream.
- **Per-person opt-in,** with separate toggles for ActivityPub and Bluesky. Only public profiles and public posts federate.

## Phase 0 — groundwork (needed by both protocols)

| # | Work | Why | Estimate |
| --- | --- | --- | --- |
| 0.1 | **Confirm the production hosting layout.** `anahita-web/docs/deploying.md` says the web app is on a static host (Amplify) and the API on `api.DOMAIN`, but `nginx.conf:307` says "same-origin production". | Decides where WebFinger, host-meta, `atproto-did` and content negotiation are served. **Settled by item 10:** the web app is served through our nginx plus the `web-shell`, on the same front as the API. | 0.5 h |
| 0.2 | **UUIDs on nodes** (already decided). A `uuid` column with a unique index, backfilled, and set on create. | Federated ids must not be alias- or integer-based | 3–4 h |
| 0.3 | **Public base URL config in `anahita-libs`:** `PUBLIC_DOMAIN` (the handle domain) and `PUBLIC_API_URL`. Only auth-service knows the domain today, and k8s sets none of it. | Every federated URL is built from these | 1 h |
| 0.4 | **Stable media URLs.** Media is private S3 with 15-minute presigned URLs (`file-grpc-service/data/services/aws_s3.go:180-226`). Add a public proxy, `GET /media/<key>` → 302 to a fresh presigned URL with cache headers. Also guard the presign step so absolute remote URLs pass through unchanged. | Remote servers cache avatar and attachment URLs | 2–3 h |
| 0.5 | **Remote identity in the data model:** see below | | 4–5 h |
| 0.6 | **Content conversion.** Outbound: Markdown → HTML (goldmark) sanitised with bluemonday. Inbound: HTML sanitised → Markdown (html-to-markdown). Server mention parsing that understands `@user@host` (`anahita-libs/data/entities/node_mention.go:5` captures only `user` today). | The web app renders bodies as Markdown (`NodeBody.jsx`); Mastodon sends HTML | 2–3 h |
| 0.7 | **Durable delivery queue:** see below | The RabbitMQ bus can lose events and delivers one copy per replica (`anahita-libs/event/event.go:24-35`, `consumer.go:65-84`) | 4–5 h |
| 0.8 | **Routing and the dev TLS lab:** a TLS listener in nginx for dev; `location = /.well-known/webfinger`, `= /.well-known/host-meta`, `/ap/`, `= /inbox` → activitypub-service, `/.well-known/atproto-did` on `*.DOMAIN` → atproto-service; and the apex rewrite from 0.1. Also fix `k8s/99-ingress.yml`, which is stale, has no TLS and already routes `/users` to auth-service. | Federation needs HTTPS on a stable hostname | 3–4 h |
| — | **Item 0 (feed gate fix) must be done.** | Restricted posts must never be served or federated | — |
| | **Phase 0 total** | | **about 20–25 h, 5–6 sessions** |

**0.5 in detail:**
- **New columns on `nodes`:** `uri` (the ActivityPub id or `at://` URI, with a unique index) and `origin_domain`.
- **New remote-actor types, one per protocol,** following the `node.<family>.<service>.<entity>.v1` convention: `node.actor.activitypub-service.person.v1` and `node.actor.atproto-service.person.v1`. The web app picks the badge from the type. Remote posts stay `node.medium.text-service.note.v1` (with `uri` set), so feeds need no changes; they are written by the protocol services through graph-grpc. Using separate actor types keeps remote actors out of local people browse, mention resolution, NodeInfo counts and admin queries automatically.
  - The web app treats it as person-like (`isPerson` and the feed's `ActorEntityTypes` include it).
  - Remote aliases are domain-qualified, for example `alice@mastodon.social` or `alice.bsky.social`. The `@` and `.` mean they can never collide with local aliases in `uq_unique_alias`.
- **New table `federation_actors`:**
  - `node_id`, `protocol` (`ap` | `atproto`), `uri` or `did`, `handle`;
  - `inbox`, `shared_inbox`, `public_key_pem`, `followers_url`, `fetched_at`.
- **New table `federation_edges`:** `edge_id`, `protocol`, `activity_id` or `record_uri` (plus `rkey`). This is needed for `Undo(Follow)` and for deleting follow records, because edges are hard-deleted today and carry no activity id.
- **New table `actor_keys`:**
  - `node_id`, `key_id`, `algorithm` (RSA-2048 for ActivityPub), `public_key`, `private_key_encrypted` (encrypted with `crypt`), `created_at`, `revoked_at`.

**0.7 in detail:**
- **Table `federation_deliveries`:** `id`, `protocol`, `payload_json`, `target` (an inbox URL, or the PDS for AT), `actor_id`, `attempts`, `next_attempt_at`, `status`, `last_error`.
- **Filling it:** each protocol service owns its own queue table (`activitypub_deliveries`, `atproto_writes`) with the same shape. The queue code is shared through `anahita-libs/federation`. Rows are enqueued by the service's event listener, plus a **reconciler** that sweeps recently changed public notes of opted-in people, so lost events are caught.
- **Draining it:** a single worker uses `SELECT … FOR UPDATE SKIP LOCKED`, with exponential backoff for up to about 48 hours.

## Service layout: two protocol services, one shared library
**Separate services: `activitypub-service` and `atproto-service`.** Existing services only get small hooks.

**Why separate services:**
- **The two protocols run differently.**
  - ActivityPub is HTTP both ways: an inbox that must answer quickly and can scale to several replicas, plus a push-delivery worker.
  - AT Protocol is a PDS client plus one long-lived Jetstream websocket that **must run as a single consumer**, with a saved cursor.
  - Mixing the two in one service forces the single-consumer constraint onto the inbox.
- **They take untrusted internet input.** Signature checks, HTML sanitising and remote fetches stay isolated from the core services, so a parser bug or a flood from a remote server can't take down text-service or actor-service.
- **Instances choose.** An instance runs one protocol by deploying only that service (and the PDS for AT). Turning a protocol off means stopping a container. The same swappable pattern as the recommendation and abuse-report services.
- **They change at different speeds.** Protocol churn (FEPs, new lexicons) stays out of the core services' release cycle.

**Shared code in `anahita-libs/federation`** (a package, not a service):
- `UpsertRemoteActor` / `UpsertRemoteNote` helpers, which call new graph-grpc RPCs keyed by `uri`;
- Markdown ↔ HTML conversion and sanitising;
- the `@user@host` mention parser;
- the durable queue worker (each service has its own table);
- the domain-blocklist check;
- access helpers for the rule "only public posts of opted-in people".

**What each service owns:**

| | activitypub-service | atproto-service |
| --- | --- | --- |
| Routes | `/.well-known/webfinger`, `/.well-known/host-meta`, `/ap/*`, `/inbox`, `/activitypub/lookup`, `/activitypub/settings` (opt-in), `/activitypub/blocks` (admin domain blocklist) | `/.well-known/atproto-did` on `*.DOMAIN`, `/atproto/lookup`, `/atproto/settings` (opt-in and handle) |
| Tables | `activitypub_accounts` (opt-in and key id), `actor_keys`, `activitypub_actors` (remote inbox, shared inbox and public key), `activitypub_deliveries`, `federation_domain_blocks` | `atproto_accounts` (opt-in, DID, handle, encrypted PDS credentials), `atproto_actors` (remote DID and handle), `atproto_writes`, `atproto_cursor` |
| Runs | HTTP (can have several replicas) + a delivery worker (single, with an advisory lock) | HTTP + the Jetstream consumer + the PDS writer (single replica) |
| Also deploys | — | the Bluesky PDS container at `pds.DOMAIN` |

- Edge-to-activity mapping (`federation_edges`) is split the same way: each service keeps its own ids.
- `federation_domain_blocks` lives in activitypub-service but is read by both through the shared package. For AT it matches on the handle or PDS host.

**Hooks in existing services** (small, and part of phase 0):
- **graph-grpc-service:** the `uuid` and `uri` columns, the two remote actor types, `UpsertRemoteActor` / `UpsertRemoteNote` / `ReadByURI` RPCs, and remote types included where person-like actors are expected (and excluded from local people browse and counts).
- **auth-service:** NodeInfo reports the protocols of the services that are actually running (config lists them) and excludes remote nodes from the counts.
- **file-grpc-service / nginx:** the stable media proxy (phase 0.4).
- **actor-service:** mention resolution skips remote types until phase 2.
- **text-service, socialgraph-service and feed-service: no changes.** Following a remote actor uses the existing follow endpoints, and the protocol services react to the follow events.

**Web:**
- The Fediverse settings section calls `/activitypub/settings` and `/atproto/settings`, and shows only the protocols whose service responds.
- The lookup box sends inputs with `@host` to `/activitypub/lookup` and bare handles (`alice.bsky.social`) to `/atproto/lookup`.

## Phase 1 — ActivityPub (new `activitypub-service`, Go)
Written by hand with small JSON structs and an HTTP-signature library (`github.com/superseriousbusiness/httpsig`, which GoToSocial uses) rather than a full ActivityStreams framework.

1. **Discovery:**
   - `/.well-known/webfinger` resolves `acct:alias@PUBLIC_DOMAIN`, only for people who opted in.
   - NodeInfo stays in auth-service, and `protocols` becomes `["activitypub"]` once inbound works.
   - NodeInfo counts must exclude remote actors and remote posts (`auth-service/data/repositories/users.go:655-700` counts every enabled person and every medium today).
2. **Actor JSON** at `/ap/users/<uuid>`:
   - a `Person` with `preferredUsername` = alias, and `url` = the web profile URL;
   - `inbox`, `outbox`, `followers`, `following`;
   - `publicKey`;
   - the avatar via the 0.4 proxy.

   Also:
   - a purged actor answers `410 Gone` with a Tombstone;
   - a disabled, archived or opted-out actor answers 404 or 410;
   - `Accept: application/activity+json` on profile URLs redirects to the actor.
3. **Objects:**
   - `/ap/notes/<uuid>` serves a `Note` (HTML content, `published`, `attributedTo`, `to: Public`, `cc: followers`).
   - The outbox is an `OrderedCollection` of recent `Create`s.
   - The followers and following collections show totals only.
4. **HTTP Signatures:**
   - Sign every outbound POST (and GET) with the actor's key. A per-instance actor, `/ap/actor`, signs fetches.
   - Verify every inbound POST: fetch and cache the key, check the `Digest`, and reject old `Date`s.
5. **Inbox** (per user and shared `/inbox`). Handled activities:

   | Activity | What Anahita does |
   | --- | --- |
   | `Follow` | Creates the remote actor node if needed. For a public local profile without `allow_follow_request`, creates the follow edge and sends `Accept`. Otherwise creates a follow-request edge; when the local person approves, the existing follow-request event triggers `Accept`, and a rejection sends `Reject`. |
   | `Undo(Follow)` | Deletes the edge |
   | `Accept` / `Reject` of our Follow | Turns the pending follow-request edge into a follow, or deletes it |
   | `Create(Note)` | Stored **only** from actors that some local person follows (or that mention a local person; that part is phase 2) |
   | `Update` | Profile or note changes |
   | `Delete` | Actor or note, which becomes a tombstone locally |
   | Anything else | Ignored, answered with 202 |
6. **Outbound:**
   - A local follow of a remote actor creates a follow-request edge and sends `Follow`.
   - A local unfollow sends `Undo(Follow)`.
   - A new, edited or deleted public note of an opted-in person sends `Create`, `Update` or `Delete` to followers' inboxes, deduplicated by shared inbox.
   - A profile edit sends `Update(Person)`.
   - Opting out sends `Delete(Person)`.
7. **Looking up remote accounts:**
   - `GET /activitypub/lookup?q=@alice@mastodon.social` runs WebFinger, fetches the actor and creates or refreshes the remote actor node.
   - On first follow, backfill the last 20 posts from the outbox.
8. **Safety, minimal:**
   - Blocking a remote actor works through the existing block edge; nothing is delivered to them and their posts are dropped.
   - An admin **domain blocklist** (a table plus a Settings tab in the item 4a admin area) rejects inbound activity from blocked domains and stops deliveries to them.
9. **Tests** run against the local lab (below).

**ActivityPub estimate:**

| Part | Estimate |
| --- | --- |
| Discovery, actor, objects, collections | 4–5 h |
| Signatures and keys | 4–5 h |
| Inbox handlers | 6–8 h |
| Outbound delivery | 4–5 h |
| Lookup and backfill | 3–4 h |
| Domain blocklist | 2 h |
| Lab tests | 4–6 h |
| **Total** | **about 27–35 h, 7–9 sessions** |

## Phase 1 — AT Protocol (native: PDS + Jetstream)
1. **Run the official PDS** (`ghcr.io/bluesky-social/pds`, SQLite, lightweight) at `pds.DOMAIN`, alongside the stack in compose and k8s.
2. **Provisioning on opt-in:**
   - atproto-service creates the person's PDS account through the PDS admin API, then stores the DID and credentials encrypted with `crypt`.
   - Their handle is `alias.DOMAIN`: wildcard DNS plus nginx `server_name *.DOMAIN` serving `/.well-known/atproto-did` from atproto-service.
   - **Handle rules:** handles are DNS labels, so aliases are lowercased and `_` becomes `-`. On a collision, or if the result isn't a valid label, opt-in explains why and offers `alias.pds.DOMAIN` instead.
   - Only public profiles can opt in, because Bluesky has no follow approval.
3. **Outbound:** atproto-service writes records to the PDS through XRPC.
   - Profile: `app.bsky.actor.profile` (display name, bio, avatar as an uploaded blob).
   - Notes: `app.bsky.feed.post`, with facets for links, mentions and hashtags.
     - Bluesky posts are limited to **300 graphemes**. Longer notes are cut off and end with a link to the note on Anahita.
     - Bluesky takes images of at most **2 MB each**: 4 in `app.bsky.embed.images`, and more in `app.bsky.embed.gallery`, which the check on 2026-10-04 missed; see [How many photos a post can hold](#open-how-many-photos-a-post-can-hold-noted-2026-10-05). Mastodon takes 4 of 16 MB. So a photo sent to Bluesky is the largest stored size that fits in 2 MB, re-encoded smaller if none does.
   - Follows: `app.bsky.graph.follow`. Unfollow and delete become `deleteRecord`.
   - Record URIs and rkeys go in `federation_edges`, or in node `uri` for posts.
4. **Inbound:** a single Jetstream consumer (`wss://jetstream*.bsky.network/subscribe`).
   - It subscribes to `wantedCollections=app.bsky.feed.post,app.bsky.graph.follow`, and `wantedDids` = the Bluesky accounts local people follow plus our own DIDs.
   - Posts by followed accounts become remote notes.
   - `app.bsky.graph.follow` records whose subject is one of our DIDs become follow edges from the remote actor to the local person ("they follow back"). Deleted follow records remove the edge.
   - The cursor is saved, so a restart resumes where it stopped.
5. **Looking up remote accounts:**
   - Resolve the handle to a DID, then read the profile from the public AppView (`app.bsky.actor.getProfile`).
   - On first follow, backfill 20 posts with `getAuthorFeed`.
   - The same lookup box handles `alice.bsky.social` and `@alice@host`.
6. **Tests:** reading from the real Jetstream works locally. Outbound needs a public PDS (see the lab).

**AT Protocol estimate:**

| Part | Estimate |
| --- | --- |
| PDS deploy, provisioning, handles | 6–8 h |
| Outbound records (with the 300-grapheme cut and facets) | 5–6 h |
| Jetstream consumer + cursor | 6–8 h |
| Lookup and backfill | 3–4 h |
| Tests | 3–4 h |
| **Total** | **about 23–30 h, 6–8 sessions** |

## Web (anahita-web), shared by both protocols

| Part | Estimate |
| --- | --- |
| **Settings → "Fediverse" section:** separate toggles for ActivityPub and Bluesky; shows your handles (`@alice@DOMAIN`, `alice.DOMAIN`); explains that posts sent out can't be fully recalled | 2 h |
| **Remote lookup box** in search, then a remote profile page with a Follow button (pending until accepted) | 2–3 h |
| **Remote badges** on actors and posts (a Mastodon/ActivityPub or Bluesky icon plus the domain) and "View original" links | 1–2 h |
| **Remote posts in the feed:** likes and replies are disabled on them in phase 1, with "Reply on mastodon.social" or "Reply on Bluesky" links instead | 1–2 h |
| **Admin domain blocklist tab** | 1–2 h |
| **Total** | **about 7–11 h, about 2–3 sessions** |

## Total for phase 0 + phase 1
**About 77–101 h, roughly 19–25 sessions.** Suggested order: phase 0, then ActivityPub (tested against GoToSocial locally, then the real network through a tunnel), then AT Protocol. Ship ActivityPub on its own first.

## Can it be tested locally? Mostly yes.
- **ActivityPub: fully local.**
  - A compose profile `fedi-lab` adds **GoToSocial** (one Go binary with SQLite; far lighter than Mastodon). A Mastodon container is optional for checking compatibility.
  - Use a fresh mkcert CA and certs for `anahita.test` and `gts.test`. The existing `anahita.local` certs expired on 2026-03-31 and aren't wired in anywhere.
  - Both containers trust the CA; GoToSocial also has an insecure TLS option for dev. Docker network aliases handle container-to-container traffic and `/etc/hosts` handles the browser.
  - Everything (follow, accept, posts, deletes) can be exercised offline and in CI.
- **Real network before any cloud deploy:**
  - Run a **Cloudflare named tunnel** (or an ngrok reserved domain) on a real throwaway subdomain, for example `fedi-dev.yourdomain`, straight from the laptop.
  - The hostname **must be stable**: actor ids are URLs, and remote servers cache actors and keys indefinitely. Never test on a hostname you'll later use for production.
- **AT Protocol:**
  - **Inbound works locally.** The Jetstream consumer is an outbound websocket, so following real Bluesky accounts and seeing their posts needs no public exposure.
  - **Outbound needs a public PDS.** The Bluesky relay crawls your PDS over public HTTPS, and your DID document points at its URL. The same tunnel works, including the wildcard handle subdomains through Cloudflare DNS.
  - For a fully offline lab, Bluesky's `atproto` dev-env runs PLC, PDS, relay and AppView locally. It's heavy (Node plus Postgres), so treat it as optional.
  - Test DIDs created on the real PLC directory are effectively permanent, so use throwaway accounts.
- **Conclusion:** no cloud deploy is needed for phase 1. It all runs locally, plus a tunnel for real-network tests. A cloud deploy is only needed for production.

## Later phases (what more we can build)
- **Phase 2, interaction:**
  - replies in and out (after item 7: `inReplyTo`, AT `reply {root, parent}`);
  - likes (`Like`, `app.bsky.feed.like`);
  - reposts (`Announce`, `app.bsky.feed.repost`);
  - notifications for remote follows, likes, replies and mentions;
  - mentions of remote people (`@user@host`) and Create delivery to the people mentioned.
- **Phase 3, richer content:**
  - photos and attachments both ways (AT blobs, AP `Document` / `Image`);
  - articles as AP `Article` (and long-form on AT);
  - hashtags federated as `Hashtag` tags and AT facets;
  - quote posts;
  - custom emojis.
- **Phase 4, groups and audiences:**
  - groups as ActivityPub `Group` actors (FEP-1b12, Lemmy-compatible);
  - followers-only posts on ActivityPub (AT can't do this);
  - "authorized fetch" (servers must sign every fetch);
  - reply controls (AT threadgates and the AP equivalents).
- **Phase 5, moderation and trust:**
  - sending reports as `Flag` activities and to Bluesky moderation (ties into item 4b's `atproto_reason`);
  - consuming Bluesky labelers;
  - user-level domain mutes;
  - **block lists, for blocking many accounts at once** (asked for by the user on 2026-10-04, after Bluesky's moderation lists): a person makes or subscribes to a list of accounts and blocks or mutes everybody on it in one step, and the block follows the list as it changes. On Bluesky these are `app.bsky.graph.list` records with `listblock`, so subscribing to an existing Bluesky list can work directly. ActivityPub has no shared format; Mastodon imports and exports blocks as CSV, which is the least to support there. Locally a list is a node and a subscription an edge, checked by the same block rule the access gates already apply;
  - relays.
- **Phase 6, portability:**
  - account `Move` in and out;
  - keeping an Anahita-held did:plc rotation key so people can take their Bluesky identity elsewhere;
  - eventually a Go PDS (indigo) instead of the TypeScript container;
  - exposing feed generators to Bluesky.

## Open: reading the fediverse from a private site (noted 2026-10-04)

A members-only or preview installation does not federate (see
`instance-privacy.md`): nothing on it is public, so there is nothing to send.
What that rules out along the way is *reading*: following accounts elsewhere
from inside a private site. It need not.

- **Bluesky:** possible with nothing exposed. Followed accounts are read from
  the public stream, and the installation never has to be reachable.
- **ActivityPub:** only with a minimal public identity for each member who
  follows somebody. The other server has to look up the follower's handle and
  signing key before it accepts the follow, though the member's posts can all
  stay private.

Not decided. It is a choice for when phase 1 is built: either leave private
sites cut off, or add a "read only" setting with those two behaviours.

## Open: how many photos a post can hold (noted 2026-10-05)

A photo post holds 4 images today (`constants.PhotoMaxFiles`, item 17). To be
looked at again once federation is built, when what other servers do with a
longer post can be seen and not guessed. The user would like 10.

**What is known.**
- **Bluesky allows more than 4.** `app.bsky.embed.images` stops at 4, but
  `app.bsky.embed.gallery`, added to their repository on 2026-06-03, takes up
  to 20 items, with a note that clients should currently stop at 10. Each is
  an image of at most 2 MB and **must carry its aspect ratio**. We record
  width and height for photos uploaded since item 17; older photos have none.
  Read on 2026-10-05 from
  `https://raw.githubusercontent.com/bluesky-social/atproto/main/lexicons/app/bsky/embed/gallery.json`.
  The earlier "4 on Bluesky, fixed for everyone" in this plan and in the
  photo plan came from reading the images schema alone, and was wrong.
- **ActivityPub has no limit of its own.** Each piece of software sets one,
  for what its users post and for what it keeps from a post that arrives.
  Stock Mastodon lets its users attach 4.

**What is not known, and should be tested in the lab before deciding.**
- What stock Mastodon does with a post that arrives with more than 4
  attachments. Expected: it keeps the first 4.
- How many Threads keeps. Its own posts hold more than 10; what it does with
  one from another server has not been checked.
- Pixelfed, Misskey and its forks, Pleroma and Akkoma: believed to allow more
  than 4, set per instance. Not checked.

**The approach, if the limit is raised.**
1. The limit becomes a setting, `PHOTO_MAX_FILES`, with 10 as the most it can
   be. Whether 10 or 4 is the default is the decision to take then.
2. **To Bluesky:** a post of up to 4 photos as `app.bsky.embed.images`, which
   every client understands; one of 5 or more as `app.bsky.embed.gallery`.
   The gallery type is recent, so it is used only when it is needed.
3. **To ActivityPub:** every photo as an attachment, in order. When there are
   more than 4, one line is added to the end of the text sent, with **one
   link to the post**: "6 more photos: https://…". A server that keeps 4 then
   gives its readers one tap to the rest; one that keeps them all shows a
   harmless link to the original.

**Considered and not chosen: a link to each image beyond the fourth.** The
text of a post is the same for every server, since each fetches it from the
one address, so the links would also sit under a full carousel wherever all
the photos are shown. A bare image file has no description, where the post's
page shows each photo with its own. Mastodon would make its preview card from
the first of them. And links to files go on working wherever they were copied
after the post is deleted.

## Decisions (item 8)
- **Native on both protocols:** ActivityPub in Go; AT Protocol through the official PDS plus Jetstream.
- **Per-person opt-in,** a toggle per protocol. Only public profiles and posts federate, and Bluesky requires a public profile.

## Dependencies
- **Hard:** item 0 (feed gate) and phase 0 (UUIDs, media URLs, delivery queue, routing).
- **Soft:**
  - Item 7 (notes as replies) is needed before phase 2 replies.
  - Item 4a (admin area) hosts the domain blocklist.
  - Item 4b's reasons already carry `atproto_reason` for phase 5.
  - Item 5's purge should also send `Delete(Person)` and delete the PDS account for opted-in people. Add that here, or to item 5 if it lands after.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

**Phase 0**
- **services `configuration.md`**: `PUBLIC_DOMAIN`, `PUBLIC_API_URL`, and the media proxy.
- **services `architecture.md`**: UUIDs, remote actor types, `anahita-libs/federation`, the durable delivery queue.

**Phase 1, ActivityPub**
- **New services page `federation.md`** (operator view): turning a protocol on, handles, the domain blocklist, what federates and what doesn't, and the local `fedi-lab` setup with GoToSocial and tunnels.
- **services `architecture.md`**: activitypub-service and its routes.
- **services `registration.md`**, *NodeInfo*: `protocols` now lists what is deployed.
- **services `permissions.md`**: only public posts of people who opted in federate.
- **services `getting-started.md`**: the `fedi-lab` compose profile.
- **web `architecture.md`**: the Fediverse settings section, remote lookup and remote badges.

**Phase 1, AT Protocol**
- **services `federation.md`**: the PDS, wildcard DNS for handles, handle rules, Jetstream.
- **services `configuration.md`** and **`deploying.md`**: PDS settings and DNS.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| 0.1 Hosting layout | settled by item 10, 2026-10-11: see below |
| 0.2 UUIDs on nodes | done 2026-10-11 |
| 0.3 Public addresses | done 2026-10-11 |
| 0.4 Stable media addresses | done 2026-10-12, for avatars and covers |
| 0.5 Remote identity | the graph's part done 2026-10-11; the protocols' own tables come with each service |
| 0.6 Content conversion | done 2026-10-11 |
| 0.7 Durable delivery queue | done 2026-10-12 |
| 0.8 Routing and the TLS lab | done 2026-10-12; the gateway's routes to `activitypub-service` come with that service |
| Phase 1, ActivityPub: the service, opting in, keys, discovery, a person's description | done 2026-10-12 |
| Phase 1, ActivityPub: signing and checking requests, fetching, looking somebody up | done 2026-10-12 |
| Phase 1, ActivityPub: the inbox, being followed from elsewhere, the delivery worker | done 2026-10-13 |
| Phase 1, ActivityPub: following from here, and the other server's yes or no | done 2026-10-13 |
| Phase 1, ActivityPub: posts out and in, profile changes, backfill, the blocklist and limits, the web app | not started |
| Phase 1, AT Protocol | not started |

## What was built

### Phase 1, ActivityPub, begun 2026-10-12

**The service, and being found.** `activitypub-service` (port 8105), with
the gateway's routes to it; tables `activitypub_accounts` and `actor_keys`
(migration `20261027000001_activitypub.sql`); the page `docs/federation.md`.

- A person turns it on for themselves (`/activitypub/settings`), and only
  when their profile is open to everybody on a site that is open to read and
  can be reached. A key is made for them then.
- WebFinger, host-meta, a person's description, and the lengths of their
  followers and following.
- **Tried in the lab against GoToSocial:** asked to find
  `rastin@anahita.test`, it asked the web app's host, was sent on to the
  API, fetched the description, and answered with the account, its name, its
  page, when it joined, what it says of itself with the hashtags as links,
  and the avatar and cover, which it fetched through the addresses that
  stay.

**Following from here.** `events.go` listens for `edge.follow.*` on the bus
and sends `Follow` and `Undo`; the inbox takes `Accept` and `Reject`.

- **Tried in the lab:** somebody here followed alice on GoToSocial, whose
  account approves its followers. The request appeared for her there and
  was pending here; she approved it and it was accepted here; unfollowed
  from here, GoToSocial no longer listed the follower.
- **The follow is made at once and settled afterwards,** where the plan had
  a request to follow that became a follow on `Accept`. Following had to use
  the existing button and the existing service unchanged, as the plan also
  says, and those make a follow of a profile that is open to everybody, not
  a request. So the follow is there from the start, recorded as pending, and
  a `Reject` takes it away. The web app can say "requested" from the record.
- **Nobody on the fediverse approves their followers,** where the plan held
  a follow from elsewhere for anybody with `allow_follow_request`. That
  setting is for a profile that is *not* open to everybody, which does not
  federate; a public profile is followed by any stranger here, and a
  stranger elsewhere is held to the same and no more. It also removes what
  was listed below as still to do: answering a request that was held.
- **Still to do:** the sweep for follows the bus lost.

**The inbox, and being followed.** `POST /inbox` and each person's own;
`Follow` and `Undo(Follow)`; the queue's table `activitypub_deliveries` and
its worker; `activitypub_follows` (migration `20261029000001`).

- **Tried in the lab:** alice on GoToSocial followed somebody here. It
  showed "requested", this installation checked her signed `Follow`, made
  the follow and sent a signed `Accept`, and within seconds GoToSocial
  showed "following". She unfollowed, and the follow was gone here.
- **The worker runs inside the service,** not as a process of its own with
  a lock, as the plan had it. The queue hands a row to one worker at a
  time, so any number of copies may run.
- **A key may have an address of its own.** Mastodon names a key as its
  owner's address with `#main-key`; GoToSocial gives it a path
  (`…/users/alice/main-key`). Who signed is found from the key's own
  address either way, and must be on the key's server.
- **Still to do, found here:**
  - **A follower from elsewhere is subscribed to the person they follow,**
    as every follower is (`edge.base.subscription.v1`), so
    notification-service will make notifications for somebody who can never
    read them. Harmless, and wasteful: to be stopped there.
  - **The web app cannot show somebody from another server,** so the
    notification that they followed has no profile behind it.
  - **No limit yet on how much one server may send,** and no list of servers
    that are refused. Both before any live site federates.

**Signed requests, and looking somebody up.** `httpsig/` (signing and
checking), `fetch/` (reading from other servers, guarded and signed),
`remote/` (who somebody elsewhere is and what is believed of it), the
installation's own actor at `/ap/actor`, `GET /activitypub/lookup`, and the
table `activitypub_actors` (migration `20261028000001`).

- **Tried in the lab against GoToSocial,** which answers nobody without a
  signature it can check: `app-binary lookup @alice@gts.test` found her and
  kept her. On the way GoToSocial fetched this installation's key and asked
  WebFinger who it belongs to.
- **Signatures are written here and not taken from the library the plan
  named.** The scheme is a hundred lines and fixed; every line of it is
  something an attacker will try, and each check has its own test.
- **Found by trying, not by reading:**
  - A server that is shown a key asks WebFinger for `name@<the host in the
    key's address>`. Here that host is the API's and not the handle's, so
    WebFinger answers for both and names the real one. Without this,
    GoToSocial refused every signed request.
  - The installation's own actor has to be found by WebFinger too, as
    `example.org@example.org`.
  - A proxy that does not pass on the `Host` it was asked for breaks every
    signature. The lab's own front door did, for GoToSocial.
- **The guarded HTTP client moved** from link-preview-service to
  `anahita-libs/safehttp`, for both services.
- **Avatars of people elsewhere are not shown yet.** Their address is kept
  (`activitypub_actors.icon_url`); copying the picture into this
  installation's storage, so that no reader's browser calls another server,
  is still to do.
- **A handle that somebody else kept here already has** falls back to
  `name@<the server they are on>`; when that is taken too the lookup fails.
  Rare, and to be looked at again with profile updates.

**Different from the plan, or decided on the way:**
- **Somebody who is not on the fediverse answers `404`, always,** and never
  `410`. The plan had a purged person answer `410 Gone`. That would tell
  anybody who asks that an address was once somebody's, and of every other
  reason the plan itself says "404 or 410". Telling the servers that knew
  somebody to forget them is done by sending them a `Delete`, which is for
  the part that sends.
- **`setViewer` is on every route other servers use,** though a server has
  no token: it is what answers for a site that is for members only, and the
  test that every route names its viewer holds this service to it like the
  rest.
- **A person's description is not served at their page's address** by
  asking for it as JSON. Their page is on the web app's host, which the
  shell serves; WebFinger gives the address directly, which is what servers
  use. Left for when something needs it.
- **Opening one's account to the fediverse again after turning it off** will
  need care once turning it off sends a `Delete`: Mastodon treats a deleted
  person as gone for good. To be decided with the part that sends.

### Phase 0, begun 2026-10-11

**0.1, the hosting layout.** Settled when the web shell was built (item 10),
and not as this plan expected. The web app is on its own host, sent by
`web-shell`, and the API on another (`api.DOMAIN_NAME`): page addresses and
API addresses collide (`/people/ana` is both), so one front could not serve
both. What follows for federation:

- **A person's handle is on the web app's host,** `ana@example.org`, since
  that is the name people know the site by.
- **Everything another server fetches is on the API's host:** a person at
  `https://api.example.org/ap/users/<uuid>`, their inbox, their posts.
- **So the web app's host must answer `/.well-known/webfinger`** by sending
  the asker on to the API. That is for 0.8, in `web-shell` and for whoever
  hosts the web app some other way.

**0.2, UUIDs.** `nodes.uuid`, unique, version 4 (migration
`20261025000001_node_uuid.sql`, and the same in `schema/init.sql` and
`legacy/upgrade.sql`). Read with every node, and a node is found by it
(`ReadByUUID` in the graph and in `anahita-libs/data/services`).

- **The database gives it, as the column's default,** and not the code that
  adds a node. Nodes are added in more than one place, and by tests and the
  upgrade from the PHP application; a default cannot be forgotten by any of
  them. Version 4 and not MySQL's own `UUID()`, which is made of the time and
  of the machine's network address.
- **Added in three steps.** A default that differs each time cannot be given
  to the rows already there by adding the column with it (MySQL error 1674),
  so the column is added empty, filled, and then made required.
- **Not sent to browsers.** The web app has no use for it.

**0.3, public addresses.** `anahita-libs/site`: the domain of a handle, the
web app's address and the API's, from `PUBLIC_DOMAIN`, `PUBLIC_WEB_URL` and
`PUBLIC_API_URL`, each worked out from `DOMAIN_NAME` and `ENV` when it is not
set, the way auth-service works out its own. `CanFederate` refuses addresses
no other server could reach: this machine, a private network, anything not
https.

- **Different from the plan:** `PUBLIC_WEB_URL` existed already (the web
  shell and the export worker go by it), so it is the third setting and not a
  new one; and nothing has to be set on a site whose hosts are the usual ones.
- **The services that have their own idea of these addresses were left as
  they are** (auth-service's `FRONTEND_URL`, actor-service's `WEB_ORIGIN`).
  They agree with this package wherever the usual hosts are used, and it
  reads `FRONTEND_URL` too.

**0.4, an address for a picture that does not change.**
`GET /avatars/<uuid>/<size>` and `GET /covers/<uuid>/<size>`, in
`identity-image-service`, answer with a redirect to a freshly signed
address, kept for five minutes.

- **Only for what anybody may see.** The node is asked about as somebody not
  signed in and must say of itself that it is public; a members-only site
  answers for nothing. Everything else is not found, with nothing said.
- **By UUID and by the node, not by the file's key** as the plan had it
  (`/media/<key>`). An address made of a key would have sent anybody on to
  any file whose name they could work out, whoever it was meant for.
- **Avatars and covers only.** They are what a person's description to
  another server carries. A photo post's images get theirs when photos
  federate (phase 3).
- **Not built: letting another server's address for a picture pass through
  unsigned.** A picture shown straight from another server tells that server
  who looked. A remote person's avatar is to be copied into this
  installation's storage, as a link preview's picture already is; that is
  for phase 1, with the service that fetches them.

**0.7, the queue of what is to be sent.** `anahita-libs/federation`
(`queue.go`): `Enqueue`, `Claim`, `Done`, `Fail` and `Run`, over a table
whose shape is `TableSQL`. Each protocol's service has a table of its own.

- **A row is taken by putting off when it is next due,** and not by a state
  of its own. A worker that stops leaves nothing to put right: its row is
  simply due again ten minutes on. The first version marked a row as being
  sent and reset the ones held too long, and eight workers on one table
  deadlocked on it.
- **Tried again after 1, 4, 16, 64 and 256 minutes, then every six hours,
  for two days;** given up at once when the other server says the address is
  gone (`Permanent`). What is given up is kept, with why.
- **Added once** when given a key, so that whatever sweeps for what the bus
  lost can add again what is already waiting.
- Tested against MySQL with `FEDERATION_TEST_DSN`: eight workers, two
  hundred rows, each sent once.

**0.8, routing and the lab.**

- **The web app's host sends on what other servers ask it.** `web-shell`
  answers `/.well-known/webfinger`, `/.well-known/host-meta` and
  `/.well-known/nodeinfo` with a redirect to the same path on the API
  (`PUBLIC_API_URL`). Whoever hosts the web app some other way has to do the
  same; that is in `docs/web-shell.md`.
- **The lab** (`make fedi_lab`, `docs/federation-lab.md`): https at
  `anahita.test`, `api.anahita.test` and `gts.test` on one machine, with a
  GoToSocial to talk to. Tried: from inside the network, GoToSocial and this
  installation's API each answer by name over https to a caller that trusts
  only the lab's authority, and an account made on GoToSocial is found by
  WebFinger.
- **Not done here: the gateway's routes to `activitypub-service`** (`/ap/`,
  `/inbox`, `/.well-known/webfinger` on the API). nginx will not start with
  a route to a service that does not exist, so they are added with the
  service, first thing in row 25.
- **Nothing to fix in `k8s/99-ingress.yml`:** item 9 replaced it with the
  overlays' own ingress, which has TLS from cert-manager.

**0.5, what comes from another server.** The graph's part.

- **`nodes.uri` and `nodes.origin_domain`** (migration
  `20261026000001_node_remote.sql`, and the same in the other two places).
  A node with a `uri` came from elsewhere; one without was made here.
- **Two types of person,** `node.actor.activitypub-service.person.v1` and
  `node.actor.atproto-service.person.v1`. A note from elsewhere is an
  ordinary note.
- **`UpsertRemote` and `ReadByURI`** in the graph and in
  `anahita-libs/data/services`. One call for a person or a note, not the two
  the plan named: what differs between them is what is refused, and that is
  decided in one place.
- **What the graph refuses, whatever the caller sends** (`nodes_remote.go`,
  each with a test): anything but a person of another server or a note; a
  note on the profile of somebody here, or by them; an address that is one
  of this installation's own, or not https or `at://`; a name that could be
  a username here; and, for something already known, any change to what kind
  of thing it is or whose it is. The services that will call this read the
  internet, so none of it is left to them.
- **Who is counted.** Everything that asks for people by their type leaves
  people elsewhere out without being told. Two places asked for "an actor
  that is not a person" or "any post", and were told: the site's count of
  posts in NodeInfo, and the count of groups before making everything
  private. Search finds people elsewhere who are kept here, which is meant.
- **Not built here: `federation_actors`, `federation_edges`, `actor_keys`.**
  The plan's own table of what each service owns gives them to
  `activitypub-service` and `atproto-service`, so they are made with those.
- **A name already held.** If somebody elsewhere takes a handle that another
  kept person still has here, the second is refused
  (`ErrRemoteAlias`). The protocol service has to bring the first up to date
  and try again; noted for phase 1.

**0.6, the text of a post.** `anahita-libs/federation`:

- **`ToHTML`**: a post's Markdown as the HTML other servers are sent
  (goldmark, with the same GitHub flavour the web app reads), the people and
  hashtags it names made into links marked up as Mastodon marks them, and
  listed beside the text for the protocol to carry. Who a name is, is asked
  of the caller; somebody who cannot be found is left as written.
- **`ToMarkdown`**: the HTML of a post that arrived, as the Markdown a post
  here is kept as (html-to-markdown).
- **Both pass through one short list of what a post may be made of**
  (bluemonday): paragraphs, emphasis, quotations, lists, code, headings and
  links to the web. No pictures, which travel beside the text; nothing that
  runs or styles. HTML typed into a post is not passed on as HTML.
- **`entities.ParseMentions` understands `@user@host`.** This was a fault
  already, with or without federation: `@alice@mastodon.social` was read as
  `@alice`, so whoever is called alice here was told they had been
  mentioned. Somebody named on another server is now nobody here, unless the
  host is this installation's own. The web app's filter was made to agree:
  it also made a link of the end of an email address.
- **The libraries were added at versions that leave the rest of the module
  alone** (html-to-markdown 2.3.3, goldmark 1.7.13, bluemonday 1.0.27). The
  newest of each asked for a newer Go and newer `golang.org/x` modules than
  the services are built with. `go mod tidy` is not run on `anahita-libs`:
  it upgrades what it has no reason to.
