# Feed generators

*Direction, not scheduled work. Belongs to feed-service. Nothing in the
person/user → graph refactor depends on it.*

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
