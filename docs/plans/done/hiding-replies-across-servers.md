# Hiding a reply from others, across servers (item 44)

Raised by the user on 2026-10-12, on seeing "Hide" on a reply that had come
from Mastodon. Begun and finished 2026-10-15: see *Progress* at the end. **What is wanted was said the same day: hidden
everywhere, preferably,** wherever the post is read, and not only on this
installation.

## What there is

Whoever looks after a thread here can hide a reply under their post
([notes-as-replies.md](notes-as-replies.md)): it is no longer shown in
the thread to anybody but its writer and whoever hid it, who can show it
again. It is an edge from the post to the reply, not a mark on the reply,
because the reply is its writer's.

For a reply that came from another server this hides it **here**. Nothing
is said to any other server: on its own server, and on every other that has
it, it is still under the post.

## What is wanted

That a reply hidden here stops being shown under the post wherever the post
is read, as far as other servers can be brought to it. Hiding it here alone
is what there is already.

## How it is used

The user's own habit, on Threads: hide a reply they do not like, then block
whoever wrote it. "Generally they are bots or rude people." So hiding is
the first of two steps of looking after one's own thread, and the second
is often wanted, and not always:

- **Hiding and blocking stay two separate actions.** Decided by the user
  the same day: "Sometimes I just want to hide a comment but not
  necessarily block the author." There is no "hide and block" in one step,
  and hiding never blocks.
- **Why a block hides, in the user's words:** on Threads "I have to first
  hide a reply on my post and then block the person who replied, otherwise
  others continue to see the reply, but I can't see the person and their
  reply." A block alone leaves the reply up for everybody but the one
  person who can no longer see it to take it down. Here one block does
  both, on what is the blocker's.
- **Blocking somebody hides what they have left, without asking.** Decided
  by the user the same day: "You don't even have to ask. When I block
  someone I don't want to see their replies either." So a block takes out
  of sight, for every reader, what the blocked person has put on the
  blocker's profile: their replies under the blocker's posts, their likes,
  and anything else of theirs there. (The user first said the same of
  profiles the blocker administers; see the decision on groups below.)
  - **Only there.** Said by the user next: "Unless the post belongs to
    another profile, in that case when I block someone the reply remains.
    It's up to the post owner and author to hide replies." A block reaches
    the blocker's own posts and profiles and no further: under somebody
    else's post, what the blocked person said stays for other readers, and
    hiding it is for whoever that post belongs to. The blocker does not see
    it themselves, as with anything from somebody they have blocked.
  - Hidden, not deleted: what they wrote is theirs, and comes back if the
    block is lifted.
  - **A person's block does not reach the groups they administer.**
    Decided by the user on 2026-10-16: it "will really complicate things".
    A group blocks somebody in its own right, done by one of its
    administrators; the services allow it already, and such a block hides
    what the blocked person left in the group as a person's does. The web
    app has no way to do it yet: item 55
    ([group-blocks.md](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/group-blocks.md)).
    Groups are not on ActivityPub for now (the user, the same day), so a
    group's block is told to no other server.
  - To look at first: what a block does today to what the blocked person
    already left, for readers other than the blocker.
- **A block is not told to the other server yet.** Here it stops the
  blocker seeing them and refuses what they send to the blocker's threads.
  Mastodon and what speaks to it also send a `Block`, so that the blocked
  person's server stops showing them the blocker's posts and stops them
  answering. That is a settled part of ActivityPub, honoured widely, which
  hiding is not: for a bot or somebody rude, a block other servers keep to
  does more than a hide they ignore. It belongs with this item.

## What other servers can be asked

- **Bluesky has it.** A post's author keeps a list of replies hidden under
  it, as a record everybody's app reads, and apps tuck those replies away.
  With AT Protocol ([federation.md](../federation.md), row 26) hiding a reply
  here can write that record, and a reply hidden there can be hidden here.
- **ActivityPub has no agreed way.** Mastodon has no "hide reply". The
  nearest things are a post's author *approving* replies before they count
  (GoToSocial's interaction policies, and the proposals Mastodon has begun
  to follow for quotes), and asking a server to `Reject` a reply. Neither
  is hiding, and neither is honoured widely. To be looked at again when
  this is built: what is true of it will have moved.

