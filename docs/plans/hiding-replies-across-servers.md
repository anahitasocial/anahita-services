# Hiding a reply from others, across servers (item 44)

Raised by the user on 2026-10-12, on seeing "Hide" on a reply that had come
from Mastodon. Begun 2026-10-15: see *Progress* at the end. **What is wanted was said the same day: hidden
everywhere, preferably,** wherever the post is read, and not only on this
installation.

## What there is

Whoever looks after a thread here can hide a reply under their post
([notes-as-replies.md](done/notes-as-replies.md)): it is no longer shown in
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
  and anything else of theirs there. And the same **on a profile the
  blocker administers**, a group or an event, as the user said it.
  - **Only there.** Said by the user next: "Unless the post belongs to
    another profile, in that case when I block someone the reply remains.
    It's up to the post owner and author to hide replies." A block reaches
    the blocker's own posts and profiles and no further: under somebody
    else's post, what the blocked person said stays for other readers, and
    hiding it is for whoever that post belongs to. The blocker does not see
    it themselves, as with anything from somebody they have blocked.
  - Hidden, not deleted: what they wrote is theirs, and comes back if the
    block is lifted.
  - To settle when it is built: whether a person's own block reaches the
    groups they administer, or whether that is the group blocking somebody,
    done by one of its administrators. The second is how blocking by a
    group would be told to other servers. (An open point of the plan's,
    not something the user has said.)
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
  With AT Protocol ([federation.md](federation.md), row 26) hiding a reply
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
2. **A block hides, for every reader: built, NOT committed** (uncommitted in
   graph-grpc-service; `git status` shows it):
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
   - Tests: `access_test.go` counts the new rule apart; new
     `block_reach_integration_test.go` passes against MySQL.
   - **To settle before committing:** with ACCESS_TEST_DSN, the whole
     repositories package has one failure,
     `TestCountsAreOfWhatIsThereAndNotHidden` (replies_integration_test.go).
     Its fixture has no block, so look at its message first: likely the
     recount SQL or an argument count. Throwaway MySQL as in
     access_integration_test.go's header, named `anahita-acl-test`, removed
     afterwards with `docker rm -fv`.
   - Still open: whether a person's block also reaches the groups they
     administer (only their own posts and profile now).
3. **Bluesky: to do.** Hiding publishes no event today. Add one (graph
   `HideReply`), have atproto-service write the root post's threadgate
   (`app.bsky.feed.threadgate`, rkey = the post's, `hiddenReplies` built when
   the write is sent; `CreateRecord` is already putRecord), for a root that
   went to Bluesky and a reply that is there. Reading threadgates made in
   Bluesky's app needs the collection added to Jetstream's
   `wantedCollections`.
4. **Web: to do.** In the hide confirmation or the hidden badge, say that a
   reply from Mastodon or another ActivityPub server is hidden here and may
   still be seen there (`reply.uri` https, or the author's type).
5. Docs when done: `federation.md` (ActivityPub part written), `permissions.md`
   for what a block hides, the web `architecture.md` replies section.
