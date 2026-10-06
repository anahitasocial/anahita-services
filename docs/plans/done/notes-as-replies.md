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
[Permissions › Comments](../../permissions.md#replies). Replies have to keep
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

### The post's author moderates their thread (added 2026-10-04)

Asked for by the user on 2026-10-04: whoever wrote the post a thread hangs
from can remove a reply from it, for everybody.

The list above lets the owner of the *profile* delete a reply. That covers a
post on your own profile, and it leaves a gap: somebody who posts in a group
is not its administrator, so they could not act on replies under their own
post. So the **author of the root post** is added to both actions below,
wherever the post is.

- **Hide.** The reply no longer appears in the thread, for anybody, and its
  own replies go with it. It is not destroyed: it stays on its author's
  profile under "Posts & replies", and its author is not told. The post's
  author can show it again. This is Bluesky's "hide reply".
- **Delete.** The reply is removed for good. Allowed for a reply that lives on
  this installation.

A reply that arrived from another server can only be hidden: the copy there
is not ours to delete. That is why both exist, and why hide is the one the
interface leads with. Until federation, every reply is local and both work.

Storage: a hidden reply is one edge from the root post to the reply,
`edge.medium.hidden-reply.v1`, created by whoever hid it. An edge and not a
flag on the reply, because the reply belongs to its author and "hidden" is a
fact about the thread. Every thread listing leaves out replies with that edge
on their root; the count on the post leaves them out too. On Bluesky it is
sent as the post's threadgate `hiddenReplies`; ActivityPub has no equivalent,
so there it only affects what this installation shows.

Who may hide or delete a reply in a thread: the root post's author, the owner
or administrators of the profile it is on, and site administrators. The reply's
own author can delete it, as before, and has no use for hiding it.

Phase 1 builds it, with the thread view: "Hide reply" and "Delete reply" in the
reply's menu, and a "Hidden replies" line at the foot of the thread, which only
the people who may hide can open.

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
- Update [Permissions](../../permissions.md): comments become replies.

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
| Decisions 1–5 | decided 2026-09-28 (see [Decisions and schedule](#decisions-and-schedule-2026-09-28-roadmap-item-7)) |
| Phase 1 | done 2026-10-05, on branch `m5-conversations`; see [What was built: phase 1](#what-was-built-phase-1-2026-10-05) |
| Phase 2 | done 2026-10-05; see [What was built: phase 2](#what-was-built-phase-2-2026-10-05) |
| Who can reply, per post | done 2026-10-05; see [What was built: who can reply](#what-was-built-who-can-reply-2026-10-05). Asked for by the user after phase 2; this is the "reply controls" that decision 4 left for later |
| Phase 3 | done 2026-10-05; see [What was built: phase 3](#what-was-built-phase-3-2026-10-05) |


---

## What was built: phase 1 (2026-10-05)

Replies exist alongside comments. Nothing was migrated; comments work as before. Described for people using it in `docs/permissions.md`, Replies.

**Graph.**
- `nodes.root_id`, indexed (migration `20261010000001_nodes_root_id.sql`, `init.sql`, `legacy/upgrade.sql`).
- **One gate for a post or a reply,** `postAccessCondition`: a post at the top level by the medium gate, a reply by its root. Every listing of posts goes through it: media lists, feeds, the check of one node's visibility. Search and the pages of hashtags and places write their gates out by hand; they apply their own gate to a post and the root rule to a reply (`postOrReply`).
- Adding a post with a `parent_id` makes it a reply. The graph works out the root from the parent and never takes it from the caller, since the root decides who sees the reply.
- `Thread` returns every reply under a post the viewer may see, oldest first. `HideReply` writes or removes the `edge.medium.hidden-reply.v1` edge.
- **Counts are counted, not adjusted.** `comment_count` on a root is recounted from the rows whenever a reply is added, removed or hidden: replies that are neither removed nor hidden, plus the comments there still are. On a reply it is the replies made directly to it.
- Replies are left out of feeds, and out of a list of notes unless `include_replies` is asked for.

**Rules** (`anahita-libs/cmd/api/permissions/reply.go`): `CanSeeThread`, `CanReply`, `CanEditReply`, `CanDeleteReply`, `CanHideReply`, `CanLikeReply`, `LooksAfterThread`. Functions over a context holding the root, not methods of `MediumPermissions`: handing a reply to that judges it by its author's profile.

**text-service.** `GET /notes/:id/thread`, `POST /notes/:id/replies`, `PATCH /notes/:id/hidden`. `GET`, `PATCH` and `DELETE /notes/:id` follow the thread's rules when the note is a reply. A reply's audience and comment switch cannot be changed (422 `reply_follows_its_thread`).

**like-service.** A like on a reply is decided by its root.

**notification-service.** A reply tells the author of the root, and the author of what was directly answered, with a `reply_add` notification pointing at the root. It no longer tells the replier's followers that a note was posted, which is what a note arriving would otherwise do.

**Web.** A Replies tab on a note's page: the thread nested by what answers what, a box to reply to the post or to any reply, edit, delete, hide and show, like, report. Hidden replies are listed apart, for whoever may show them again.

**Where it differs from the plan.**
- **A removed reply that has been answered is kept as an empty row** marked with `deleted_at`, the column actors use for the same idea. It is shown only in its thread, as a placeholder with no text and no name.
- **A reply's access is the empty string, not NULL.** The column does not allow NULL. Nothing reads it.
- **The scope a reply needs is the one for posts** (`media:write`), not the one for comments.
- **The switch for comments does not stop whoever looks after the thread.** The plan listed the open switch as a condition and then said those people can always reply; the second was taken as the rule.
- **Likes on replies are drawn by the thread itself,** not by the like control posts use, because the thread is kept on the page and not in the store.
- **"Posts & replies" on a profile is not built.** The list can include replies (`include_replies`); nothing asks for it yet. It belongs with item 24.

**Tests.** Against real MySQL: who sees which notes through the medium gate, as a visitor, somebody outside a private group, a follower of it and the author; that a reply does not get through search or a tag page to somebody who cannot see its root, and does to somebody who can; one-by-one visibility; a list of notes with and without replies; a thread read whole and in order, with a blocked person's reply left out; counts, hiding and showing; a removed reply kept as a placeholder and gone from every other listing. Without a database: the reply rules; placing a reply in a thread; what a thread shows to most people and to whoever looks after it. In the web app: arranging a thread.

**Not checked.** Anything that needs a signed-in session: posting a reply, editing, deleting, hiding, liking, and the notification. Reading was checked on the dev stack; nothing in a browser.

**Known limits.**
- A thread is read whole, up to 500 replies. Longer ones are cut, and say so.
- A reply under a hidden reply is still counted in the post's number. Only the reply that was hidden is left out of the count.
- Removing a reply does not remove the notification that it was made.
- On photos, articles and topics the old comments are still what is shown. That is phase 2.

---

## What was built: phase 2 (2026-10-05)

Every comment is a reply. `comment-service` is still deployed, with nothing to serve.

**The migration,** `20261011000001_comments_become_replies.sql`, and the same statements in `legacy/upgrade.sql`. On the development database:

| | |
| --- | --- |
| Comments turned into replies | 10,400 |
| Comments deleted because their post no longer exists | 10 |
| Switched back on, under a post that is still published | 1,572 |
| Replies switched on afterwards | 3,168 of 10,403 |
| Edges brought up to date | 2,690 likes, 314 hashtags, 216 mentions |
| Posts whose count was wrong before and is right now | all of them: no post's count differs from the replies under it |

- Each becomes a note owned by its author, a direct reply to the post it was under, with no audience of its own. Its id does not change.
- `parent_type` is brought up to date as well. Comments still carried the PHP application's name for their post's type.
- 1,253 have no owner, because their author's account no longer exists. They are shown without a name, as they were.
- Every post's count is counted again from what is under it, and posts with nothing under them are set to zero. Eleven carried a number from comments deleted long ago.

**Rehearsed on a copy of the development data** before it was applied: run, checked, run again (nothing changes), timed. A backup of the development database was taken first, `db-data/dev-before-comments-become-replies-2026-10-05.sql.gz`. There is no other way back.

**Web.** The reply thread is what a post's page and the lightbox show under every kind of post. The Comments tab is gone.

**Found on the way.**
- **The first version took over five minutes.** One statement deleted the edges of orphaned comments by joining on "this end or that end", which MySQL cannot use an index for. As two statements, one for each end, the whole migration takes two seconds.
- **The first version left 2,126 posts with the wrong count.** It recounted only posts that had a reply to count, so a post whose replies are all switched off kept its old number. Both were found by the rehearsal.
- **A thread sent replies without what the viewer may do with them** (phase 1), so nobody was offered edit or delete on their own reply. Found by the user. A thread now sends each reply in full, and a test reads what is sent.

**Not done in this phase.**
- **Reposting a reply, shown with the post it answers.** Nothing stops a reply being reposted through the API; the thread has no button for it and a feed does not show the context.
- **Old links to a comment.** There was no page for a single comment to redirect from; a comment's link was to its post, which still works.
- The comment code is all still there: comment-service, the comment branches in like, search, feeds and notifications, and the web app's comment components. That is phase 3.

**Checked on the dev stack.** Threads of a photo and a topic that had comments return them as replies, with authors and avatars; the photo's count matches; feeds and a list of notes contain no replies. Nothing in a browser by me.

## What was built: who can reply (2026-10-05)

A post's author chooses who may reply to it, the way Bluesky's "post
interaction settings" do: anyone, nobody, or any of the people who follow
them, the people they follow, and the people the post mentions. The rule is
in `docs/permissions.md`, Who the author lets reply.

- **One column**, `nodes.reply_access` (migration
  `20261012000001_nodes_reply_access`, and the same in `schema/init.sql` and
  `legacy/upgrade.sql`). Empty means the author has not said. No rows are
  changed: a post closed with the older switch reads as `nobody` without the
  column being filled in.
- **The rule** is `permissions.CanReply`, which now also reads the author's
  relationship to the viewer (`RootAuthorViewerRel`) and the post's mentions.
  It narrows the profile's rule and never widens it, and whoever looks after
  the thread replies whatever is set.
- **Set with the post or afterwards**: `reply_access` on creating an article,
  note, topic or photo post, and `PATCH /…/:id/reply-access`. The older
  `PATCH /…/:id/comment-status` still works and keeps the two in step.
- **Web**: a dialog (`components/ReplyAccessDialog.jsx`) opened from a button
  in the composer beside the audience and the language, and from "Who can
  reply" in a post's menu, which replaces "Close comments". The thread says
  who can reply when it is limited.

**Decisions made while building, to revisit if they turn out wrong.**
- **Measured against the author, not the profile.** In a group, "followers"
  are the author's. Bluesky has no groups to compare with; the group's own
  rule (followers only, or anybody registered) is already the profile's.
- **A mention is matched by name in the post's text as it is now.** Editing a
  post to take a name out takes away that person's right to reply; what they
  already said stays.
- **Not remembered between posts.** The composer starts on "anyone" every
  time.
- **`authorized.comment` on a post in a list is still the profile's rule and
  the switch**, without the groups: a list does not read the author's
  relationship for every post. The thread's `can_reply` is the full answer,
  and the server refuses a reply either way.

**Left out, for want of what they depend on.** Bluesky's "select from your
lists" (there are no lists) and "allow quote posts" (item 25).

**Checked.** Unit tests for the rule and the values; the column read, written
and left alone by an unrelated edit, on a real database; the migration run
twice and against a table without the column. On the dev stack a post and its
thread return `reply_access`, and the new routes refuse a visitor. Nothing
signed in, and nothing in a browser, by me.

## What was built: phase 3 (2026-10-05)

Comments are gone from the code. Nothing was changed in the database: phase 2
left no comment rows to remove.

**Removed.**
- **comment-service**: its code, its image, and its entries in the compose
  file, the gateway (`/comments/` now answers 404), the Kubernetes base and
  overlays, the Makefile and `go.work`.
- **The shared library**: `CommentPermissions`, the comment response, the
  comment node type and service name, the comment events, and
  `AnahitaNode.IsComment`.
- **The graph**: the comment branch in the post gate, the feed gate, the
  mixed-kind gate and search; the "include comments" option on search and on
  both feeds (the protobuf field numbers are reserved); the comment half of
  the count a post keeps.
- **like-service, notification-service and the event listeners** of the
  hashtag, actor and abuse report services: every comment branch and every
  subscription to comment events.
- **The web app**: the comment components, actions, reducers, API modules and
  prop types, the comment like controls, the comment card in search and in
  the feeds, the "include comments" switch on the search page, and the older
  open-and-close control. Wording that said "comment" says "reply", in English
  and French.

**Changed rather than removed.**
- **`authorized.comment` on a post** is now the reply rule
  (`permissions.CanReplyToPost`), not the comment rule. It needs `media:write`
  where it needed `comments:write`, and it follows who the author lets reply
  when the post is on its author's own profile. The field keeps its name.
- **The reply button in a feed** did nothing: it was the comment button, left
  from before threads. It is now a link to the post, where the thread is.
- **What deleting an account would cost** counts replies, and no longer counts
  a reply as a post. The field is `replies` where it was `comments`.

**Found and fixed: a purge did not recount replies.** Deleting an account for
good recounts what other people's posts show. It looked for comments by their
type, found none after phase 2, and left the number of replies on a post as it
was, counting replies that were gone. It now lists every post that loses a
reply, and every reply that loses an answer, and counts both by the rules the
thread itself uses. A test purges somebody from the middle of a thread.

**Kept on purpose.**
- **The `comment` permission** in `actor_features`, and the columns
  `comment_count` and `comment_status`: what profiles saved still applies, and
  item 27 is where columns are renamed.
- **`notifications.comment_id`**, which a reply notification fills with the
  reply. Notifications from before, of the kinds `comment_add` and
  `comment_edit` and under the service name `comment-service`, are still
  stored and still shown and removed with their post.
- The comment types named in older migrations and in the legacy upgrade,
  which is what they convert.

**The `comments:write` scope is removed** from the catalog and from the
seeded clients, in a step of its own after this phase, once the user had said
there is no live installation. Client records made before then still name it:
they are cleared and seeded again.

**To do when deploying to Kubernetes.** The manifests no longer describe
comment-service, and `kubectl apply -k` does not remove what it no longer
describes: delete its Deployment and Service by hand.

**Still not built** (from phase 2): reposting a reply with the post it
answers, and a page for a reply on its own. A link to a reply, from search or
from a report, opens it as a note without its thread.

**Checked.** Every service builds and passes its tests; the graph's against a
real database, with the fixtures that were comments rewritten as replies. The
web app's 24 suites pass and it builds. On the dev stack, rebuilt without the
service: a post, its thread, search and a feed answer, and `/comments/`
answers 404. Nothing signed in, and nothing in a browser, by me.

---

## Decisions and schedule (2026-09-28, roadmap item 7)

### Verdict: yes, it's a good idea
- **One kind of content means one set of rules.** Most of the September 2026 permission bugs existed twice, once for posts and once for comments. Replies get likes, reposts, hashtags, mentions, editing and search with no code of their own, and threads come for free. Comments today are flat: 0 comments reply to another comment.
- **It removes a whole service** (comment-service) and every comment branch in like-service, search, the leaders feed, notification-service, anahita-libs permissions and the web app.
- **The Instagram, Facebook and TikTok model** (comments as second-class content under media) doesn't fit a federated network. The flat look under photos can stay, as a presentation choice.

### Fediverse fit: very good
- **ActivityPub / Mastodon:**
  - A reply *is* a `Note` with `inReplyTo`, owned by its author. That's exactly the existing plan's decision 2 (owner = author).
  - Replies to an `Article` or a photo post are also Notes with `inReplyTo`.
  - Lemmy and other group-based servers (FEP-1b12) call them "comments" on screen, but on the wire they're the same Note plus `inReplyTo`.
- **Bluesky / AT Protocol:** a reply is an `app.bsky.feed.post` with `reply: { root, parent }`. That's one-to-one with the plan's `root_id` + `parent_id`.
- **Things to design with federation in mind** (add to the existing plan's Risks):
  1. **Moderation of remote replies is "hide", not "delete".** A profile or post owner can't delete someone else's reply that lives on another server. Bluesky has "hide reply" and threadgates, and Mastodon lets you filter. So thread moderation should be modelled as hide-from-thread for remote replies, and delete for local ones. Anahita doesn't need this until it federates, but the permission naming shouldn't assume delete is always possible.
  2. **Replies inherit the thread's audience** (existing decision 3). ActivityPub allows narrower audiences per reply, which causes half-visible threads. Inbound replies wider than their root should be clamped to the root's audience.
  3. **Reply controls** (existing decision 4, "later") map to Bluesky threadgates and Mastodon's quote/reply policies when federation arrives.

### Name
- **"Replies"** in the UI: "Reply", "12 replies", a "Posts & replies" profile tab, and "Who can reply?" on the Permissions tab. The entity stays `note`, with `parent_id` / `root_id` in the data.
- Per the existing plan, the permission key stays `comment` in `actor_features`, so saved settings keep working. Only the label changes.

### Decisions to record in `notes-as-replies.md`
1. **Federation is a goal**, which settles decisions 2 and 3 in the ActivityPub shape.
2. **The reply's author owns it** (`owner_id = created_by`).
3. **No per-reply audience;** replies inherit the root's (`access = NULL`).
4. **Reply controls come later.** Keep the profile comment setting plus the post's open/closed switch.
5. **Re-enable the ~1,625 disabled pre-2016 comments** under published posts during the migration. It can run separately before Phase 2. The disabled comments under unpublished posts stay as they are.
6. **UI name: "Replies".**
7. **Replies can be reposted,** shown with their parent post above them, as Bluesky does.
8. **The author of a post can hide or delete any reply in its thread** (2026-10-04), wherever the post is. See *The post's author moderates their thread*. About 3–4 hours on top of Phase 1.

### Interplay with other items (to note in the roadmap)
- **Item 0 (feed gate fix) first.** Replies depend on the root-access rule being applied in every listing, and the feed gate is exactly where that is broken today.
- **Item 1 (delete private notes) first.** It deletes 604 comments; better to delete them before they become notes.
- **Item 5 (complete purge):** with owner = author, a purge walking `owner_id` already catches replies. Keep the `created_by` walk anyway, for posts on other people's profiles. Doing item 7 first simplifies item 5, but either order works.
- **Item 4b (abuse reports):** the reasons JSON's `applies_to` should use `node.medium.` rather than naming comment types. If 4b ships first, include both, and remove the comment type after Phase 3.
- **Item 3 (recommender):** the "you engaged" signal counts replies (notes with `parent_id`) instead of comments.
- **Item 2 (remove global browse):** already done by then. Note lists default to top-level only (`parent_id IS NULL`).

### Estimate (the existing plan has none)

| Phase | Scope (from `notes-as-replies.md`) | Estimate |
| --- | --- | --- |
| 1: replies alongside comments | `root_id` column + index; root-based access in the graph filter and every listing; `root_id` and `parent_id` browse filters; feeds exclude replies; `permissions.CanReply` and root-based edit, delete and like; text-service `POST /notes/:id/replies` and `GET /notes/:root/thread`; web thread view; real-MySQL access tests | 12–15 h (3–4 sessions) |
| 2: switch comments over | Migration of 11,066 comments plus retyping 2,743 + 327 + 223 edges, with the pre-2016 re-enable; reply events in notification-service; the web comment UI on photos, articles and topics rewired to replies (99 files, flat look kept); reposting replies with context | 12–16 h (3–4 sessions) |
| 3: remove comments | Delete comment-service (nginx, compose, k8s) and the comment branches (like, search, leaders feed, `CommentPermissions`, constant, web containers, actions, reducers, API); update `docs/permissions.md`; redirect old comment URLs to their notes | 4–6 h (1–1½ sessions) |
| **Total** | | **about 28–37 h, 7–9 sessions** |

### Verification (in addition to the existing plan)
- The real-MySQL access tests cover:
  - a reply to a private root, seen through the author's public profile, stays hidden in every listing (feeds, profile lists, search, hashtags, mentions, locations);
  - a reposted reply is hidden when its root is.
- **Migration:** counts before and after (11,066 comments → notes with `parent_id`; the edge counts match), and a second run changes nothing. Old `/comments/...` links redirect to the note.
- **Manual:** reply to a photo (flat look), reply to a note reply (nested thread), like and repost a reply, and delete a reply that has replies (a placeholder stays).

### Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **Phase 1:** **services `permissions.md`**: a *Replies* section (visibility by root, who can reply, edit and delete).
- **Phase 2:** **services `permissions.md`**: *Comments* becomes *Replies* ("Who can reply?"); **web `architecture.md`**: the thread view and the flat look on photos.
- **Phase 3:** **services `architecture.md`**: comment-service removed from *Services* and *Routing*; **services `docs/README.md`**: the *Permissions* entry mentions replies, not comments.
