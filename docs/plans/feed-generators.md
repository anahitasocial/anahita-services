# Feed generators

*Belongs to feed-service. Scheduled on 2026-09-28 as roadmap item 14; the
design decisions, definition schema and estimates are in
[Decisions and schedule](#decisions-and-schedule-2026-09-28-roadmap-item-14) at the end.*

## Context

Anahita needs content discovery, and it will not have a large-scale ML
recommender. The question is what replaces one. Bluesky has the best available
answer, and it maps unusually well onto Anahita's node/edge architecture.

The immediate prompt was "should hashtags be followable?" — but following a
single tag turns out to be a special case of a more general mechanism, and
building the general one instead avoids a one-off feature.

## How Bluesky does discovery

**Both AI and plain algorithms — but the innovation is neither.** It is that
ranking is *unbundled* from the platform.

A **feed generator** is a separate service anyone can run:

1. it subscribes to the **firehose** — the Relay's aggregated stream of every
   post on the network;
2. it applies whatever logic it likes;
3. it exposes one endpoint, `getFeedSkeleton`, returning an **ordered list of
   post URIs — not post content**;
4. the AppView/client then *hydrates* those URIs into real posts.

The skeleton design is the clever part. The generator stays tiny, stores no
content, and never handles permissions — the AppView applies the viewer's blocks
and mutes during hydration. **Ranking is decoupled from both storage and access
control.**

**What logic do generators use?** The author's choice. Most popular feeds are
simple deterministic rules — keyword filters, curated account lists, engagement
counts, follows-of-follows. Some use ML classifiers for topic feeds. Bluesky's
own **"Discover" feed is a genuine ML recommender** trained on engagement — but
it is *one feed among many*, in a tab beside "Following" (pure
reverse-chronological) and the user's pinned custom feeds.

So: **algorithmic choice, not algorithmic absence** — a marketplace of rankers
rather than one opaque algorithm. Supporting pieces: **lists** (user-curated
account sets, usable as feeds or for moderation), **starter packs** (bundles of
accounts and feeds adopted at once), and **labelers** (third-party moderation,
equally composable).

## The design for Anahita: a feed generator is a node

This is the part that fits the nodes-and-graph architecture rather than fighting
it.

Introduce `node.feed.feed-service.generator.v1` — a node with an owner, a name,
an alias, and its query definition in `meta`. Then:

- **Following a feed is just a follow edge** (person → feed node). No new
  relationship concept, no new table.
- Feeds get profiles, avatars, follower counts, access control and discovery
  **for free**, because they are nodes like everything else.
- **"Follow a hashtag" stops being a special case** — it becomes a feed whose
  definition names that tag, which also yields multi-tag and compound filters at
  no extra cost.
- Feeds are shareable and browsable by popularity using graph primitives that
  already exist.

Anahita is a single instance over one database, so a generator is a **query
definition, not a network service** — no firehose, Relay or AppView split
needed. That is a large simplification over Bluesky's version.

## Ranking without ML

Graph-native ranking primitives already exist in `graph-grpc-service`:

- `BrowseOrderedByDegree` (`data/repositories/graph.go:209`) — orders by
  `COUNT(edge.id) DESC`, i.e. pure graph degree.
- `Trending` (`data/repositories/graph.go:291`) — takes a time window
  (`1h`/`6h`/`24h`/`DAILY`).
- `BrowseNeighbours`, `ReadNeighbourhood`, `Traverse`, `ReadShortestPath` —
  distance-from-viewer signals.

**A social graph is itself a ranking signal** — degree, recency, engagement,
distance from the viewer — with no model required. This is the same place
Bluesky's most-used custom feeds sit.

## Why not the simpler alternatives

Three designs were considered for "followable hashtags":

**(A) Local tag following, Mastodon's model.** The tag stays a tag; following is
a local edge plus a feed-query change. Simple, but single-tag only, and it adds
a second bespoke source to the feed query that nothing else reuses.

**(B) Hashtag as a `Group`/`Service` actor that `Announce`s matching content** —
the Lemmy-community / Guppe-Groups pattern. Genuinely federated, so a Mastodon
user could follow `@bread@anahita.io`. But it needs a handle in the actor
namespace, an inbox accepting deliveries from anywhere, and it rebroadcasts — a
real spam and moderation surface. It also changes what a hashtag *means*, from a
label on content to a place content is sent to.

**(C) Feed generators — this document.** Subsumes (A) entirely, avoids giving
tags actor semantics, and is the only option that handles multi-tag and compound
filters. More work up front; strictly more general. Recommended over (A) rather
than after it, since (A) is a special case of (C) and would be thrown away.

## Open design questions

1. **Definition language.** A constrained JSON filter (tags, authors, node
   types, time window, ordering) is safer than anything resembling arbitrary
   query text, which would be an injection and cost surface given generators are
   user-authored.
2. **Access control.** Follow Bluesky: the generator selects *candidates*, and
   the existing access layer (`SetAccessQuery` and friends) filters at
   hydration. A generator must never be able to widen visibility.
3. **Cost.** A user-defined feed is a user-defined query. Needs a complexity
   cap, result limits, and probably caching per feed rather than per viewer.
4. **Federation.** A feed node could later expose an AP `OrderedCollection`, so
   remote instances can consume an Anahita feed. Worth keeping the door open;
   not required initially.
5. **Seeding.** Ship a few first-party feeds (Following, Trending, a per-group
   feed) built on the same mechanism, so it is exercised from day one rather
   than being a power-user afterthought.

## Related

- Hashtags as labels vs entities, and why only actors get handles: see the
  person/user → graph plan.
- `edge.actor.follow.v1` drives feeds; `edge.base.subscription.v1` drives
  notifications. A followed feed uses the former.


---

## Decisions and schedule (2026-09-28, roadmap item 14)

### Context
- **What we want:** people build their own feeds from sources such as actor types (people, groups, events), hand-picked sets of actors of any type, and the people they follow. They filter by media type (notes, articles, topics, photos, or "with images"), hashtags, keywords and locations.
- **Rules:** everything is **chronological**. No "For You".
- **The existing plan already chose the right model:** a feed is a node (`node.feed.feed-service.generator.v1`) with its definition in `meta`, and following a feed is an ordinary follow edge. It also says: select candidates, then apply the existing access layer ("a generator must never widen visibility"), and use a constrained JSON definition, never query text.
- **"Follow a hashtag" becomes a one-tag feed,** so no separate feature is needed.

**What exists today:**
- feed-service has only `GET /feeds/leaders/` and `GET /feeds/actor/:actor_id/` (`feed-service/cmd/api/routes.go:31-32`). They're backed by graph-grpc `feed_leaders.go` / `feed_actor.go` with `access_feed.go`, which is fixed by item 0.
- Media types come from `feed-defaults/defaults.json`.
- Search uses `LIKE` (`graph-grpc-service/data/repositories/search.go:101,139`).
- The ranking primitives (`Trending`, `BrowseOrderedByDegree`) aren't needed, since everything is chronological.

### On "For You" and AI
- **A "For You" doesn't need AI.** Item 3 shows how: weighted signals from the social graph, deterministic and explainable. Bluesky's most popular custom feeds are plain rules like these; only its own "Discover" feed uses a model.
- **But choosing chronological is a sound product decision** (no hidden logic, no engagement bait), and it's what we plan.
- Every feed is newest first. A "Top" ordering option (likes plus replies in a time window) is recorded as a possible later addition, one enum value away, but not built.

### Decisions (item 14)
- Feeds are **nodes**, **private ("Only me") by default**. The owner can make a feed public or followers-only, and others can follow and pin it.
- **Chronological only** (newest first).

### Confirmed and extended by the user: 2026-10-11

- **The default feed is chronological:** posts from the actors the viewer follows, newest first. Many people are unhappy with algorithmic feeds, and the default is not one.
- **A ranked feed, when one is built, is never the site's.** It is a feed somebody makes: they choose the parameters and the weight of each, save them with that feed, and call it what they like. That is their "For You", and they can have several.
- **What this asks of the work now:** nothing ranked is built in item 14. The definition's reserved `order` key is where it goes later: beside `"newest"`, an order by weights, with the weights stored in the same definition, shown in the same plain-language summary and copied by "Copy this algorithm". The signals and weights of the recommendation service (item 3) are the natural ones to offer, so a ranked feed and suggested posts would be one mechanism and not two.

### The name: custom feeds (user, 2026-10-11)

The feature is called **custom feeds**, everywhere a person reads it: the menu, the editor, the buttons, the help. The user chose it over "your algorithm" as better for the people using it and for telling others about it, "algorithm" being a word many now dislike. So: "Create a custom feed", "Edit feed", "Copy this feed" where the sections below say "Your algorithm" and "Copy this algorithm". One ordered by weights, when there are such feeds, is a custom feed like any other, named by whoever made it: nobody is shown the words "ranked feed" or "algorithmic feed". The user asked for this in particular. "Algorithm" and "generator" stay as words for the code and for these plans.

### "Each feed is your own algorithm"
- The definition below **is** the feed's algorithm, and the editor is where people build it. The web app presents it that way: a "Your algorithm" editor, not a settings form.
- **Plain-language summary:** every feed shows a readable sentence generated from its definition, for example "Photos and notes from my groups tagged #garden or #seeds, without replies, newest first". Nothing about a feed is hidden, which is the point of choosing this over an opaque "For You".
- **Remix:** "Copy this algorithm" on any feed you can see duplicates its definition into a new private feed you can adjust. That's how good feeds spread without anyone editing someone else's feed.
- **Ordering stays chronological** (decided 2026-09-28). The definition keeps a reserved `"order": "newest"` key, so later options ("top in the last 7 days", or boosting posts from chosen actors) are one more field in the same editor, not a redesign.

### The definition (constrained JSON in the feed node's `meta`)
```json
{
  "sources": {
    "following": true,
    "actors": [123, 456],
    "actor_types": ["person", "group", "event"],
    "members_of": [789]
  },
  "filters": {
    "media_types": ["note", "article", "topic", "photo"],
    "with_images": false,
    "hashtags_any": ["permaculture", "seeds"],
    "keywords_any": ["compost"],
    "locations_any": [42],
    "language": "en"
  },
  "exclude": { "hashtags": [], "keywords": [], "actors": [] },
  "include_replies": false,
  "order": "newest"
}
```
- **Sources are combined with OR.**
  - `following` means the owner's leaders: people, groups and events followed.
  - `actors` is a hand-picked set of any actor type (a list).
  - `actor_types` means "any actor of these types". It can only be combined with at least one filter, so a feed can't be "everything on the site" (in line with item 2).
  - `members_of` means posts by members of these groups.
  - No sources at all means the site: allowed only when a hashtag, keyword or location filter is present. That's how a hashtag or keyword feed works.
- **Filters:** AND across kinds, OR within a list. `exclude` always wins.
- **Validation limits:** at most 50 actors, 10 hashtags, 10 keywords (each 2–50 characters, matched as whole words where possible), 5 locations and 5 groups. Unknown keys are rejected. Hashtags are normalised.
- **Keywords:** `LIKE` on name and body, with metacharacters escaped as search already does, is fine at this scale. Add a MySQL `FULLTEXT` index (ngram parser for multiple languages) only if it gets slow.
- **`language`** uses `nodes.language`, which nothing writes yet. It's optional, and ignored until posts carry a language.

### Query and access
- **A new graph-grpc RPC, `BrowseFeed(definition, viewer, cursor, limit)`,** builds the SQL from the definition only through a builder. There are no raw fragments, so injection is impossible.
  - Hashtags and locations join `edge.tag.*`, `following` joins follow edges from the owner, and keywords use `LIKE`.
- **Access:** every candidate goes through the same gate as the leaders feed (after item 0: `COALESCE(object.access, node.access)`, owner access, blocks in either direction, lifecycle, archive). It's evaluated **for the viewer**, not for the feed's owner, so a public feed never shows a follower more than they could see anyway.
- **Pagination:** a keyset cursor on `(created_at, id)` instead of `OFFSET`. It stays fast and stable as new posts arrive.
- **Cost limits:**
  - 40 items per page;
  - a query timeout (e.g. 2 s) that returns a friendly "this feed is too broad" message;
  - an in-memory cache of the first page per feed for 30 s for public feeds (hydration and access still per viewer, so only ids are cached);
  - at most 20 feeds owned per person.
- Replies (item 7) are excluded unless `include_replies` is set.

### Endpoints (feed-service)

| Endpoint | Purpose |
| --- | --- |
| `POST /feeds/` `{ name, alias?, description, access, definition }` | Create. Validates the definition. `access` defaults to `myself`. |
| `GET /feeds/:id`, `PATCH /feeds/:id`, `DELETE /feeds/:id` | Read, edit and delete. Owner or site admins. |
| `GET /feeds/:id/items?cursor=` | The feed itself: skeleton, then hydration, like the existing feeds |
| `POST /feeds/preview` `{ definition }` | The first page for an unsaved definition, so the editor can show results live |
| `GET /feeds/mine`, `GET /feeds/following` | My feeds, and feeds I follow |
| Pinning: `PUT /feeds/pins` `{ ids: [...] }` | The ordered list of feeds pinned as home tabs, stored as an edge `edge.feed.pin.v1` with order in `meta`, or in a small `feed_pins` table |
| Follow and unfollow | The existing socialgraph follow endpoints, with the feed node as the target |

- **Seed first-party feeds per person on first use,** as the existing plan's question 5 suggests: "Following" (the current leaders feed, kept as its fast path), and "My groups" (`sources.members_of` = the groups I follow).
- **Hashtag shortcut:** "Follow #tag" on a hashtag page creates, or reuses, a private one-tag feed and pins it.

### Web (anahita-web)
- **Home tabs:** Following, plus your pinned feeds. Drag to reorder, stored through `/feeds/pins`.
- **Feed editor** (`/feeds/new`, `/feeds/:id/edit`):
  - pick sources (chips for "People I follow", actor-type toggles, an actor picker for hand-picked sets, a group picker);
  - filters (media-type checkboxes, "with images", hashtag, keyword and location chips, exclude chips);
  - access (Only me, Followers, Registered, Public);
  - a **live preview** from `POST /feeds/preview`.
- **Feed page** (`/feeds/:alias`): a profile-like header (name, owner, description, follower count, Follow and Pin buttons), then the chronological list. It reuses the existing feed item components.
- **Hashtag page:** a "Follow #tag" button (the shortcut above).
- **My feeds page:** owned and followed feeds, with Edit, Delete, Pin and Share.

### Estimate

| Part | Time |
| --- | --- |
| Feed node type, definition schema and validator, CRUD endpoints, pins | 4–5 h |
| `BrowseFeed` builder (sources, filters, excludes), access for the viewer, keyset cursor, timeout, cache, tests (including "never widens visibility") | 7–9 h |
| Preview endpoint, seed feeds, hashtag shortcut, "Copy this algorithm", plain-language summary | 3 h |
| Web: home tabs + pinning, feed editor with live preview, feed page, hashtag follow, my feeds | 7–9 h |
| Update `feed-generators.md` and `docs/permissions.md` | 1 h |
| **Total** | **about 22–27 h, 5–7 sessions** |

### What was built: 2026-10-11

Row 19, on branch `m7-discovery` in both repos. What a feed is and how it is used is in `docs/feeds.md`; this is what was done and where it differs from the sections above.

**Services**

- **`anahita-libs/feeds`:** the definition (`Parse`, `Check`, the limits, the words for kinds of post and profile) and the cursor. Used by feed-service when a feed is saved and by graph-grpc-service before it builds a query.
- **graph-grpc-service:** `FeedCustomService.Browse` (`proto/graphpb/feed_custom.proto`, `handlers/feed_custom.go`, `repositories/feed_custom.go`). Gated with `SetFeedAccessQuery`, `SetLifecycleQuery` and `SetArchiveQuery` for the reader, and paged by a cursor on `(created_at, id)`. `FilterVisible` judges a feed node by its own access and its owner's: before this a feed would have fallen among hashtags and places, which anybody may see.
- **feed-service:** `handlers/custom_feed.go` and its routes: make, read, change, delete, copy, list mine, preview, items, and the feeds on the home page.
- **No migration.** The definition is in the node's `meta`, and a feed on the home page is an edge with its place in `ordering`.
- **Tests:** `anahita-libs/feeds` (what is tidied, every refusal, the cursor), and `feed_custom_integration_test.go` against a real database: 33 cases over who sees what, each source, filter and exclusion, paging while a post is written, two posts in one second, and a feed's own audience.

**Web**

- Home has tabs: Following, then the feeds kept there, with the open one in the address (`?feed=`).
- "Custom feeds" in the left menu (`/feeds`): the viewer's feeds and the ones of other people's they keep, with adding to and removing from the home page and moving among its tabs.
- The editor (`/feeds/new`, `/feeds/:id/edit`) with what the feed shows in a sentence and a live preview beside it.
- A feed's own page (`/feeds/:id`): add to home, copy, edit, delete.
- "Follow #tag" on a hashtag's page makes a feed of that hashtag and puts it on the home page.

**Different from the plan above**

- **`actor_types` is a filter, not a source.** As a source added to the others it could not say "the groups I follow", which is the feed most wanted. As a filter it does: `following` with `actor_types: ["group"]`.
- **`members_of` was left out,** for the same reason: that feed is the line above.
- **`with_images` was left out.** `media_types: ["photo"]` is the photo posts.
- **Following a feed is keeping it on the home page.** One edge, `edge.feed.pin.v1`, and no follow edge to a feed: the follow rules are about actors, and a second way of holding on to a feed added nothing a person would notice.
- **No feeds are made for a new account.** Following is the first tab already.
- **No cache of a first page,** and the time allowed is the three seconds every query has, not two.
- **Moving tabs is by arrows on the list page,** not by dragging.
- **A feed is addressed by its id,** `/feeds/:id`, not by an alias.
- **The hashtag shortcut is in the web app:** it makes the feed and pins it with the ordinary endpoints.
- **Wording:** "custom feed", "Copy this feed". Nothing a person reads says "algorithm".

**Added the same day, after the user tried it**

- A feed is put on its maker's home page when it is made, and the web app lists the feeds on the home page under Home in the left menu, with "Custom feed" under them to make another. The separate "Custom feeds" entry in the menu went; the list page is reached from the end of the tabs on Home.
- Tests for the endpoints (`feed-service/cmd/api/handlers/custom_feed_test.go`), which had none.

**Not tried**

Nothing that needs somebody signed in was tried by me: making, changing, copying and pinning a feed, and every screen. The routes answer as they should without a session (404 for a feed that is not there, 401 for the ones that need signing in), the query is tested against a real database, and the web app builds and its tests pass.

### Verification
- **Algorithm summary:** the summary sentence matches the definition for every combination in the validator's tests. "Copy this algorithm" creates a private feed with the same definition, owned by the copier.
- **Access (real-MySQL tests):** a public feed built on a private group's hashtag shows nothing from that group to a non-member viewer and shows it to a member. Blocked authors never appear.
- **Definitions:** a definition with only `actor_types` and no filter is rejected. Unknown keys are rejected, and anything over the limits gives 422.
- **Pagination:** posting while paging doesn't duplicate or skip items (keyset).
- **Manual:**
  - Build "Photos from my groups tagged #garden", preview it live, save it private, and pin it; it appears as a home tab.
  - Make it public; a second user follows and pins it and sees only what they're allowed to see.
  - "Follow #permaculture" from a hashtag page creates and pins a one-tag feed.
  - A keyword feed for "compost" shows new matching notes in order.

### Dependencies
- **Hard:** item 0 (feed access gate).
- **Soft:**
  - item 2 (no global browse; feeds are the discovery surface);
  - item 7 (the `include_replies` flag);
  - item 12 (the `event` actor type);
  - item 8 (remote posts appear automatically in feeds whose sources include followed remote actors; exposing feeds as ActivityPub collections is later);
  - item 3 (a non-AI "Suggested" feed could later be one more generator on the same mechanism).

### Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **New services page `feeds.md`**: the feed definition schema with examples, limits, and how access is applied for each viewer.
- **services `architecture.md`**: feed nodes, `BrowseFeed`, the feed endpoints.
- **services `permissions.md`**: feed visibility (Only me / Followers / Registered / Public) and following feeds.
- **web `architecture.md`**: home tabs and pinning, the algorithm editor, feed pages.
