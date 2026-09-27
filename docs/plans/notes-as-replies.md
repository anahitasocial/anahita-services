# Notes as replies: retire comments and comment-service

## Context

Comments are their own kind of node (`node.base.comment-service.comment.v1`),
served by their own service, with their own branches in like-service, search,
the leaders feed, notification-service, the shared responses and permissions,
and the web app.

A comment is, in practice, a short note that answers a post. Bluesky and
Mastodon model it exactly that way: a reply is an ordinary post with a parent.
That gives threads (replies to replies) for free, and every reply can be liked,
reposted, quoted, tagged, mentioned, edited and searched with no code of its
own.

It would also remove a whole class of bug. Most of the permission problems
fixed in September 2026 existed twice, once for posts and once for comments:
comments that anybody could add to posts they could not read, comment lists
that nobody but site administrators could see, like checks judged against an
unloaded owner. One kind of content is one set of rules to keep right.

**Outcome:** a comment is a note with a parent. Notes can reply to any post
(a note, article, topic or photo) and to other notes, to any depth. Comments,
comment-service and every comment branch elsewhere are gone.

### Where things stand (2026-09-27)

| | Count |
| --- | --- |
| Comments | 11,066, on 3,037 posts, at most 66 on one post |
| Comments that reply to a comment | 0 (comments are flat) |
| Edges pointing at comments | 2,743 likes, 327 hashtags, 223 mentions |
| Notes | 5,706 |
| Notes with a parent | 0 |
| Comments with `enabled = 0` | every one before 2016-01-30, about 9,400; 1,625 of them under posts that are still published |

