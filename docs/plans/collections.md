# Collections: saved posts, grouped, each with its own audience

*Roadmap item 34, raised by the user on 2026-10-06. **Kept for later: nothing is to be built until the open decisions below are settled.** The design here is a proposal to start from, not something agreed.*

## The idea, in the user's words

Replace the Saved items with Collections, where people can make a collection of nodes. Each collection has privacy settings and is a node. It would make a nice additional service.

## What exists today

Saved posts (roadmap item 33, done 2026-10-06):

- A saved post is an edge from the person to the post, `edge.base.bookmark.v1` (`constants.EdgeBaseBookmarkEntityType`).
- `BookmarksRepo` in graph-grpc-service (`data/repositories/bookmarks.go`): `Set`, `IsSavedMany`, `List`, `Count`. Every method takes the person it is about and none answers about anybody else. `List` goes through `SetMediumAccessQuery`, so a post its saver can no longer see is left out.
- Graph RPCs `SetSaved` and `ListSaved`. Each post carries `is_saved_by_viewer`.
- text-service serves `GET /bookmarks/`, `PUT /bookmarks/:id` and `DELETE /bookmarks/:id`; nginx sends `/bookmarks/` there.
- Nothing is counted on the post, no event is published, and nobody is told.
- Web: "Save" in a post's menu, and a Saved page in the left menu at `/saved`, with one line saying only you can see it.

Item 18 of the first plan (2026-09-28) decided against albums and collections, for two reasons that still have to be answered: a collection of other people's posts needs their consent, and a collection others can see needs access filtered for each viewer.

## Proposed design

### A collection is a node

- Type `node.collection.collection-service.collection.v1`: a name, an optional description, an owner, and `access`.
- Owned by a person. Groups do not get collections in the first version.
- `access` uses the keywords posts already use: `myself` (the default), `followers`, `registered`, `public`. It is checked by the same gate as everything else.
- Being a node, it is deleted and purged with its owner, can be reported like any node, and could later be followed or liked without new machinery.

### What is in it is an edge

- `edge.collection.item.v1`, from the collection to the post. One post is in a collection once.
- Order: most recently added first. No arranging by hand in the first version.
- A post can be in several collections.
- Any post the person may see can be added: notes, articles, topics and photos. Replies too. Actors and other collections cannot.

### What becomes of Saved

Proposed: every person has one collection that is there without being made, called Saved.

- It is always `myself`. It cannot be renamed, shared or deleted.
- "Save" on a post puts it there in one press, as now. Choosing a collection is a second step, offered after.
- A migration moves each `edge.base.bookmark.v1` into its owner's Saved collection, creating it where needed. Nothing anybody saved is lost.
- `is_saved_by_viewer` comes to mean "in any of the viewer's collections".

The other way is to leave the bookmark edge as it is and add collections beside it. That needs no migration, and leaves two ways of holding on to a post.

### Who sees what

- **The collection itself** is seen by whoever its `access` admits.
- **Each post in it** is seen only by a viewer who could see that post anyway. The list is read for the viewer, through `SetMediumAccessQuery`, never for the collection's owner. A public collection holding a followers-only post shows that post to followers and to nobody else.
- **The number on a collection** is counted for the viewer, or not shown to anybody but the owner. A stored count would tell a stranger that hidden posts are there.
- **Blocks:** a post by somebody who has blocked the viewer, or whom the viewer blocked, is left out, as everywhere.

### The author's say

Proposed, by analogy with quote posts:

- A collection that only its owner sees can hold any post the owner may see. Nobody is told, as with Saved now.
- A collection others can see may hold a post only if its author allows it to be quoted (`permissions.CanQuote`, from the post's `quote_policy` or its author's default). Widening a collection's audience is refused while it holds posts that do not allow it, and the answer names them.
- An author can take their own post out of somebody's visible collection, as they can detach a quote.
- Whether the author is told that their post went into a visible collection is open. See below.

### The service

