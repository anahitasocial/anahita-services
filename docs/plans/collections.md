# Collections: an album of posts of any kind

*Roadmap item 34, raised by the user on 2026-10-06. **Kept for later: not scheduled, and not to be built until it is.** The user settled its shape the same day; what is still open is listed near the end.*

## What it is

A collection is a post that holds other posts: an album that takes notes, articles, topics and photos alike. Somebody gathers posts into it, puts them in order, gives it a title and a description, and it is then read, liked, replied to and reposted like any other post.

It is a feature of Anahita installations only. It is not sent to Mastodon or Bluesky.

## Decided by the user, 2026-10-06

1. **Saving stays as it is.** Saved posts are private and are not turned into a collection. Collections are a new feature beside them.
2. **A collection is a media node,** with a title and a description. It can be liked, replied to, reposted and everything else a media node can.
3. **It has the same levels of privacy** as other media nodes.
4. **Its author can change the order** of what is in it.
5. **It can be viewed as a slide show,** in the light box there already is.
6. **People, groups and events can have collections.**
7. **What goes into a collection comes from what you saved,** and goes into a collection that already exists.
8. **It does not federate.**

On 7, this page reads it as two things, to be confirmed when the work is scheduled: a post is added to a collection from the adder's saved posts, and the picker offers the collections there are, without making a new one on the way. A collection is made first, like any post.

## What exists today

Saved posts (roadmap item 33, done 2026-10-06), which this builds on and does not change:

- A saved post is an edge from the person to the post, `edge.base.bookmark.v1`. `BookmarksRepo` in graph-grpc-service (`data/repositories/bookmarks.go`) holds `Set`, `IsSavedMany`, `List` and `Count`. `List` goes through `SetMediumAccessQuery`.
- text-service serves `GET /bookmarks/`, `PUT /bookmarks/:id` and `DELETE /bookmarks/:id`.
- Nothing is counted on the post and nobody is told.
- Web: "Save" in a post's menu, and the Saved page in the left menu.

Also there to be used: the access gate for media, `GraphNodeGRPC.FilterVisible`, the handlers media share in `anahita-libs` (access, reply access, quote policy, pin, delete), the per-actor feature permissions in `graph-grpc-defaults/actor_features/`, and on the web the light box in `containers/media/Stepper`.

Item 18 of the first plan (2026-09-28) decided against albums and collections for two reasons: a collection of other people's posts needs their consent, and a collection others can see needs access filtered for each viewer. The second is answered below. The first is still open.

## Design

### The collection

- Type `node.medium.collection-service.collection.v1`. A media node: `name` is the title, `body` the description.
- `owner_id` is the profile it is on, a person, a group or an event. `created_by` is who made it.
- Everything media have comes with the type and needs no code of its own: `access`, likes, replies and who can reply, reposts, quotes and who can quote, pins, hashtags and mentions in the description, reports, Interactions, saving, language, delete and purge.
- It appears in feeds and on profiles as a post, shown by its title, description and the first few things in it.
- Who may make one on a group or an event is a feature permission like the ones for other media, in `actor_features`. Events are roadmap item 12 and are not built yet; collections on them come with them.

### What is in it

- An edge from the collection to the post, `edge.collection.item.v1`, with its place in the order kept on the edge. One post is in a collection once, and can be in several collections.
- Any media node can go in: a note, an article, a topic, a photo, and a video when there are videos.
- Putting something in, taking it out and moving it are done by whoever may edit the collection: its author, and on a group or event its administrators.
- A new item goes to the end. Moving sends the whole order in one request.
- Proposed limits, as constants: 100 items in a collection, and a rate limit on adding.

### Who sees what

- The collection is seen by whoever its `access` admits, through the ordinary gate.
- Each post in it is seen only by a viewer who could see that post anyway. The list is read for the viewer and never for the collection's author. A public collection that holds a followers-only post shows it to followers and to nobody else. Posts by somebody blocked, either way, are left out.
- The number of items shown is the number that viewer can see. No count is stored on the node, as it would tell a stranger that hidden posts are there.
- The preview in a feed is built from the first items the viewer can see.

