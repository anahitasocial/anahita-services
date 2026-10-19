# Notifications: right, then worth acting on

*Roadmap rows 96 and 97, planned 2026-10-19 from an audit of notifications
in both repositories, asked for by the user the same day ("our
notifications need some love"). See the
[roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md)
for order and dependencies.*

## Context

A notification is a row in `notifications`, one for each person told,
written by notification-service when it hears an event on `anahita_events`:
who did it (subject), to what (object), where (target), and, for a reply or
a quote, the reply itself (`comment_id`). The list fills the nodes in when
it is read, so no text is copied into a row. The web app shows a bell with a
count, and a page of notifications.

That much is sound, and stays. The audit found the rest wanting:

- **What a notification shows is not checked against who may see it.** The
  list reads posts with `GraphNodeGRPC.BatchRead`, which checks nothing, and
  returns their text. New posts and edits are told to every subscriber of a
  profile whoever the post is for. Blocking leaves the blocked person
  subscribed.
- **Notifications outlive what they are about.** Deleting a post removes
  only the rows whose subject is whoever deleted it: likes, reposts,
  mentions and replies stay, and all of them when a moderator deletes it.
  Unfollowing an event takes the RSVP notifications with it.
- **Delivery can double a row or drop one.** No key stops a repeated event
  writing a second row; the consumer acknowledges each message as it
  arrives, drops a handler's error, does not recover a panic and does not
  say when its connection closes.
- **In the web app,** Stop Notifications on a post throws, the settings for
  a profile's notifications call routes the services do not have and are
  never shown, the bell does not ask until its first interval has passed
  and stops for good after signing out and in, seven kinds the server sends
  have no words, and deleting from the list breaks its paging.
- **Email** is not sent for notifications, and must never be: auth-service
  alone sends mail, for accounts, invitations, signups and a group's
  deletion. Of those, the emails to administrators reach only twenty of
  them, and the group emails go to disabled administrators and to the one
  who acted.
- **No tests** of any of it, in either repository, and no page in the docs.

## Decisions

Settled with the user on 2026-10-19:

- **No email for notifications.** In-app only, and a test keeps it so: no
  service but auth-service and mail-grpc-service may use the mail client.
- **No sockets.** The bell asks, as the feed does, but more cleverly: see
  [The bell asks less](#the-bell-asks-less). Server-Sent Events are the
  next step if live updates are ever wanted, with the asking kept as the
  fallback; not planned.
- **Subject, object and target stay.** They are ActivityStreams' actor,
  object and target, and one table and one way of showing a notification
  cover every kind.
- **Actions inside notifications** where there is an obvious next step
  (row 97).

Proposed in this plan, for the user to overrule:

- **A notification about something the reader may no longer see is
  deleted** when the list finds it, as a reply's are when it goes: one that
  leads nowhere is noise, and the unread count then agrees with the list.
- **Events are still never redelivered.** Retrying a handler that fails
  part way would write its rows twice and, if mail were ever sent from one,
  send it twice, which is what happened on anahita.io on 2026-10-15. The
  comments in auth-service that say otherwise are put right.

## Row 96: notifications that are right

### Part 1. Who may see it

1. **The list checks every node it fills in.** Browse and Edit ask the
   graph's `FilterVisible` for the subject, object and target of every
   notification, for this viewer, and read only those. A notification whose
   object or target the viewer may not see, or whose subject they may not
   (blocked, disabled), is deleted and left out.
2. **New posts and edits are told only to who may see them.** The handlers
   for `<entity>_add` ask `FilterVisible` once for each
   subscriber, as mentions do; replies and quotes the same, for the people
   they are told to.
3. **A block ends the subscriptions both ways,** with the follows, in
   graph-grpc-service's block handler, and notification-service deletes the
   notifications between the two on `edge.block.*.added`.

### Part 2. Cleaning up after the graph

4. **A deleted post takes every notification about it,** by object or by
   `comment_id`, whoever is subject or target: likes, reposts, quotes,
   mentions and the replies under it. So does a repost taken back, from the
   ids in the event, without reading the repost that is gone.
5. **A mention taken out of a post in an edit takes its notification,** on
   `edge.mention.*.deleted`.
6. **Unfollowing deletes only the follow's notification,** and leaving an
   actor's administrators only the administrator's: each delete names its
   type.
7. **Subscriptions change only with what made them.** Losing administrator
   rights unsubscribes only somebody who no longer follows; unliking leaves
   subscriptions alone. Nobody is subscribed to themselves on each post.
8. **Subscribing to a post works:** a reply under it is told to its
   subscribers, as to its author.

### Part 3. Delivery

9. **One row for one thing.** A unique key on recipient, type, subject,
   object, target and `comment_id`; a repeated event makes no second row,
   and an edit told again moves its row to the top, unread.
10. **The consumer** (anahita-libs `event`) recovers a panic in a handler,
    logs a handler's error, and says when its connection closes. It still
    acknowledges on arrival.
11. **The table** gains an index on `(recipient_id, is_read)` for the count
    and `(recipient_id, created_at)` for the list; `comment_id` becomes
    `bigint`, as every id is. The unused `meta` column goes. The
    notifications there are go with the change, and the PHP site's are not
    carried over: none is worth keeping (the user, 2026-10-19).
12. **The mail rule as a test:** a Go test fails if any service but
    auth-service and mail-grpc-service imports the mail client.

### Part 4. The web app

13. **The bell**, rebuilt as [The bell asks less](#the-bell-asks-less)
    says, with its count in the store, so that reading or deleting one
    changes it at once. Its button is labelled with the count.
14. **Words for every kind the server sends** (`video_add`, `admin_add`,
    and an edit of each kind of post), the ten for kinds it never sends
    removed, and a plain fallback for a kind the app does not know. The
    hard-coded English strings move into the bases.
15. **The dead settings go:** a profile's notification settings dialog, its
    page and routes, and the API calls to `/notifications/settings` and
    `togglesubscription`, none of which reach anything. Choosing what one is
    told about is row 97.
16. **Get notifications and Stop notifications on a post work,** with the
    subscription's own state, and say when they fail.
17. **The list:** deleting keeps its paging right (the total lowered, the
    next page from the right place), the item menu carries icons, unread is
    said in words to a screen reader, links stay in the app, and an empty
    list says so.

### Part 5. Email

18. **Emails to administrators reach all of them,** not the first twenty.
19. **A group's deletion and restoring are emailed** only to administrators
    whose accounts are enabled, and not to the one who did it.
20. **The mail client keeps one connection,** instead of one left open for
    each email, and an SMTP connection over TLS gives up after a timeout.

### Documentation

- anahita-services: a new `docs/notifications.md` (the kinds, who is told,
  who never is, how they are cleaned up, that none are emailed);
  `docs/permissions.md` (who is told about a post); `docs/architecture.md`
  and `docs/README.md` (the page, the plans table); `docs/deploying.md` (the
  consumer); `docs/api/` (`make api_docs`).
- anahita-web: `docs/architecture.md` (the bell and the page),
  `docs/configuration.md` (`REACT_APP_NOTIFICATIONS_CHECK_INTERVAL`).

## The bell asks less

Asking is cheap: one count on an index, per open tab. It is made cheaper
still, and quicker when it matters.

1. **At once** when the app opens with somebody signed in, and when the tab
   is looked at again, the window comes into focus or the connection comes
   back.
2. **Less and less often while nothing is new:** 15 seconds, 30, 1 minute,
   2, then every 5 minutes for as long as the tab is looked at. Unlike the
   feed's heartbeat, it never stops on a tab being looked at, and it never
   asks on one that is not.
3. **Soon again after something new,** and after the viewer does something
   that tends to bring answers: posting, replying, following, answering an
   event. Back to 15 seconds.
4. **One tab asks for all of them.** The tabs of a browser choose one to
   ask, with the Web Locks API, and it tells the others over a
   `BroadcastChannel`; when it closes, another takes over. Where a browser
   has neither, each tab asks for itself.
5. **Spread out:** each wait is a fifth longer or shorter at random, so
   tabs opened together do not ask together.
6. **A failure keeps the last count** and waits longer, rather than turning
   the bell into an error.
7. **Nothing after signing out,** and the count goes with the session.
8. **The answer says what is newest** (`latest_id` beside `count`), so the
   list, when open, knows to fetch the new ones although the count is
   unchanged.

`REACT_APP_NOTIFICATIONS_CHECK_INTERVAL` becomes the longest wait,
5 minutes unless set.

## Row 97: notifications worth acting on

Planned with row 96, begun 2026-10-19. Settled with the user that day:

- **Read when opened or acted on:** clicking a notification, or using one
  of its actions, marks it read; seeing it in the list does not, so the dot
  keeps meaning "not looked at yet".
- **The bell opens a menu** of the latest eight on a desktop, with their
  actions, Mark all read and See all; on a phone it goes to the full page,
  as now.
- **Choices by kind of thing,** about eight switches, all on to begin with.
- **Read ones are kept 90 days** after they were read, unread ones until
  they are (`NOTIFICATIONS_KEEP_DAYS`, 0 for forever).

Built in five parts: (1) read when opened, `read_at` and how long they are
kept; (2) actions; (3) grouping; (4) the bell's menu; (5) choosing what one
is told about.

1. **Actions inside a notification,** one row of buttons, full width and
   side by side, the declining one on the left. Their state comes from the
   graph when the list is read, never from the row, so an answered request
   shows its answer instead:

   | Kind | Action |
   | --- | --- |
   | `actor_follow_request` | Decline · Accept |
   | `actor_invite` | Decline · Accept (as now) |
   | `actor_follow` | **+ Follow**, unless already following |
   | `reply_add`, `actor_mention`, `actor_mention_reply`, `quote_add` | Reply, in place |
   | `event_changed`, `event_reminder` | none, after all: whoever is told has answered already, and the answer buttons need the event's times and limits, which a notification does not carry; it opens the event |
   | the others | none: the notification opens what it is about |

   Acting marks it read.
2. **Grouped:** "Worf and 3 others liked your post". Each row keeps its
   subject; a `group_key` (such as `voteup:<post id>`) groups them when the
   list is read, for likes, reposts and follows.
3. **A notification is read when it is opened.** `read_at` in place of
   `is_read`. Mark all read (`PATCH /notifications/read-all`) was built on
   2026-10-19, with roadmap row 99.
4. **The bell opens the latest eight** in a menu on a desktop, with their
   actions, Mark all read and See all.
5. **Choosing what one is told about,** by kind of thing, in a
   Notifications tab of a person's settings: likes; reposts and quotes;
   replies; mentions; follows and requests; new posts from what one
   follows; events; invitations and administration.
6. **How long notifications are kept** is the operator's choice
   (`NOTIFICATIONS_KEEP_DAYS`, 90 unless set, 0 for forever): a read one
   goes that many days after it was read; an unread one stays.

## Status

| Part | State |
| --- | --- |
| Row 96, part 1: who may see it | ✅ 2026-10-19 |
| Row 96, part 2: cleaning up | ✅ 2026-10-19 |
| Row 96, part 3: delivery | ✅ 2026-10-19 |
| Row 96, part 4: the web app | ✅ 2026-10-19 |
| Row 96, part 5: email | ✅ 2026-10-19 |
| Row 97, part 1: read when opened, how long they are kept | ✅ 2026-10-19 |
| Row 97, part 2: actions | ✅ 2026-10-19 |
| Row 97, part 3: grouping | ✅ 2026-10-19 |
| Row 97, part 4: the bell's menu | ✅ 2026-10-19 |
| Row 97, part 5: choosing what one is told about | ✅ 2026-10-19 |
