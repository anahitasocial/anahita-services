# Blocking a server yourself (roadmap row 152)

Asked for by the user on 2026-11-15, from FediDB's list of features: "A
person should be able to block a whole server from their own profile and
feed; right now only the super admins can block a whole server from the
site settings." Today a server is refused for everybody, by a super
administrator (`federation_domain_blocks`, [Federation](../federation.md)),
and one person can block or mute one profile at a time (row 133).

## Contents

1. [What a block of a server does](#what-a-block-of-a-server-does)
2. [Where it is made](#where-it-is-made)
3. [Storage](#storage)
4. [Out of scope](#out-of-scope)
5. [Documentation](#documentation)
6. [Phases](#phases)

## What a block of a server does

For the person who blocks it, and nobody else:

1. **Out of sight:** posts, replies, reposts and quotes from that server
   leave their feeds, custom feeds, search, hashtag and location pages,
   and the replies under their posts, as a mute's do. Told by the post's
   `nodes.origin_domain`, and its subdomains.
2. **No notifications** from its people.
3. **Follows end, both ways,** with everybody on it, and new ones are
   refused: a follow from there is answered with `Reject`. Nothing is sent
   to the server to say it was blocked, as Mastodon does; its people are
   not told.
4. **Undone** from the same list: follows are not made again.
5. **Never names a site's own domain,** and a server the site refuses
   already needs nothing.

## Where it is made

1. From any profile on another server, in its ⋮ menu: **Block server**,
   with a line saying what it does, then Cancel and Block.
2. Settings › Blocked: a list of servers beside the list of profiles, each
   with Unblock, and a box to add one by its domain.

## Storage

A table `person_domain_blocks` (`person_id`, `domain`, `created_at`), its
key the two; checked where mutes are, in the same queries, so that every
list a mute reaches a server block reaches too. Its rows go with a purge,
and in a person's download, as Mastodon's `domain_blocks.csv`, which row
124 can then import.

## Out of scope

1. Blocking a server on Bluesky: a Bluesky account's server is not a
   community. A person blocks Bluesky accounts one by one.
2. Sharing one's list with others.

## Documentation

1. anahita-services `docs/federation.md`, `docs/permissions.md` (Mutes,
   Blocks), `docs/data-export.md`, the API reference.
2. anahita-web `docs/architecture.md`.
3. Both changelogs.

## Phases

| Phase | What | Status |
| --- | --- | --- |
| 1 | The table, the routes, and every query a mute reaches; follows ended and refused | ⬜ |
| 2 | Block server in a profile's menu; servers in Settings › Blocked | ⬜ |
| 3 | The download; the docs; tried from a Mastodon account at both sizes; changelogs | ⬜ |