### The service

A new `collection-service`, laid out like text-service and photo-service: Echo, the shared media handlers, its own dockerfile, compose entry, nginx location `/collections/`, Kubernetes manifest, Makefile targets and `go.work` entry. It keeps no tables: the collection is a node and its contents are edges, through new graph-grpc RPCs (add, remove, reorder, list for a viewer, which of these posts are in which of my collections).

An installation that does not want collections does not deploy it. Saving is in text-service and is not affected.

| Endpoint | Who | What |
| --- | --- | --- |
| `POST /collections/:owner_id/` `{ name, body, access }` | whoever may post there | Make one |
| `GET /collections/:owner_id/` | anybody | That profile's collections the viewer may see |
| `GET /collections/:id`, `PATCH`, `DELETE` | as for any medium | Read, edit, delete. Deleting leaves the posts alone |
| `GET /collections/:id/items` | whoever its access admits | The posts in order, as the viewer may see them |
| `PUT /collections/:id/items/:post_id` | editors | Add a post. It must be one the adder has saved |
| `DELETE /collections/:id/items/:post_id` | editors | Take a post out |
| `PATCH /collections/:id/items` `{ order: [...] }` | editors | Put them in a new order |
| `GET /collections/for/:post_id` | signed in | Which collections the viewer may edit hold this post, for the picker |
| The routes media share | as for any medium | `access`, `reply-access`, `quote-policy`, `pin` |

Likes, replies, reposts and quotes go through the services that serve them for every medium.

### Web

- **Making one:** a Collection form in the composer, with title, description and audience. It starts empty.
- **Adding to one:** on the Saved page, "Add to collection" on each card opens a picker of the collections the viewer may edit, with a tick beside each that already holds the post. It does not make a new collection.
- **Reading one:** the page any post has, with the items below in the masonry the other lists use, each a card of its own kind. Pressing one opens the light box, which steps through the collection in order, whatever kind each item is. The Stepper shows photos today and would have to show a note, an article and a topic as a slide.
- **Arranging:** for an editor, drag to reorder and a button to take an item out.
- **On a profile:** a Collections tab, for people, groups and events, shown when the feature is on for that actor.
- **In a feed:** a card with the title, the description and a few thumbnails or first lines.

## Still open

1. **The author's say.** Somebody saves another person's post and puts it in a public collection. Proposed: a collection others can see may hold a post only if its author allows it to be quoted (`permissions.CanQuote`), and an author can take their own post out, as they can detach a quote. The alternatives are no rule, or a setting of its own.
2. **Whether an author is told** that their post went into a collection others can see.
3. **Replies and other collections as items.** Proposed: neither. A reply reads poorly without what it answers, and a collection inside a collection has no end.
4. **A collection on a group:** whether an administrator adds from their own saved posts, as this page assumes, or from the group's posts.
5. **The reading of decision 7,** above.

## Estimate

Not scheduled. As a rough size: the service, graph RPCs and tests about 3 sessions; the web form, picker, page, arranging and light box about 3; 6 in all.

## How it would be checked

- A public collection holding a followers-only post shows that post to a follower and not to a stranger, in the list, in the feed preview and in the light box, and the stranger's count leaves it out.
- A post that has not been saved by the adder is refused. One that has is added at the end; adding it twice changes nothing.
- A new order is kept, and is refused when it names a post that is not in the collection.
- A follower of a group who is not an administrator cannot add to or arrange the group's collection.
- The collection is liked, replied to, reposted, quoted, pinned, reported and saved like a note.
- Deleting a collection leaves its posts alone. Deleting a post takes it out of every collection. Purging an actor removes its collections and their edges.
- With collection-service not running, saving still works and the web app shows nothing about collections.

## Depends on

- Saved posts (item 33, done).
- Quote posts (item 25, done), if the quote setting is used for the author's say.
- Events (item 12), for collections on events.
- Videos (item 16), for videos as items.

## Status

| Item | State |
| --- | --- |
| Idea recorded | 2026-10-06 |
| Shape decided by the user | 2026-10-06 |
| Open questions | five, above |
| Work | not scheduled, not started |
