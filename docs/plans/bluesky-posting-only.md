# A second profile posting to a Bluesky account

anahita-web roadmap row 110. Asked for by the user on 2026-10-28, who wanted
the posts of their blog, a Website here (@rastinmehr), on the Bluesky
account their own profile (@rastin) is connected to.

## Why

A Bluesky account connected here is that profile
([One person, one profile](../federation.md#bluesky)): what is posted with it
in Bluesky's own app is kept here as the profile's, and whoever follows,
mentions or answers it there is taken to mean that profile. So a second
profile connecting the same account is refused today (`409 bluesky_taken`).
Bluesky gives an account as many app passwords as its owner likes, but it is
still one account, and the two profiles would both claim what it does.

What is wanted is narrower: a profile someone looks after (their blog, a
group they run) sending its posts to a Bluesky account that is already
somebody's here, as theirs, without being that account.

## What changes

**Posting only.** A profile may connect a Bluesky account that is already
connected here, when whoever connects it is the one who connected it first.
That second connection only sends:

| | The account's own profile (connected first) | A profile posting only |
| --- | --- | --- |
| Its posts sent to the account | Yes | Yes |
| Posts made in Bluesky's app kept as its own | Yes | No |
| Who a follow, a mention or a reply there means | This profile | Never this one |
| Bluesky's notifications read as it | Yes | No |
| Its follows here followed there | Yes | No |
| What was kept of the account before, removed when connected | Yes | No: done once, by the first |

**An answer to a post it sent** still arrives: Bluesky tells the account's own
connection, and the post it answers is known by what was sent
(`atproto_sent`), so the answer is put under the blog's post, as now.

**Only the same person.** The second connection is refused unless whoever
makes it is the person whose profile the account is, or is that profile
(`409 bluesky_taken` otherwise, as now): nobody can post to somebody else's
Bluesky account by knowing its app password.

**Each with its own app password.** Each connection keeps its own token, so
one can be removed on Bluesky without the other.

**Ending.** Removing the account's own connection ends every one posting
only to it: they would otherwise be the account here, with nobody reading
it. Removing one that posts only touches nothing else.

## How

- `atproto_accounts`: a column `posts_only` (0 or 1), and the unique key on
  `did` kept for the first connection only, through a generated column that
  holds the `did` where `posts_only` is 0 and NULL where it is 1.
- `WhoConnected(did)` and `Shadowed()` read only the first connection;
  `Working()`, what is read as, leaves out those posting only.
- `PUT /atproto/settings` (connecting): where the account is connected
  already by the viewer's own profile, the connection is made posting only
  rather than refused, and its answer says so (`posts_only: true`).
- Writing: unchanged. Each profile's posts are written with its own
  connection.

## In the web app

In the Bluesky card of a profile's settings, when the account given is
already this person's here: a line under the fields, "This Bluesky account
is @rastin here. This profile's posts will be sent to it too; nothing from
Bluesky comes back to it." Then Cancel and **Connect**. Once connected, the
card says "Posting to @handle" rather than "Connected as @handle".

## Tests

- A second profile of the same person connects as posting only; anybody
  else's is still refused.
- Its posts are written with its own token; a post made in Bluesky's app is
  kept for the first profile only.
- A reply on Bluesky to a post the second profile sent is put under it.
- Removing the first connection ends the second; removing the second leaves
  the first.

## Documentation

- `docs/federation.md`, under Bluesky: "One person, one profile" says what a
  profile posting only does and does not do.
- The API reference, from the code.
- anahita-web's Bluesky card.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-28 |
| Implementation | not begun |
