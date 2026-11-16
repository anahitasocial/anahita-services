# What is yours, and what you take part in (roadmap row 146, done 2026-11-16)

Asked for by the user on 2026-11-15, while planning channels (row 147):
each person chooses, for each profile they follow, whether its posts reach
them as notifications or only in their feed; and taking part in a post
should tell them of the people they follow there, not of everybody. Settled
the same day, after weighing a level for each kind of notification and a
"people I follow" switch against it: no new setting at all, one rule.

## Contents

1. [The rule](#the-rule)
2. [Who tells you what](#who-tells-you-what)
3. [A profile's posts](#a-profiles-posts)
4. [Channels](#channels)
5. [What the code does today](#what-the-code-does-today)
6. [What changes](#what-changes)
7. [Out of scope](#out-of-scope)
8. [Documentation](#documentation)
9. [Phases](#phases)

## The rule

**What is yours, or your channel's, tells you of everyone. What you take
part in tells you of the people you follow, and of whoever answers you.**

Your switches in Settings › Notifications still turn each kind off, and
Unfollow post ends a thread.

## Who tells you what

| The post | Who is told to you | Unless |
| --- | --- | --- |
| Yours, a post or a reply | Everyone who likes, reposts, quotes or answers it | You turn that kind off |
| A channel's you look after | Everyone, as if it were yours | You turn that kind off |
| Somebody else's that you replied to, reposted, or chose Follow post on | The people you follow who like, repost or reply to it; and whoever answers you | You choose Unfollow post, or turn the kind off |
| A post by a follower in a group or an event you look after | As somebody else's | — |
| A new post from a profile you follow | Told while its notifications are on, the default | You choose Turn off, on the notification or on the profile |

Liking a post subscribes you to nothing: people like a lot, and would be
told of a lot (the user, 2026-11-15).

## A profile's posts

1. **A new follow is notified of the profile's posts,** as now. On a site
   of a few thousand people, each following a few, being told when they
   post is wanted (the user, 2026-11-15).
2. **The Following button** (today "Unfollow", which unfollows at once)
   becomes **Following**, with a menu: **Notify me of posts**, on or off,
   then **Unfollow**. The same for a person, a group, an event and a
   channel.
3. **Each post notification** (`note_add`, `article_add`, `photo_add`,
   `topic_add`, `video_add`, `audio_add`) has **Turn off** in its menu,
   with the line beside it "Its new posts will be in your feed, without a
   notification." It turns that profile's notifications off, with Undo. A
   group of several notifications from one profile has it too.
4. Off is the subscription to the profile taken away while the follow
   stays, which the API allows already
   (`DELETE /notifications/subscriptions/:target_id`). An admin's own
   subscription is the same switch.

## Channels

A channel is the author of its posts and its replies, and no person is
told of what happens to them today. Each of its admins is told, as if the
post were theirs: likes, reposts, quotes and replies. Not the admin who
did it, and each one's switches are their own.

## What the code does today

From the check of the code on 2026-11-15:

1. A subscription is an edge, `edge.base.subscription.v1`, from a person
   to a profile or a post, a row in `edges` with an unused `meta`.
   Following a profile, or becoming one of its admins, makes one
   (`event_edge_follow.go`, `event_edge_admin.go`); Follow post makes one
   by hand. Replying and liking make none.
2. A new post tells the profile's subscribers (`event_node_medium.go`). A
   reply tells the author of the post at the top, the author of what it
   answers, and the post's subscribers (`event_node_reply.go`). A like
   tells the post's author.
3. `audio_add` is made, but it has no topic, so it cannot be turned off,
   and the web app shows it as "other".
4. Following somebody on another server makes no subscription, and
   following from another server makes none either.
5. The web app never takes a profile's subscription away, and its Follow
   button has no menu.

## What changes

1. **A post's subscription counts the people its subscriber follows.**
   graph-grpc-service's `BrowseSources` for a post returns, with each
   subscriber, whether they follow whoever acted; notification-service
   tells a post's subscriber of a like, a repost or a reply only then. The
   author of the post and of what a reply answers are told as now,
   whoever acted. Existing Follow post subscriptions follow the new rule.
2. **Replying subscribes to the post at the top,** as Follow post does,
   and so does **reposting**. Deleting the reply or the repost does not
   unsubscribe: Unfollow post does.
3. **Likes and reposts reach a post's subscribers** (new): today they are
   told to its author alone.
4. **A channel's admins are told** whatever its author would be.
5. **`audio_add`** goes under the `posts` topic, in the web app's kinds,
   and in every language base.
6. **Turn off** is an action the server offers on post notifications,
   `notify_off`, beside `answer_request` and `follow`; the web app shows it
   in the notification's menu, with its icon.
7. **Following** with its menu in the web app, reading
   `is_subscribed_by_viewer`, which the actor already carries.

## Out of scope

1. A level for each kind of notification (Everyone, People I follow,
   Off), and a single "only from people I follow" switch. Weighed on
   2026-11-15 and left out: nobody has asked to hear only some people on
   their own posts. Either can come later without undoing this.
2. A level between on and off for a profile's posts: taking part already
   subscribes.
3. Notifications for a follower's post in a group one looks after, beyond
   what there is now.

## Documentation

1. anahita-services `docs/notifications.md`: a section of its own, **Who
   tells you what**, for everybody: the rule in one sentence, then the
   chart above, then how to turn each thing off. The kinds table with
   `audio_add`; the subscriptions as they are; following across servers
   makes no subscription; Turn off among what can be done from one;
   answering an invitation, which the web app does without a server
   action.
2. anahita-services `docs/the-graph.md`: what a subscription to a post
   counts.
3. anahita-web `docs/architecture.md`: the Following menu, Turn off, and
   invitations in a notification.
4. `CHANGELOG.md` of both repositories.

## Phases

| Phase | What | Status |
| --- | --- | --- |
| 1 | `audio_add` everywhere; the docs fixed where they and the code disagree | ✅ 2026-11-16 |
| 2 | Replying and reposting subscribe; a post's subscribers told of likes, reposts and replies from the people they follow; a channel's admins (today a website's) told as its author | ✅ 2026-11-16 |
| 3 | Following with its menu; Turn off on post notifications, with Undo | ✅ 2026-11-16 |
| 4 | Who tells you what, in the docs; tried at both sizes; changelogs | ✅ 2026-11-16 |
