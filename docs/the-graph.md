# The graph

Everything in Anahita is a node, and every relationship between two things
is an edge. A person, a group, a post, a hashtag, a location and a feed are
nodes; following, liking, tagging, citing and saving are edges. This page is
about what the two are, how their types are named, and every type there is.

![A person follows a group, likes a post on its profile, and keeps a custom
feed on their home page; a hashtag and a location are tagged on the post.
Each arrow is an edge; the dashed one is the post's owner_id.](images/graph.svg)

## Contents

- [Why one graph](#why-one-graph)
- [Nodes](#nodes)
- [How a type is named](#how-a-type-is-named)
- [Every kind of node](#every-kind-of-node)
- [Edges](#edges)
- [Every kind of edge](#every-kind-of-edge)
- [Who may see a node](#who-may-see-a-node)
- [Nodes from other servers](#nodes-from-other-servers)
- [Events are not types](#events-are-not-types)
- [Where the code is](#where-the-code-is)

## Why one graph

A school's community holds people, groups, events, notes, articles, photos
and videos, and they are all connected: who follows what, who is in which
photo, which article cites which note, which posts are about `#seeds`. Kept
as one graph, each of these is read, permitted, searched and suggested in
the same way:

- **One rule for who may see what,** applied in one place, to every kind of
  node ([Who may see a node](#who-may-see-a-node)).
- **One shape for search and suggestions** to walk: "posts liked by people
  you follow" and "people cited by people you follow" are the same walk
  over different edges ([How things are found](how-things-are-found.md)).
- **Something new needs no new tables.** A custom feed is a node, and
  keeping it on the home page is an edge
  ([A feed is a node](feeds.md#a-feed-is-a-node)). A post from Mastodon is
  a node too, so following it, liking it and blocking its author need
  nothing new.

The graph is owned by one service, `graph-grpc-service`. Every other service
reads and writes it through that one, over gRPC
([Architecture](architecture.md#components)).

## Nodes

A node is a row of the `nodes` table. Every kind shares it; what a column
means can depend on the kind, and a column a kind has no use for is left
empty.

| Column | What |
| --- | --- |
| `id` | Its number on this installation |
| `uuid` | What another server is told it is called: random, and never changed |
| `type` | What kind of node it is ([How a type is named](#how-a-type-is-named)) |
| `name`, `alias`, `body` | What it is called, its handle or address, and what it says |
| `owner_id` | The profile it is on: the person or group a post was posted to. On a profile, empty |
| `created_by` | Who made it, who need not be its owner: a person's post in a group is owned by the group |
| `parent_id`, `root_id` | On a reply, what it answers and the post at the top of its thread. On a repost, the original |
| `quote_id` | On a note that quotes a post, that post |
| `access` | Who may see it ([Who may see a node](#who-may-see-a-node)) |
| `meta` | What one kind keeps that the others do not, as JSON: a feed's definition, for one |
| `uri`, `origin_domain` | What it is called on the server it came from, when it came from another one |
| `follower_count`, `vote_up_count`, `comment_count`, `citation_count`, … | Counts the graph keeps in step with the edges, so that a list does not count each time it is read |

## How a type is named

A type is a name from a fixed grammar, read left to right from the most
general to the most particular:

```
node.<family>.<service>.<entity>.v1      node.actor.person-service.person.v1
edge.<family>.<name>.v1                  edge.actor.follow.v1
```

- **The family** is what everything of that kind has in common. Every
  `node.actor.` can be followed, blocked and posted to; every `node.medium.`
  is a post, with an owner, an author and an audience. Code that asks
  "is this a profile?" asks whether the type begins with `node.actor.`.
- **The service** is the one that answers for that kind of node. A group's
  is still `group-service`, an older name, because a type is never renamed.
- **The version** is there because a type is stored for good: in every row,
  in every edge's `node_a_type` and `node_b_type`, and in what other
  services keep. A change to what a type means would be a new version, read
  beside the old, never an edit of the old.

## Every kind of node

| Family | Type | What |
| --- | --- | --- |
| Actors | `node.actor.person-service.person.v1` | A person with an account here |
| | `node.actor.group-service.group.v1` | A group, which people follow and post in |
| | `node.actor.actor-service.event.v1` | An event, which people say they are going to; often hosted by a group |
| | `node.actor.actor-service.website.v1` | A website's feed, verified, as a profile to follow ([Websites](websites.md)) |
| | `node.actor.activitypub-service.person.v1` | Somebody on a server of the fediverse |
| | `node.actor.atproto-service.person.v1` | Somebody on Bluesky |
| Media | `node.medium.text-service.note.v1` | A note: a short post, and the kind that arrives from another server |
| | `node.medium.text-service.article.v1` | An article: long, with a title; what a website posts for each item of its feed |
| | `node.medium.text-service.topic.v1` | A topic: a conversation with a title, started on a profile, with replies under it |
| | `node.medium.photo-service.photo.v1` | A photo post, of one or more photos ([Photo posts](photos.md)) |
| | `node.medium.video-service.video.v1` | A video post ([Videos](videos.md)) |
| Tags | `node.tag.hashtag-service.hashtag.v1` | A hashtag ([Hashtags and locations](hashtags.md)) |
| | `node.tag.location-service.location.v1` | A location posts are tagged with |
| Feeds | `node.feed.feed-service.generator.v1` | A custom feed: what it shows, kept in its `meta` ([Custom feeds](feeds.md)) |
| Reposts | `node.base.repost-service.repost.v1` | A repost: its `parent_id` is the original, and its owner the profile that reposted it |

A reply is a node of the same type as a note, with a `parent_id`. People
elsewhere have types of their own, not the person type with a flag:
whatever lists the people of this installation by type then leaves them out
without having to be told.

## Edges

An edge is a row of the `edges` table: a type, and two nodes, `node_a` and
`node_b`, with each one's type beside its id. It points from `a` to `b`, and
reads as a sentence: *Ana* **follows** *the library*.

| Column | What |
| --- | --- |
| `type` | What the relationship is |
| `node_a_id`, `node_a_type` | Where it is from |
| `node_b_id`, `node_b_type` | Where it is to |
| `meta` | What it says beyond that, as JSON: whether a tag of a person has been approved, for one |
| `ordering` | Its place among its siblings, for the ones kept in an order somebody chose: feeds on the home page |
| `created_by`, `created_at` | Who made it, and when; what fades with time is counted from this |

There is one edge of a type between the same two nodes, never two
(`uc_edge`): liking a post twice is liking it.

## Every kind of edge

| Type | From | To | Means |
| --- | --- | --- | --- |
| `edge.actor.follow.v1` | A profile | A profile | Follows it |
| `edge.actor.follow-request.v1` | A profile | A profile | Has asked to follow it, and is waiting for an answer |
| `edge.actor.block.v1` | A profile | A profile | Blocks it: neither sees the other |
| `edge.actor.admin.v1` | A person | A group or an event | Is one of its administrators |
| `edge.actor.host.v1` | A group | An event | Hosts it; what puts it on the group's list of events |
| `edge.actor.invite.v1` | A person | A profile | Was invited to follow it, by `created_by` |
| `edge.base.voteup.v1` | A person | A post | Likes it |
| `edge.base.bookmark.v1` | A person | A post | Saved it. Private: nobody else is told, the post's author included, and nothing is counted from it |
| `edge.base.subscription.v1` | A person | A post or a profile | Is notified of what happens to it ([Notifications](notifications.md)) |
| `edge.tag.hashtag.v1` | A hashtag | A post | The post carries the hashtag |
| `edge.tag.location.v1` | A location | A post | The post is tagged with the location |
| `edge.tag.mention.v1` | A profile | A post | The post names the profile: "this is for you" |
| `edge.tag.person.v1` | A person | A photo or video post | The person is in it, tagged by its author; shown once they approve |
| `edge.medium.citation.v1` | A post | A post | The first shows the second within its text, as an article or a topic does an address on a line of its own |
| `edge.medium.hidden-reply.v1` | A post | A reply | The reply is hidden from the post's thread; the reply stays its author's |
| `edge.feed.pin.v1` | A person | A custom feed | Keeps it on their home page, in the place its `ordering` says |

## Who may see a node

Every node has an audience, its `access`: `public`, `registered`,
`followers`, `leaders`, `mutuals`, `admins` or `myself`, each measured
against the profile the node is on, not whoever wrote it
([The audiences](privacy.md#the-audiences)). A block, either way, hides
everything.

The graph applies these to every read, whatever asked: a list, a search, a
suggestion, a feed. What a service is sent is already what the viewer may
see, so no service has a rule of its own to get wrong
([Permissions](permissions.md)).

## Nodes from other servers

A person on Mastodon or Bluesky, and what they post, are nodes here once
somebody here has met them: looked them up, followed them, or been replied
to. Each has a `uri`, what it is called where it lives, and an
`origin_domain`; a node without a `uri` was made here. See
[What a node is called elsewhere](architecture.md#what-a-node-is-called-elsewhere)
and [Federation](federation.md).

## Events are not types

What happens to the graph is told to the services that care over the bus,
RabbitMQ, by events with names in the same grammar and an action at the end:
`node.medium.text-service.note.added.v1`, `edge.follow.socialgraph.added.v1`.
These are the names of things that happened, not of nodes or edges, and are
never stored as a type. They are listed in `anahita-libs/event`
([Architecture](architecture.md#components)).

## Where the code is

| What | Where |
| --- | --- |
| The two tables | `migration-job/schema/init.sql` |
| The type names | `anahita-libs/constants/entity_types.go`, and beside the code of the kind (`anahita-libs/feeds`, `anahita-libs/videos`, `text-service/constants`) |
| The audiences | `anahita-libs/constants/access.go` |
| Every read and write | `graph-grpc-service/data/repositories` |
| How other services call it | `anahita-libs/proto/graphpb`, over gRPC ([Services over gRPC](grpc.md)) |