## Sketch

- Hiding stays what it is here.
- With AT Protocol: it is written to, and read from, the thread's record.
- With ActivityPub: whatever the servers of the day honour, and where they
  honour nothing, the web app says that the reply is hidden on this site
  and may still be seen on others.

## Depends on

Replies across servers (row 25b). AT Protocol (row 26) for the half that
can be done properly.

## Progress (2026-10-15, for whoever picks this up next)

Looked at first, as the plan asked: a block hid the blocked person's things
from the two people involved only. Every other reader still saw their
replies under the blocker's posts, their likes (and they were counted), and
their posts on the blocker's profile. And on ActivityPub a block ended no
follow, so the blocker's posts went on being delivered to the blocked
person's server, and no `Block` was sent.

1. **ActivityPub: done, committed (services `9fec5e6`).** A block ends every
   follow between the two both ways (`activitypub_follows`), sends `Block`
   (an id made from the two, `blockID`), and unblocking sends `Undo` of it.
   The sweep forgets follows left across a block (`Missed.FollowsBlocked`).
   Tests: `TestSomebodyHereBlocksSomebodyElsewhere`,
   `TestASweepForgetsAFollowAcrossABlock`. Incoming `Block` is still ignored.
2. **A block hides, for every reader: done, committed.** In
   graph-grpc-service:
   - `replies.go`: `HiddenIDs` also returns replies written by somebody the
     root's owner or writer blocks, so text-service drops them (and what is
     under them) for every reader; `recountThread` leaves them out;
     `TouchedByBlock` finds what to count again.
   - `socialgraph_block.go`: `recountForBlock` after a block is made or
     lifted (threads and like counts).
   - `graph.go`: `setBlockedLikersQuery` leaves blocked likers out of a
     post's likers; `like_rel.go` `UpdateLikeCount` leaves them out of the
     count.
   - `access_medium.go`: `notLeftByBlocked` keeps out, for everybody, a post
     somebody left on a profile whose owner blocks them (feeds use the same
     rule).
   - Tests: `access_test.go` counts the new rule apart;
     `block_reach_integration_test.go`; and
     `TestCountsAreOfWhatIsThereAndNotHidden`, whose fixture already had
     Alice blocking Dave, now expects Dave's reply out of the count and
     hides Bob's instead. The whole package passes with ACCESS_TEST_DSN.
   - Described in `docs/permissions.md`.
   - Still open: whether a person's block also reaches the groups they
     administer (only their own posts and profile now). And a blocked
     person's reply still shows on their own profile's Replies tab, as on
     Bluesky.
3. **Bluesky: done, committed.** graph `HideReply` says
   `edge.hidden-reply.medium.added|deleted.v1` on the bus (object the reply,
   target the root). atproto-service (`hidden.go`) queues a "gate" write
   for a root that went to Bluesky as a connected account (or came from
   it as their own); when written, it reads the root's threadgate
   (`GetRecord`), adds or takes out that one reply by whether it is hidden
   then, keeps the rest (`allow`, replies hidden in Bluesky's app), at most
   300, and puts it back. Deleting the root deletes its threadgate. Tests:
   `TestAHiddenReplyIsHiddenOnBluesky`,
   `TestAHiddenReplyUnderAPostNotOnBluesky`. Not tried against real
   Bluesky. Left for later: a reply hidden in Bluesky's own app hidden
   here (threadgate in Jetstream's `wantedCollections`, mapped to
   `HideReply` by the root's owner).
4. **Web: done, committed (web `dd9b22d`).** Asking to hide a reply from
   the fediverse (`https` address, or an ActivityPub author:
   `utils/federation.js#fromFediverse`) says it is hidden here only, naming
   its server.
5. Docs: `federation.md`, `permissions.md` and the web `architecture.md`
   written.
6. **Left for later:** reading threadgates made in Bluesky's app; an
   incoming ActivityPub `Block` (ignored). Blocking by a group is item 55.
   None of it was tried against real servers or in a browser.