A new `collection-service`, laid out like hashtag-service: Echo, `an_mw.SetViewer`, gRPC clients from `anahita-libs/data/services`, its own dockerfile, compose entry, nginx location, Kubernetes manifest, Makefile targets and `go.work` entry. It keeps no tables: collections are nodes and their contents are edges, both through new graph-grpc RPCs.

An installation that does not want collections would not deploy it, but then nothing can be saved either. If that matters, saving stays in text-service and only collections move.

| Endpoint | Who | What |
| --- | --- | --- |
| `GET /collections/` | signed in | The viewer's own collections, Saved first |
| `GET /collections/person/:id/` | anybody | That person's collections the viewer may see |
| `POST /collections/` `{ name, description, access }` | signed in | Make one |
| `GET /collections/:id` | whoever its access admits | Read one |
| `PATCH /collections/:id` | owner | Rename, describe, change access |
| `DELETE /collections/:id` | owner, site administrators | Delete it. The posts are untouched |
| `GET /collections/:id/items` | whoever its access admits | The posts, as the viewer may see them |
| `PUT /collections/:id/items/:post_id` | owner | Add a post |
| `DELETE /collections/:id/items/:post_id` | owner, or the post's author | Take a post out |
| `GET /collections/for/:post_id` | signed in | Which of the viewer's collections hold this post, for the picker |

`/bookmarks/` would stay as a short way to the Saved collection, served by collection-service, so the web app's Save keeps working through the change.

Limits, to be set in constants: about 50 collections a person, about 1,000 posts in one, a name of 100 characters. Adding is rate-limited with the shared limiter.

### Web

- The **Saved** page in the left menu becomes **Collections**: Saved first, then the others, each a card with its name, its audience and a few thumbnails.
- A collection's page: its header with name, owner and audience, then the same masonry list as now. Edit and Delete for the owner.
- **Save** in a post's menu saves in one press. The confirmation offers "Add to a collection", which opens a picker with a tick beside each collection that holds the post and a field to make a new one.
- The audience picker is the one posts use.
- On a person's profile, a **Collections** tab listing the ones the viewer may see. Not shown when there are none.
- The line "Only you can see the saved items" stays on the Saved collection and is replaced by the audience on the others.

## To be decided before any design is fixed

1. **Saved as a collection, or beside collections.** Proposed: as a collection, with a migration.
2. **The author's say.** Proposed: the quote setting decides for visible collections. The alternatives are no rule at all, or a setting of its own.
3. **Whether an author is told** when their post goes into a collection others can see. Telling them is honest; it is also noise, and Saved was built to tell nobody.
4. **Whether collections can be followed,** and whether a collection that gains a post appears in its followers' feeds. That makes a collection a publication, which is a larger feature.
5. **Whether groups get collections,** looked after by their administrators.
6. **Where saving lives** if an installation does not run collection-service.
7. **Federation.** Saved posts are private on Mastodon and Bluesky and are not sent anywhere. A public collection has no settled shape on either network. Proposed: collections stay on this installation in the first version.

## Estimate

Not estimated until the decisions are made. As a rough size, with the proposals above taken as they stand: the service, graph RPCs, migration and tests about 3 sessions; the web pages and the picker about 2 to 3.

## How it would be checked

- A public collection holding a followers-only post shows that post to a follower and not to a stranger, and the stranger's count does not include it.
- A post whose author allows nobody to quote can be added to a private collection and is refused by a public one. Making that private collection public is refused, and the answer names the post.
- An author takes their post out of somebody's public collection; they cannot take it out of the owner's private one, and cannot learn that it is there.
- After the migration, every person's Saved collection holds what they had saved, in the same order, and a second run changes nothing.
- Deleting a collection leaves its posts alone. Purging a person removes their collections and the edges into them.

## Depends on

- Saved posts (item 33, done).
- Quote posts (item 25, done), for the author's say.
- The shared rate limiter (done with abuse reports).
- Download your data (item 21) would include a person's collections.

## Status

| Item | State |
| --- | --- |
| Idea recorded | 2026-10-06 |
| Open decisions | seven, above |
| Design and work | not started |
