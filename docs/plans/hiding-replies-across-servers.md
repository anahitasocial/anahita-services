# Hiding a reply from others, across servers (item 44)

Raised by the user on 2026-10-12, on seeing "Hide" on a reply that had come
from Mastodon. Planned, not built. **What is wanted was said the same day: hidden
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
