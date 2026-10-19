# Permissions

[Documentation](README.md) › Permissions

Who can post, reply, like and add followers on a profile, and who can edit or
delete a reply. It covers the defaults an installation ships with, how to
change them, and what profile owners can change for themselves.

What each route answers when the viewer may not is in the [API reference](api/index.html).

## Contents

- [Two separate settings](#two-separate-settings)
- [Access levels](#access-levels)
- [A members-only installation](#a-members-only-installation)
- [Who is always allowed](#who-is-always-allowed)
- [Seeing posts](#seeing-posts)
  - [Choosing who can see a post](#choosing-who-can-see-a-post)
- [Posting](#posting)
- [Replies](#replies)
  - [Seeing a reply](#seeing-a-reply)
  - [Replying](#replying)
  - [Who the author lets reply](#who-the-author-lets-reply)
  - [Who looks after a thread](#who-looks-after-a-thread)
  - [Editing, deleting and hiding a reply](#editing-deleting-and-hiding-a-reply)
  - [Replying as a website](#replying-as-a-website)
  - [Liking a reply](#liking-a-reply)
- [Likes](#likes)
- [Following](#following)
- [Invitations](#invitations)
- [Events](#events)
- [Reposts](#reposts)
- [Mentions](#mentions)
- [Quotes](#quotes)
- [Saved posts](#saved-posts)
- [Pinned posts](#pinned-posts)
- [Downloading photos and videos](#downloading-photos-and-videos)
- [Custom feeds](#custom-feeds)
- [Suggestions](#suggestions)
- [Reporting](#reporting)
- [Account types](#account-types)
- [Removing accounts](#removing-accounts)
- [The defaults](#the-defaults)
- [What profile owners can change](#what-profile-owners-can-change)
- [Accepted values](#accepted-values)
- [Where the code is](#where-the-code-is)

## Two separate settings

Every profile, whether a person or a group, has two settings that are easy to
confuse:

| Setting | Question it answers | Where it is set |
| --- | --- | --- |
| **Access** | Who can *see* this profile and its posts | Settings › Privacy |
| **Permissions** | Who can *add* to it: posts, replies, likes, followers | Settings › Permissions |

Permissions never widen access. Anybody refused by the profile's access is
refused everything below, whatever the permissions say.

## Access levels

A permission is one of these levels. Each one is measured against the profile
the post is on, not against the person who wrote the post.

| Level | Admits | Shown as |
| --- | --- | --- |
| `registered` | Anybody signed in | Anyone signed in |
| `followers` | People who follow the profile | Followers |
| `leaders` | People the profile follows (people only) | Leaders |
| `mutuals` | People who follow the profile and are followed back (people only) | Mutuals |
| `admins` | The profile's administrators. For a person, that is the person | Admins / Only me |

A profile's administrators meet every level. A group follows nobody, so
`leaders` and `mutuals` are only offered for people.

## A members-only installation

Everything on this page assumes somebody who is not signed in can read what is
`public`. An installation can switch that off with `SITE_READ_ACCESS=registered`:
then they can read nothing, whatever each profile and post says, and `public`
means the same as `registered`. With `SITE_READ_ACCESS=preview` they can read
only the start of what is public. See
[Privacy](privacy.md#what-the-installation-allows), which also gathers the
audiences of posts and profiles in one place.

## Who is always allowed

These people are allowed whatever a profile has set:

- **Site administrators and super administrators**
- **The profile's owner**: the person, or for a group its administrators

The one exception is editing somebody else's reply, which nobody but its
author can do (see [Replies](#replies)).

## Seeing posts

A post has an access level of its own, as well as the one on the profile it is
on. Somebody sees a post when both admit them:

1. the **profile's** access admits them, and
2. the **post's** access admits them.

Both are measured against the profile the post is on. A followers-only post in
a group is for the group's followers, whoever wrote it.

Two people see a post without its own access being consulted:

- **The person whose profile it is on** sees everything on it.
- **The author** sees their own post for as long as they can see the profile
  it is on.

Nobody sees a post by, or on the profile of, somebody they have blocked or
been blocked by.

A block also takes out of sight, for **every** reader, what the blocked
person has left on what is the blocker's:

- their replies in a thread whose post the blocker owns or wrote, and what is
  under those replies; the thread's reply count leaves them out;
- their likes of the blocker's posts, in the list of who liked it and in the
  count;
- their posts on the blocker's profile.

Only there. Under somebody else's post, what the blocked person wrote stays
for other readers, and hiding it is for whoever that post belongs to. Nothing
is deleted: it is still theirs, and comes back if the block is lifted. A block
of somebody on another server is also told to that server (see
[federation.md](federation.md)).

A person's block does not reach the groups and events they administer. A group
or an event blocks somebody in its own right, done by one of its
administrators from the menu on what that person wrote in it, and lifted from
its settings (Privacy, Blocked). Its block hides what the blocked person left
in it, for every reader, as a person's does, and keeps them out of it. It is
told to no other server: groups are not on the fediverse.

| Request | Allowed to |
| --- | --- |
| `POST` and `DELETE /socialgraph/<person>/blocks/<group>` | The group's administrators, and site administrators |
| `GET /socialgraph/<group>/blocks/` | The same |

Posts and replies in a group carry `authorized.block_from_owner`, true for the
group's administrators and for site administrators on what somebody else wrote
there (`permissions.CanBlockFromOwner`). In feeds and lists it is true for the
group's administrators only; a site administrator blocks from the post's own
page.

A post's access is always one of the levels above. There is no way to share a
post with named people only, and no private messaging: the private notes of
the PHP version were removed, not carried over.

The same rule applies on a profile, in the home feed, and under a hashtag or a
location. Search applies both access checks but not the two exceptions above,
so searching may not find your own restricted posts.

What depends on another post is judged by that post:

| Item | Seen by |
| --- | --- |
| A reply | Whoever can see the post at the top of its thread |
| A repost | Whoever can see the original post, and the reposter's profile |

So following somebody who reposts a followers-only post does not show it to
you. You see it only if you could see the original.

### Choosing who can see a post

The author chooses a post's audience when writing it, and whoever may edit the
post can change it afterwards. It is sent as `access` when the post is created
(`POST /notes/:owner_id/` and the same for articles, topics and photos; for a
video, when its upload begins, `POST /videos/:owner_id/uploads`) and changed
with `PATCH /…/:id/access`. Left out, a new post is `public`.

Which audiences a post may have depends on where it is:

| Where the post is | Audiences |
| --- | --- |
| Your own profile | `public`, `registered`, `followers`, `leaders`, `mutuals`, `myself` |
| Somebody else's profile | `public`, `registered`, `followers`, `leaders`, `mutuals` |
| A group, as one of its followers | `public`, `registered`, `followers` |
| A group, as one of its administrators, or as a site administrator | `public`, `registered`, `followers`, `admins` |

Anything else is refused with 422 `access_not_allowed`, and the answer lists
the audiences that are allowed. Two of the gaps are deliberate:

- **No "only me" on somebody else's profile.** It would be a post only its
  author and that person can see, which is a private message. This platform
  does not have those.
- **No "administrators only" from a follower who is not one.** That would be the
  same thing, addressed to a group's administrators.

As with every level, the audience is measured against the profile the post is
on: `followers` are that profile's followers, not the author's. A post is
never seen more widely than its profile, so on a followers-only group a
`public` post is still only seen by its followers. The web app shows those wider
choices disabled, and does not offer `leaders` when writing a post.

An installation can take `public` out of all four rows, for everybody, with
`MIN_CONTENT_ACCESS=registered`; see
[Privacy](privacy.md#new-content-min_content_access).
A post that is already public may keep that audience.

Every person and group in a response carries `authorized.audiences`: the
audiences a post the viewer writes there may have, from this same rule. The
web app's composer offers those and nothing else.

A post carries `authorized.audiences` too, for a viewer who may edit it: the
audiences it may be changed to. They can differ from the profile's list,
because "only me" depends on who wrote the post, not on who is changing it.

## Posting

Who can create a note, article, topic, photo or video on somebody's profile.

- The owner and the profile's administrators always can.
- Everybody else has to be able to see the profile and meet the level set for
  that kind of post.
- `registered` is not accepted for posts. Nobody can open their profile to
  posts from anybody who happens to be signed in.

Default: `followers`.

## Replies

A reply is a post that answers a post or another reply. Most are notes; a
reply with pictures is a photo post, and one with a GIF or a video a video
post (since 2026-10-14: "nodes are nodes"). It can be liked, reposted, tagged
and searched like any post of its kind, and replies can be answered in turn,
to any depth. Whatever its kind, every rule below applies to it the same way:
text-service, photo-service and video-service each judge a post that is a
reply by its thread, with the thread read by one shared loader
(`anahita-libs/cmd/api/handlers/threads.go`), and each refuses a change to a
reply's audience, reply rule or quote rule with `422
reply_follows_its_thread`.

Every kind of post is answered with replies: notes, photos, articles and
topics. What used to be comments were turned into replies on 2026-10-05, each
a reply made directly to the post it was under, and the service that served
them is gone.

**Everything about a reply is decided by the post at the top of its thread,**
called the root here. A reply is owned by whoever wrote it and has no audience
of its own, so none of the rules for an ordinary post apply to it. Its owner
and its access are not consulted for anything.

### Seeing a reply

A reply is seen by exactly the people who can see the root. That holds in
every place a note can turn up: the thread, a profile's list, a feed, search,
and the page of a hashtag or a location.

So a reply to a post in a private group is seen only by the group's
followers, though the person who wrote it has a public profile. And when a
post is later made private, everything said under it goes with it.

Replies are left out of feeds and out of a profile's list of notes. They are
read in their thread.

### Replying

Everybody who is not always allowed has to be able to **see the root** first.
After that, it depends on the access of the profile the root is on:

| Profile access | Who can reply |
| --- | --- |
| `public` or `registered` | The profile's setting for replies: `registered` or `followers` |
| `followers` | Followers only, whatever the setting says |
| `leaders`, `mutuals`, `admins`, `myself` | Anybody who can see the profile |

The setting only matters on a public or registered profile, because a narrower
profile already limits who can see its posts. A followers-only profile is
spelled out as followers only, even though people the profile follows can also
read it. Default: `registered`.

The setting is stored under the name `comment`, from when replies were
comments, so what profiles had saved still applies. The scope a token needs is
`media:write`: a reply is a post.

Then the post's own setting, which its author chooses (below).

A reply cannot be given an audience, and neither its switch for comments nor
who may reply to it can be changed. All three follow the root.

### Who the author lets reply

A post carries who may reply to it, in `nodes.reply_access`, returned as
`reply_access`:

| Value | Who may reply |
| --- | --- |
| `anyone` | Whoever the rule above lets. What a post has when its author has not said. |
| `nobody` | Nobody. |
| `followers` | People who follow the post's author. |
| `following` | People the post's author follows. |
| `mentioned` | People the post mentions by `@name`. A name with a host after it, `@name@example.net`, is somebody on another server and nobody here, whoever here shares the name. |

The last three can be combined, joined by commas (`followers,mentioned`), and
somebody in any one of them may reply.

- **It narrows and never widens.** The profile's rule is applied first. In a
  group that lets only its followers comment, being mentioned in a post does
  not let somebody outside the group reply.
- **Measured against the author, not the profile.** They are the same for a
  post on its author's own profile. For a post in a group, `followers` are
  the people who follow whoever wrote it, not the group's followers.
- **The people who look after the thread (below) reply whatever is set.**
  Closing a post is how they stop other people, not themselves.
- **Set when the post is made** (`reply_access` beside `access`) **or
  afterwards** with `PATCH /{articles,notes,topics,photos}/:id/reply-access`,
  by whoever may edit the post. A value that is not one is refused with 400
  `reply_access_invalid`; a reply answers 422 `reply_follows_its_thread`.
- **The older switch**, `comment_status`, is kept in step: off is `nobody`,
  and turning it on makes `nobody` `anyone`. A post closed before this
  setting existed has no value stored and still reads as `nobody`.
- **A post from another server** (the fediverse or Bluesky) can be replied
  to here by anybody who can see it, and its `reply_access` reads `anyone`.
  Who its author lets reply is their server's to hold to; nothing of it is
  kept here.

The rule is `permissions.CanReply`; the values are in
`anahita-libs/constants/reply_access.go`. The thread says the answer for the
viewer (`can_reply`) and the setting (`reply_access`), so the app can say why
not.

Not built: letting in a list of people (there are no lists). Who may quote a
post is a setting of its own; see [Quotes](#quotes).

### Who looks after a thread

Three kinds of people can deal with what is said under a post:

- **the author of the root**, wherever it is posted;
- **the profile the root is on**: the person, or a group's or a website's
  administrators;
- **site administrators.**

The first is there for somebody who posts in a group they do not administer.
The replies are under their post, and they can act on them without being one
of the group's administrators.

### Editing, deleting and hiding a reply

| | Who |
| --- | --- |
| **Edit** | Its author only, and only while they can still see the root. Nobody who looks after the thread: an edited reply still carries its author's name. A reply that is a website's, by whoever replies as it |
| **Delete** | Its author, and whoever looks after the thread. The author can delete it even after losing sight of the root |
| **Hide**, and show again | Whoever looks after the thread. Not its author, who can delete it |

**Deleting** removes a reply for good, and is asked of the service of its
kind (`DELETE /notes/:id`, `/photos/:id`, `/videos/:id`), so that a photo or
video reply's files go with it. Its words are changed at `PATCH /notes/:id`
whatever its kind. If it has been answered, an empty
placeholder is left where it was, with no text and no name, so the replies
under it still hang from something. Deleting a post removes its whole thread.

**Hiding** takes a reply out of the thread for everybody, with everything
said under it. It is not destroyed and its author is not told. Whoever looks
after the thread still sees it, marked as hidden, and can show it again. A
hidden reply is not counted in the number of replies a post shows.

Hiding exists as well as deleting because, once this installation federates, a
reply that came from another server can be hidden here and cannot be deleted
there.

### Replying as a website

Under a website's own post, a reply in words by one of its administrators is
the website's, its owner and author, so the site answers its readers as
itself; which administrator wrote it is kept for those who handle reports
(`written_by`). Anywhere else a reply is its writer's. Who may connect a
website is `WEBSITES_FROM`, administrators unless set. See [Websites](websites.md).

### Liking a reply

Under the rule for liking the root: see it, and meet the like permission of
the profile the root is on.

## Likes

Who can like a post or a reply on a profile. The viewer has to be able to
read the post, then meet the level set for likes.

Default: `followers`.

## Following

The rules are in `anahita-libs/cmd/api/permissions/follow.go`, and
socialgraph-service asks them.

| Action | Who |
| --- | --- |
| Follow a profile | Yourself, a profile you can see, and that has not blocked you |
| Unfollow | Always yours to do, whether or not you can still see the profile |
| Remove a follower | Whoever looks after the profile: the person, a group's administrators, a site administrator |
| Ask to follow | Yourself, of a profile that lets people ask and has not blocked you, when you do not follow it already |
| Accept a request | Whoever looks after the profile that was asked |
| Ignore a request, or take it back | Whoever looks after the profile, or whoever asked |

Nobody follows, or asks to follow, on somebody else's behalf. The one
exception is the group setting below.

Asking does not need you to be able to see the profile. A profile you can see,
you can follow outright; the ones worth asking are the ones you cannot.
"Others can request to follow" (`allow_follow_request`) is the profile saying
it may be asked, and it can be turned off again.

A request is an edge of its own, `edge.actor.follow-request.v1`, and is not
counted among followers. Accepting removes it and makes the follow. Following
directly, where that is allowed, removes a request that was waiting.

Whoever looks after the profile is told of a request (`actor_follow_request`),
and that notification goes when the request is answered or taken back.
Whoever asked is told when it is accepted (`actor_follow_request_accept`).

### What a stranger is shown of a private profile

A profile somebody may not see is refused whole (403), with one exception. A
profile that lets people ask to follow it, or that the viewer has been invited
to follow, shows **its name, alias, picture
and verified mark, and nothing else** to somebody signed in whom it has not
blocked. Without that there would be nothing to ask from.

- **Opening the profile** answers 200 with that much, marked
  `restricted: true`, and `is_follow_requested_by_viewer`.
- **The lists of people and of groups** include such profiles for somebody
  signed in, each marked the same way. Not the featured list, and not any
  other listing: search, followers, mentions and feeds are unchanged.
- **Visitors** are shown nothing, as before. So is anybody when the profile
  is disabled, archived, on its way out, or does not let people ask.

The rule is `permissions.CanSeeLimitedProfile`. In a list the cutting down is
done inside the graph (`restrictUnseen`), before the actor leaves it, and the
response is a `LimitedActor`, a written-out list of fields. It is not the
ordinary item with fields blanked.

## Invitations

Nobody is made a follower of a group without saying yes. They are invited,
told, and choose. The rules are in
`anahita-libs/cmd/api/permissions/invite.go`, and what the data allows of
each person is in the graph (`data/repositories/invites.go`).

**Who can invite** is the group's setting, `invite` under socialgraph-service:
`admins` or `followers`. Default, and what a group that never set it gets:
`admins`. Site administrators always can. Only to a group the inviter can see
that is in use. Nobody is invited to follow a person.

**Who can be invited:** a person who follows the inviter. The answer for
each person named is one of:

| Answer | Meaning |
| --- | --- |
| `invited` | An invitation was made |
| `already_follower` | They follow the group |
| `not_follower` | They do not follow the inviter |
| `blocked` | Somebody among the person, the inviter and the group has blocked another |
| `pending` | They are invited already and have not answered |
| `cooldown` | They declined within the last 90 days |
| `requested` | They have asked to follow and are waiting; accept that instead |
| `unavailable` | Not a person, not there, or the inviter themselves |
| `full` | 500 invitations to the group are waiting |

Limits: 50 people in one request (400), and 200 invitations made in the last
day that are still waiting (429).

**An invitation** waits 30 days, then lapses. While it waits, the person
invited:

- is shown the group's name and picture even where they could not see it
  (see *What a stranger is shown of a private profile*);
- becomes a follower on accepting, whatever the group's access says. The
  invitation is the group's consent.

Declining keeps the invitation, marked refused, for 90 days, which is what
stops the next one. Whoever invited is not told of a refusal.

| Action | Who |
| --- | --- |
| Accept or decline | The person invited, and nobody for them |
| See who is still to answer | Whoever looks after the group |
| Take an invitation back | Whoever made it, or whoever looks after the group |

**Adding a follower without asking** is for site administrators alone
(`CanAddOthersAsFollowers`), for moderation and for moving data. It was the
group's `follower` setting, which is still stored and is no longer read.

## Events

An event is an actor like a group, with a time and a place: followed, posted
on, looked after by administrators. Everything said of a group here is true
of an event: its access, its permissions, who can invite to it. What only an
event has is below. The rules are in
`anahita-libs/cmd/api/permissions/event.go`.

Going to an event is following it. The answer, **going** or **maybe**, is on
the follow. Following an event the ordinary way, with nothing said, counts as
maybe.

| Action | Who |
| --- | --- |
| Make an event of your own | Anybody signed in, unless the installation sets `EVENTS_FROM` higher. They look after it, and are going |
| Make an event for a group | That group's administrators, and nobody else, whatever `EVENTS_FROM` says. The group's administrators become the event's |
| Answer going or maybe | Somebody who can see the event and whom it has not blocked, or somebody invited to it |
| Take an answer back | Always yours to do |
| See where to join online, and the address it is held at | Whoever is going, and whoever looks after the event. Not maybe, not invited |
| Change it, call it off | Whoever looks after it |

- **Places.** An event can limit how many are going. Going is refused when
  every place is taken (409 `event_full`); maybe is not counted. The check and
  the answer are made under a lock on the event, so the last place is given
  once however many ask at the same moment.
- **Over or called off.** No answers are taken (410 `event_closed`), and
  nobody can be invited. Leaving still works. An event that is called off is
  kept, saying so.
- **Invitations.** Accepting one is going, or maybe when every place is
  taken. An event with a limit on places may have twice that many invitations
  waiting, and never fewer than 50; with no limit, 500 as for a group.
- **For the followers of the group hosting it.** An event hosted by a group
  can be opened to everybody who follows that group
  (`events.open_to_host_followers`). They are then treated as invited, all at
  once: shown the event's name, time and description where they could not
  otherwise see it, in the group's list of events and on the event's own
  page, and able to answer going or maybe. It is not an access level. What is
  posted on the event stays for the people who answered, and a follower of
  the group sees that only once they have. The relation says so as
  `via_host`.
- **Where it is held** is an address kept on the event's own node
  (`geo_address` and the fields beside it), not a place on the site's map.
  Places are public and shared, and an event at somebody's home should not
  put their home among them. Nothing searches for an event by it. A public
  venue can be tagged on the event like a place on any post.
- **Its place on a map** is kept only when whoever saves the event asks for
  a map (`show_map`), since finding it sends the address to a geocoder. The
  point is sent with the address and to nobody who is not sent the address.
- **Its address on the site** is set when it is made and does not change when it is
  renamed, since invitations and calendars link to it.

## Reposts

Reposts have no setting under Permissions and cannot be restricted per
profile.

A repost is shown only to people who can see the original post. See
[Seeing posts](#seeing-posts).

Only people repost, as only people reply. What a person reposts is in the
list of reposts on their profile, and not among its posts. They can have
reposts among the posts as well ("Also show reposts in the Posts tab", under
Settings › Privacy). It changes that one list and nothing else: the list of
reposts always has them, and followers are sent them in their own feeds, which
is what a repost is for. It does not change who can see a repost.

## Mentions

A post mentions a profile when it names it: by its `@handle`, or by its
address on a line of its own, which shows its card. People, groups and
events can be mentioned, here or on another server.

- **actor-service keeps the mentions,** as `edge.tag.mention.v1` from the
  profile to the post, each time a post is added or edited: it adds what is
  newly named and takes away what is no longer. A person's own description
  can name people too.
- **A person chooses who may mention them,** under Settings › Permissions,
  "Who can mention you?": the `mention` permission of `socialgraph-service`,
  one of `registered` (anybody signed in, the default), `followers`,
  `leaders` (people they follow), `mutuals` or `admins` (nobody but
  themselves). Somebody they block may not mention them either. A mention
  that isn't allowed is no edge at all, and nobody is told; the `@handle`
  stays in the text as written. Groups and events may always be mentioned.
  The rule is `permissions.CanMention`.
- **The person is told,** in their notifications, "mentioned you in a post"
  or "in a reply" (`actor_mention`, `actor_mention_reply`), from the event
  `edge.mention.actor-service.added.v1`. Only a person here, never for a
  draft, never for naming themselves, and only when they may see the post,
  so being named in a private group's post tells nobody outside it that the
  post exists. A group or an event mentioned is told nothing.
- **A mention is not a citation.** A post naming a post cites it; naming a
  profile mentions it (the user, 2026-10-17).

## Quotes

A quote is a note that carries another post under its own words. It sits
between a repost, which has no words of the reposter's, and a reply, which
stays in the other person's thread. The note is an ordinary note: its own
audience, likes and replies, on its author's profile.

Quoting puts somebody's post in front of another audience under somebody
else's words, so **the person quoted keeps the say**.

### Who can quote a post

Somebody signed in, who can see the post, when:

- **it is a post at the top level and still there.** Not a reply, which
  belongs to its thread, and not a repost;
- **it is public**, on a profile that is public or for members. A note quoting
  a post with a narrower audience would be one most of its readers could not
  see the point of;
- **its author has not blocked them**;
- **the post's quote policy lets them**:

| Policy | Who can quote |
| --- | --- |
| `anyone` | Anybody who can see the post |
| `followers` | People who follow the post's author |
| `nobody` | Nobody but its author |

A post has its own policy when its author sets one, in the post's interaction
settings, beside who can reply. A post that has none follows its author's own
setting ("Who can quote my posts", under Settings › Permissions), and a person who
has never chosen lets anyone. Both are kept in `nodes.quote_policy`: on a post
it is that post's, on a person it is their default.

Somebody can always quote their own post, whatever its policy. Only people
quote, and only a note can quote.

A refusal answers 403 `quote_not_allowed` with a `reason`: `policy`,
`not_public`, `blocked` or `unavailable`. A post the viewer cannot see answers
`unavailable`, the same as one that is not there.

### What a note shows of the post it quotes

**A quote never shows anybody a post they could not otherwise see.** The
quoted post is looked up for each reader, as that reader, every time the note
is sent. The note carries `quote`:

- the post, with its author, the profile it is on and its date, when the
  reader may see it;
- `{"state": "unavailable"}` when they may not, or it has been removed. So a
  post later made private, or whose author blocks the reader, disappears from
  the quote for that reader;
- `{"state": "detached"}` when its author took it out (below).

One level only: a quoted note that itself quotes something is shown without
it.

### Taking a post out of a quote

The author of a quoted post can remove it from any note that quotes it
(`DELETE /notes/<the quoting note>/quote`), and so can a site administrator.
The note stays, saying "Post removed by its author". It cannot be undone, and
the note cannot be made to quote the post again.

Changing a post's policy to `nobody` stops new quotes. Notes that already
quote it stay until the author takes the post out of each.

### Counting and telling

A post counts the notes that still quote it (`quote_count`): not one that was
removed, and not one it was taken out of. Its author is told when it is
quoted (`quote_add`), unless they quoted it themselves.

The rules are `permissions.CanQuote` and `permissions.CanDetachQuote`; the
lookup for each reader is `handlers.SetQuotes`, which asks the access gate
(`FilterVisible`) before it reads anything.

## Saved posts

Anybody signed in can save a post they can see, to find it again, and take
it out of their list whenever they like. What somebody saved is theirs alone:
nobody else can see the list, and nobody, the author of a post included, can
find out who saved it or whether anybody did. A saved post stays in the list
only while its saver can still see it.

## Pinned posts

A post can be pinned to the profile it is on. It then comes first in that
profile's posts, and first in the profile's list of its kind (notes, photos
and so on), marked as pinned.

- **A profile has one pin.** Pinning another post moves the pin to it: the
  post that had it gives it up. One is also exactly what Bluesky has, so a
  pin here will be a pin there with nothing left over.
- **The pin is the profile's, not the post's.** It is set by whoever the
  profile belongs to: a person on their own profile, and a group's
  administrators in the group; and site administrators, who may change any
  post. The author of a post in a group they do not run can edit and remove
  their post, and cannot pin it.
- **Offered in feeds to whoever runs the profile.** Feeds and lists offer Pin
  to the profile and its administrators only; a site administrator pins from
  the post's own page (`permissions.RunsProfile`).
- **Only a post at the top level.** A reply is read in its thread.
- **It changes the order and nothing else.** A pinned post is seen by exactly
  the people who could see it anyway.

`PUT` and `DELETE /{articles,notes,topics,photos}/<id>/pin`. The rule is
`permissions.CanPin`, and a post says whether the viewer may in
`authorized.pin`.

## Downloading photos and videos

Whether what a post holds may be downloaded is its author's to say, on
each post, and is no until they say yes. The rule is
`permissions.CanDownload`, and `handlers.MayDownload` is what a download
address asks.

| Who | May download |
| --- | --- |
| Somebody who can see the post, when it allows download | Yes |
| The post's author | Always, whatever the post says |
| Somebody who can see the post, when it does not | No (`403`) |
| Somebody who cannot see the post | No, and told there is no such post (`404`) |
| A visitor on a site in `preview` mode | No |

Allowing download never widens who can see a post: it is asked only of
somebody who already can.

Whoever may edit the post may change the switch: its author, and the
admins of the group it is on. A profile's default for new posts is changed
by whoever may edit the profile. See [Photo posts](photos.md#downloading).

## Custom feeds

A custom feed is its maker's, and has an audience of its own. See
[feeds.md](feeds.md) for what a feed is made of.

- **Who may see a feed** is its `access`: `myself` unless its owner says
  otherwise, or `followers`, `registered` or `public`. It is judged as a post
  is, by its own access and its owner's, and a block either way hides it. One
  that may not be seen is answered as if it were not there.
- **What a reader sees in a feed** is decided for the reader, and never for
  the feed's owner. Whatever a feed selects goes through the gate every feed
  uses. A public feed built on a private group shows that group's posts to
  its followers alone. A feed never widens what anybody may see.
- **Whoever may see a feed** may read it, copy it, and keep it on their home
  page. Its owner, and site administrators, may change and delete it.
- **Making and changing one** needs somebody signed in, with the
  `media:write` scope. A person owns 10 at most.
- **No new public feed** on an installation that takes no new public content
  (`MIN_CONTENT_ACCESS`). One that was public already may stay so.

## Suggestions

Suggestions of who to follow are made to somebody signed in, about that
person, and to nobody else. See [recommendations.md](recommendations.md).

- **Nobody is suggested whom the viewer may not see.** What is found is put
  to the graph (`FilterVisible`) before any of it is sent, and so is what a
  reason names.
- **Nobody is suggested who asked not to be** (`discoverable`), nor anybody
  on either side of a block with the viewer, nor an account that is
  disabled, archived, deleted or unconfirmed.
- **What somebody chose, and whom they turned down, is theirs alone.** Whoever
  was turned down is not told.

## Reporting

Anybody signed in may report anything they can see, except themselves and
what they wrote. Something they cannot see cannot be reported, and answers as
if it did not exist.

Only administrators and super administrators may read what has been reported,
see who reported it, and decide a case. The person reported is not told. See
[Abuse reports](abuse-reports.md).

## Account types

A person's account type is **registered**, **administrator** or **super
administrator** (`person_type` in the code and the API). Everybody is registered when they join; the first super
administrator is made from `SUPER_ADMIN_EMAIL` (see
[Registration](registration.md)). After that:

| Rule | |
| --- | --- |
| Who may change a person's account type | An administrator may make somebody registered an administrator. Everything else, making a super administrator and taking a type away from anybody, is a super administrator's. Always after proving who they are again |
| Whose | Anybody else's, never their own |
| Of whom | A person here: not a group, not a profile from another server, not a deleted person |
| Recorded | In `node_deletion_logs`, as `made-registered`, `made-admin` or `made-super-admin`, with who did it |
| When it takes effect | At the person's next token, within fifteen minutes |

What sets a super administrator apart is the installation's configuration and
its system tasks, which is why an administrator may add administrators but not
make a super administrator or remove anybody's type (the user, 2026-10-18).

Nobody changing their own account type is what keeps a super administrator: the last
one can only be demoted by another, and there is no other. One who wants to
step down makes somebody else a super administrator first, who can then
change theirs. It is done from the **Account type** card in the
Administration section of a person's settings in the web app, or with
`PATCH /accounts/:id/person-type`. The rule is
`anahita-libs/constants/person_types.go` (`MayChangePersonType`).

## Removing accounts

Disabling, archiving, deleting and purging a person or a group each have their
own rule, set in code and not changeable per profile:

| Action | Who may |
| --- | --- |
| Disable or enable | Site administrators and super administrators |
| Archive | The person themselves, a group's administrators, or a super administrator |
| Delete, with 30 days to restore | The person themselves, a group's administrators, or a site administrator |
| Purge, with no way back | Super administrators only |
| See the list of accounts and their activity | Super administrators only |
| Appoint an administrator to any group | Site administrators and super administrators |
| Change a person's account type | Administrators make administrators of registered people; the rest is super administrators'; never their own: see [Account types](#account-types) |

What each one does is in [Account lifecycle](account-lifecycle.md).

## The defaults

The defaults live in `graph-grpc-defaults/actor_features/`:

| File | Applies to |
| --- | --- |
| `default.json` | Every profile, as the base |
| `person.json` | People, layered over `default.json` |
| `group.json` | Groups, layered over `default.json` |
| `event.json` | Events, layered over `default.json` |
| `website.json` | Websites, layered over `default.json` |

Each service lists its `add_permissions` as an entity and a level:

```json
{
    "service": "text-service",
    "add_permissions": [
        { "entity": "note", "access": "followers" },
        { "entity": "article", "access": "followers" }
    ],
    "include_common_permissions": ["content_creator"]
}
```

Replies and likes are shared by every service that has posts, so they are set
once in `default.json` under `common_permissions` and pulled in with
`include_common_permissions`:

```json
"common_permissions": {
    "content_creator": [
        { "entity": "comment", "access": "registered" },
        { "entity": "like", "access": "followers" }
    ]
}
```

### Changing them

Edit the files and the change takes effect on the next request. graph-grpc
reads the directory named by `ACTOR_FEATURES_PATH` every time it reads a
profile, so no restart is needed.

`docker-compose.yml` mounts `graph-grpc-defaults/actor_features` at
`/defaults/actor_features`, which is where `ACTOR_FEATURES_PATH` points by
default. The graph-grpc image also carries a copy made at build time, at
`/app/defaults/actor_features`. An installation that runs the image without
the mount has to set `ACTOR_FEATURES_PATH` to that path, and rebuild the image
to pick up a change.

Only use the values listed under [Accepted values](#accepted-values). A value
the permission checks do not recognise refuses everybody except the owner, the
profile's administrators and site administrators, and nothing reports it.

## What profile owners can change

A profile's owner and its administrators can change the levels for their own
profile under **Settings › Permissions**. Site administrators can too, for any
profile.

- A profile that has never saved this tab follows the defaults, including any
  later change to them.
- Saving stores the profile's **whole** feature list, not only the values that
  changed. From then on, later changes to the defaults do not reach that
  profile.
- **Restore defaults** clears the profile's saved settings, and the defaults
  apply again.

On a profile whose access is narrower than `registered`, the row for replies
explains who can reply instead of offering a choice. See
[Replying](#replying).

## Accepted values

The server refuses to save anything else. These are the values the permission
checks enforce.

| Entity | People | Groups |
| --- | --- | --- |
| Posts (`note`, `article`, `topic`, `photo`) | `followers`, `leaders`, `mutuals`, `admins` | `followers`, `admins` |
| `comment` | `registered`, `followers` | `registered`, `followers` |
| `like` | `registered`, `followers`, `leaders`, `mutuals`, `admins` | `registered`, `followers`, `admins` |
| `follower` | `admins` (not enforced) | `followers`, `admins` |

## Where the code is

| What | Where |
| --- | --- |
| What each level admits | `anahita-libs/cmd/api/permissions/add_access.go` (`AddAccessAllows`), tested in `add_access_test.go` |
| Seeing a profile and a post | `anahita-libs/cmd/api/permissions/actor.go` and `medium.go` (`CanRead`); in lists and feeds, `graph-grpc-service/data/repositories/access.go`, `access_actor.go`, `access_medium.go` |
| A post reached by its id | `anahita-libs/cmd/api/permissions/visibility.go` (`CanReadByID`, `CanReadEach`), `anahita-libs/data/services/graph_node_grpc.go` (`FilterVisible`) |
| Choosing who can see a post | `anahita-libs/cmd/api/permissions/audience.go` (`MediumAudiences`), tested in `audience_test.go` |
| Posting and likes | `anahita-libs/cmd/api/permissions/capabilities.go` (`CanPostTo`, `CanLike`) |
| A group's or an event's block | `anahita-libs/cmd/api/permissions/block_from_owner.go` (`CanBlockFromOwner`) |
| Replies | `anahita-libs/cmd/api/permissions/reply.go` (`CanReply`, `CanEditReply`, `CanDeleteReply`, `CanHideReply`, `CanLikeReply`); the thread, `anahita-libs/cmd/api/handlers/threads.go` |
| Who the author lets reply | `anahita-libs/constants/reply_access.go`; table `nodes`, column `reply_access` |
| Following, and what a stranger is shown | `anahita-libs/cmd/api/permissions/follow.go`, `limited_profile.go` (`CanSeeLimitedProfile`); `graph-grpc-service/cmd/server/handlers/actor_restricted.go` (`restrictUnseen`) |
| Invitations | `anahita-libs/cmd/api/permissions/invite.go` (`CanInvite`), `graph-grpc-service/data/repositories/invites.go` |
| Events | `anahita-libs/cmd/api/permissions/event.go` (`CanRsvp`, `CanSeeOnlineURL`); setting `EVENTS_FROM` |
| Account types | `auth-service/cmd/api/handlers/person_type.go` (`EditPersonType`), `graph-grpc-service/cmd/server/handlers/actor_person_type.go` (`SetPersonType`, which refuses one's own), `graph-grpc-service/data/repositories/nodes_person_type.go` |
| Adding a location | `location-service/cmd/api/permissions/locations.go` (`CanAdd`); setting `LOCATIONS_FROM` |
| Mentions | `anahita-libs/cmd/api/permissions/mention.go` (`CanMention`) |
| Quotes | `anahita-libs/cmd/api/permissions/quote.go` (`CanQuote`, `CanDetachQuote`), `anahita-libs/constants/quote_policy.go`, `anahita-libs/cmd/api/handlers/quotes.go` (`SetQuotes`) |
| Pinned posts | `anahita-libs/cmd/api/permissions/pin.go` (`CanPin`, `RunsProfile`) |
| Downloading | `anahita-libs/cmd/api/permissions/download.go` (`CanDownload`), `anahita-libs/cmd/api/handlers/download.go` (`MayDownload`) |
| The defaults | `graph-grpc-defaults/actor_features/`, read by `graph-grpc-service/data/actor_features/`; setting `ACTOR_FEATURES_PATH` |
| Accepted values | `anahita-libs/validators/request.go` (`validatePersonAddPermissionAccess`, `validateActorAddPermissionAccess`) |
