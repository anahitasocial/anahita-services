# Notifications

What people are told, in the app, about what others do: who is told what,
who never is, how a notification goes when what it tells of goes, and how
they are delivered. The plan they follow is
[plans/notifications.md](plans/notifications.md) (roadmap rows 96 and 97).

## No email

**Notifications are never emailed.** They are in the app only, under the
bell. The only emails Anahita sends are auth-service's, each about one thing
that happened: codes, password resets and security alerts; invitations;
signup requests and answers; a group or an account scheduled for deletion.

A test keeps it so: `anahita-libs/data/services/mail_senders_test.go` fails
if any service but auth-service and mail-grpc-service uses the mail client.

## What a notification is

A row in `notifications`, one for each person told, written by
notification-service when it hears an event on the bus (`anahita_events`).

| Column | Holds |
| --- | --- |
| `type` | What it tells of: the kinds below |
| `recipient_id` | Who is told |
| `subject_id` | Who did it |
| `object_id` | What it was done to or with: a post, for most kinds |
| `target_id` | Where: the profile it belongs to |
| `comment_id` | What holds what was done, when that is not the object: a reply, or a repost |
| `is_read` | Whether the recipient has read it |

Nothing is copied into a row: the list fills the nodes in when it is read,
so a notification always shows a post as it is now.

**One row for one thing.** The unique key `one_of_each` (recipient, type,
subject, object, target and `comment_id`) keeps a repeated event from
making a second row. Telling somebody the same thing again moves the row to
the top of their list, unread: an event moved twice says so once.

## The kinds

| `type` | Told when | Who is told |
| --- | --- | --- |
| `note_add`, `article_add`, `photo_add`, `topic_add`, `video_add` | A profile posts | Its subscribers who may see the post |
| `note_edit`, `article_edit`, `photo_edit`, `topic_edit`, `video_edit` | A post is edited | Its profile's subscribers who may see it |
| `reply_add` | A reply is made | The author of the post at the top of the thread, the author of what it answers, and the post's subscribers, each when they may see the thread and the reply |
| `quote_add` | A note quotes a post | The quoted post's author, when they may see the note |
| `repost_add` | A post is reposted | Its author |
| `voteup` | A post is liked | Its author |
| `actor_mention`, `actor_mention_reply` | A post or a reply names a person | The person, when they may see it ([Permissions](permissions.md)) |
| `actor_follow` | A profile is followed | The person, or a group's or an event's administrators |
| `actor_follow_request`, `actor_follow_request_accept` | Somebody asks to follow, and is let | The person or the administrators; then the one who asked |
| `actor_invite`, `actor_invite_accept` | Somebody is invited to follow, and accepts | The one invited; then the one who invited |
| `admin_add` | Somebody becomes an administrator of a profile | Its other administrators |
| `event_rsvp` | Somebody answers an event | Its administrators |
| `event_changed`, `event_cancelled`, `event_reminder` | An event moves, is called off, or is near | Each person who answered it |

**Subscribers.** Following a profile, or becoming one of its
administrators, subscribes to it; each is a reason to stay subscribed, and
the subscription ends with the last of them. Get notifications, in a post's
menu, subscribes to that post: its replies are told.

## Only what its reader may see

Nobody is told of a post they may not see, and nobody is shown one:

1. **When a notification is made,** each recipient is asked about, as a
   registered member, with `permissions.CanReadEach`: the post, and the
   reply that holds what was done. A post for a profile's followers is told
   to its followers, not to everybody subscribed; one for its
   administrators, to its administrators. Asked as a member, a site
   administrator is told what they would be as one.
2. **When the list is read,** the viewer is asked about every post and
   reply in it at once. A notification about one they may no longer see (it
   was made private, they left its audience, or it is gone) is left out and
   deleted, so that the unread count says what the list does. When the
   question cannot be answered, the list is empty rather than unchecked.
3. **A block** ends the follows and the subscriptions between the two,
   both ways, and what each was told of the other goes, with what a group's
   or an event's administrators were told of the one it blocked.

The rule is the same one every post is read by, `permissions.CanRead`, asked
by id through the graph's `FilterVisible`. A test keeps the two in step:
`graph-grpc-service/data/repositories/visibility_agreement_integration_test.go`.

## Who is never told

- **Whoever did it**, of their own doing.
- **A profile on another server,** of a like, a repost, a reply, a quote, a
  mention or a follow: its own server is told, by federation.
- **Of a draft.** Nothing is said of a post until it is published.

## When what it tells of goes

A notification that leads nowhere is noise, so it goes with what it tells
of, for everybody who was told, whoever's hand removed it:

| What goes | What goes with it |
| --- | --- |
| A post or a reply | Every notification naming it as object, `comment_id` or target: its likes, reposts, quotes, mentions, and the replies under it |
| A repost, taken back | Its `repost_add`, found by the repost's id in `comment_id` |
| A like, taken back | Its `voteup` |
| A name, taken out of a post in an edit | Its `actor_mention` (actor-service says so, `edge.mention.*.deleted.v1`) |
| A follow | Its `actor_follow`, and the subscription unless they administer the profile |
| An administrator | Their `admin_add`, and the subscription unless they follow the profile |
| A note from another server, taken back there | Everything about it |
| An account, purged | Everything to, by or about it (graph-grpc-service's purge) |
| A block | What the two were told of each other |

Taking one of these back tells nobody.

## Delivery

Events are acknowledged as they arrive and never delivered twice, so
nothing is written or sent twice; a handler that fails says so in the log,
and an event published while notification-service is down is not heard
([Deploying](deploying.md#running-more-than-one-copy-of-a-service)). Keep
notification-service at one copy: each copy hears every event.

## The API

| Route | What it does |
| --- | --- |
| `GET /notifications/?start=&limit=` | The viewer's notifications, newest first |
| `GET /notifications/unread-count` | How many they have not read |
| `PATCH /notifications/:id` | Marks one read |
| `DELETE /notifications/:id` | Deletes one |
| `POST /notifications/subscriptions` | Subscribes to a profile or a post |
| `DELETE /notifications/subscriptions/:target_id` | Ends that subscription |

Every route is the viewer's own: nobody reads, marks or deletes another's
notifications. The reference is [api/notification-service.json](api/notification-service.json).