The comment rules in force are documented in
[Permissions › Comments](../permissions.md#comments). Replies have to keep
them.

---

## Decisions to make first

1. **Is federation a goal?** In ActivityPub a reply *is* a `Note` with
   `inReplyTo`, owned by its author. If Anahita will ever federate, that settles
   decisions 2 and 3 below in favour of the ActivityPub shape. If it never
   will, they are still the recommendation, but for product reasons only.
2. **Who owns a reply.** Recommended: **the author.** It is their content:
   they edit and delete it, and it can appear under a "Posts & replies" tab on
   their profile. The alternative, owned by the profile the thread is on, keeps
   replies inside a group but makes them nobody's own. It also means the author
   is not the owner, which is the case the permission code has got wrong most
   often.
3. **Can a reply have its own audience?** Recommended: **not at first.** A
   reply's visibility is its thread's. See [Visibility](#visibility).
4. **Reply controls per post.** Bluesky lets the author of a post choose who
   may reply (everyone, people they follow, people mentioned). Recommended:
   later. The first version keeps today's rules, the profile's comment setting
   plus the post's open or closed switch.
5. **The pre-2016 disabled comments.** Re-enable the 1,625 under published
   posts, or migrate them disabled as they are. This does not depend on the
   rest of the plan, and can be decided and done before it.

---

## Design

### Data

A reply is a note, `node.medium.text-service.note.v1`, with:

| Column | Value |
| --- | --- |
| `parent_id` | The post or reply it answers |
| `root_id` (new) | The post at the top of the thread. The same as `parent_id` for a direct reply |
| `owner_id` | The author, per decision 2 |
| `created_by` | The author |
| `access` | NULL, meaning "the thread's audience". See below |

`root_id` is what keeps nested replies cheap. Without it, deciding whether a
reply is visible means walking up through its parents in SQL. With it, every
reply is one join to one row, whatever its depth. It needs an index.

A top-level note is one with no parent. `root_id` stays NULL on it.

### Visibility

**A reply is visible to exactly the people who can see its thread's root.**
Nobody may see an answer to something they are not allowed to see.

This is the rule comments follow today. The graph's access filter already
judges a comment by its post (`commentAccessCondition` in
`graph-grpc-service/data/repositories/access_medium.go`), by running the post
filter over the parent row. Replies use the same mechanism keyed on `root_id`.
Nothing about the rule is new; only the key changes.

Because the owner is the author (decision 2), filtering a reply by its own
owner would be wrong. It would show a reply to a private group post to anybody
who can see the author's profile. The filter must use the root's owner and the
root's access, never the reply's own.

`access` on a reply is NULL and means "inherit". Replies do not need a value of
their own to be correct. If per-reply audiences are ever added (decision 3), the
only choices that make sense are "same as the thread" and *narrower* than it,
never wider. The effective audience is then the narrower of the two. Mastodon
allows narrower replies, and it is a known source of threads where people see
half a conversation, so it should be added only if somebody asks for it. NULL
already means "inherit", so adding it needs no data migration.

### Who can reply

The rules for adding a comment carry over unchanged, judged against the
**root** post:

- the viewer can read the root;
- the root's comment switch is open (`comment_status`);
- on a public or registered profile, the profile's comment setting:
  `registered` or `followers`;
- on a followers-only profile, followers only; on anything narrower, anybody
  who can see it;
- the profile's owner and administrators, and the root's author, always can.

`permissions.CommentAccess` and `CommentPermissions.CanAdd` become
`permissions.CanReply`, taking the root. The permission entity keeps the name
`comment` in `actor_features` and on the Permissions tab ("Who can comment?"),
so saved settings keep working. The word on screen can change later.

### Editing and deleting

As for comments today:

- **Edit:** the reply's author only, while they can still read the root.
- **Delete:** the author; the owner of the profile the root is on, or its
  administrators; site administrators.

That is not how an ordinary note is governed, because the profile a thread
lives on has to be able to moderate it. So a reply's edit and delete checks
use the root's profile, not the reply's owner. That is one condition in
`MediumPermissions`, keyed on `root_id`.

Deleting a reply that has replies of its own leaves them in place, under a
"deleted" placeholder, as Bluesky and Mastodon do. Deleting the root hides the
whole thread.

### Likes, reposts, hashtags, mentions

All come for free, because a reply is a note. like-service loses its comment
branch: a reply is liked under the rules of its **root** post, which is the
rule for comments today, keyed on `root_id` instead of `parent_id`.
`permissions.CanLike` takes the root.

Whether replies can be *reposted* is a product choice. Bluesky allows it;
reposting a reply shows it with its context. Nothing technical prevents it.

### Counts

`comment_count` on the root becomes the reply count for the whole thread. A
per-reply count of direct replies is useful for "3 replies" under a reply, and
can be derived or kept the same way.

The count update after adding or deleting a comment currently fails
(`invalid modifier ID`), so counts are stale today. The reply code must set the
modifier when it writes the count.

### Feeds and profile lists

Replies must not flood timelines. By default, feeds and a profile's note list
show **top-level notes only** (`parent_id IS NULL`). Replies are seen in their
thread. A profile can offer a "Posts & replies" tab that includes them.

The leaders feed's current option to include comments by people the viewer
follows becomes "include replies". It keeps the root visibility rule.

### Notifications

`node.comment.comment.comment.added.v1` and its siblings become reply events
on notes. notification-service's `event_node_comment.go` becomes
`event_node_reply.go`. Recipients: the root's author and owner, the parent's
author if different, and anybody mentioned, as now.

### Search

Replies are notes, so search finds them through the note path. Its comment
branch and `comment_parent_node` gate go away. That gate currently also applies
the comment's own empty access, which is why comments never appear in search
today. The reply version must use the root rule above.

---

## Phases

Each phase can ship on its own. The first is a clean place to stop.

### Phase 1: replies exist alongside comments

Nothing is migrated. New replies are notes, and old comments keep working.

- **Schema:** add `root_id` to `nodes`, indexed. Allow `parent_id` on notes.
- **Graph:**
  - the access filter judges notes with a `root_id` by their root;
  - Browse can filter by `root_id` (a whole thread) and by `parent_id` (direct
    replies);
  - feeds and note lists exclude replies unless asked for them.
- **anahita-libs:** add `permissions.CanReply`. Point edit, delete and
  `CanLike` at the root for a reply. Replies get the `comment`/`like` flags that
  posts already carry.
- **text-service:**
  - `POST /notes/:id/replies` creates a reply. It sets `parent_id`, `root_id`,
    owner and author, and updates the root's count with a modifier.
  - `GET /notes/:root/thread` returns a thread.
- **Web app:** a thread view for notes. The existing comment UI is untouched.
- **Tests:** add the thread rules to the graph's real-MySQL access tests
  (`access_integration_test.go`). Cover a reply to a private root seen from
  the author's public profile, which must stay hidden.

### Phase 2: switch comments over

- **Data migration**, one script, reversible from a backup:
  - comments become notes: `type`, `service = 'text-service'`;
  - `owner_id = created_by`;
  - `root_id = parent_id`, since every comment is a direct reply to a post;
  - `access = NULL`;
  - the pre-2016 `enabled` flags as decided in decision 5;
  - `node_a_type`/`node_b_type` on the likes, hashtags and mentions that point
    at comments: 2,743, 327 and 223 edges today.
- **Events:** notification-service consumes the reply events.
- **Web app:** the comment UI on photos, articles and topics reads and writes
  replies. It can keep looking like comments on those types, flat with no
  nesting, while notes show full threads.
- **Deployment:** comment-service stays deployed but unused, as a way back.

### Phase 3: remove comments

- Delete comment-service: its routes in nginx, its compose entry and
  `k8s/04-comment.yml`.
- Remove the comment branches:
  - like-service (`IsComment`);
  - search (`comment_parent_node`);
  - the leaders feed;
  - `CommentPermissions` in anahita-libs;
  - the `NodeBaseCommentEntityType` constant;
  - the client's comment containers, actions, reducers and API.
- Update [Permissions](../permissions.md): comments become replies.

---

## Risks

- **Visibility regressions.** The whole design rests on replies being judged
  by their root. Every listing that can return a note must apply the root rule:
  feeds, profile lists, search, hashtags, mentions, location views. A listing
  that forgets it leaks a reply to a private post. The real-MySQL access tests
  are the guard, and each new listing needs a case there.
- **Owner as author.** Code that assumes a post's owner is the profile it
  appears on will be wrong for replies. The permission checks, moderation and
  the author's-profile view each need to be checked for it.
- **Old links.** Comment ids stay the same when they become notes, so a link
  to a comment can redirect to its note. Nothing that stored a comment URL
  should break silently.
- **Scale of the client change.** 99 client files mention comments, though many
  are translations. Phase 2 keeps the comment look on non-note posts to limit
  what changes at once.

---

## Status

| Item | State |
| --- | --- |
| Decisions 1–5 | open |
| Phase 1 | not started |
| Phase 2 | not started |
| Phase 3 | not started |
