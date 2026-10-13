# Hiding a reply from others, across servers (item 44)

Raised by the user on 2026-10-12, on seeing "Hide" on a reply that had come
from Mastodon. Planned, not built, and **what is wanted is still to be
said**: this records what there is, and what could be meant.

## What there is

Whoever looks after a thread here can hide a reply under their post
([notes-as-replies.md](done/notes-as-replies.md)): it is no longer shown in
the thread to anybody but its writer and whoever hid it, who can show it
again. It is an edge from the post to the reply, not a mark on the reply,
because the reply is its writer's.

For a reply that came from another server this hides it **here**. Nothing
is said to any other server: on its own server, and on every other that has
it, it is still under the post.

## What "from others" could mean

1. **Others here.** What there is. Nothing to build.
2. **Others everywhere.** That the reply should stop being shown under the
   post wherever the post is read. This is the one that needs a plan.

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

## Sketch, for when it is wanted

- Hiding stays what it is here.
- With AT Protocol: it is written to, and read from, the thread's record.
- With ActivityPub: whatever the servers of the day honour, and where they
  honour nothing, the web app says that the reply is hidden on this site
  and may still be seen on others.

## Depends on

Replies across servers (row 25b). AT Protocol (row 26) for the half that
can be done properly.
