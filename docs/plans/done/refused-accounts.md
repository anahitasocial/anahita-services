# Refusing an account from another network

anahita-web roadmap row 116. Planned 2026-10-29, asked for by the user:
"we should be able to block an account on Fediverse or Bluesky site wide.
Right now we can only block a Fediverse server."

## Why

A whole server can be refused (`federation_domain_blocks`, Site settings →
Fediverse), but not one account on it, and not one account on Bluesky. A
spammer on a large server, or on Bluesky, is either let in or refused with
everybody they share it with. Each person can block them for themselves,
but nobody can do it for the site.

The Bluesky side reads the servers' list by handle (`refused(handle)`), so
`alice.bsky.social` written there refuses that account today, but only
until its owner changes the handle.

## Decided

| What | Decided (the user, 2026-10-29) |
| --- | --- |
| Who | **Administrators**, as for removing a post or acting on a report: it is moderating one account. Refusing a server stays with super administrators |
| What is here from them | **Removed** when they are refused: their profile, posts, replies, likes and follows, as when their server says they were deleted. Let back, they can be looked up again; what was removed does not come back |
| The word | **Refuse**, as for servers. A block is each person's own |
| Where the list is | **Site settings → Fediverse**, in two tabs: **Servers** (the list there is now) and **Accounts**. Administrators open Site settings for this tab alone, and see only Accounts in it (the user, 2026-10-29) |

## What a refused account is

Kept by what does not change: a fediverse account by its address (the
actor's `id`, `activitypub_actors.uri`), a Bluesky account by its DID
(`atproto_actors.did`). With it, how it was shown when refused (name and
handle), why, who refused it and when.

Refused, an account:

- is not listened to: an activity it sends is answered `403` before its key
  is fetched (`inbox.go`, beside the server check), and what Bluesky tells
  of it is passed over (the stream asks for it no more: `Wanted`; posts,
  reposts, reactions and imports check its DID);
- cannot be looked up, followed or backfilled from here;
- is sent nothing: it is not among a post's recipients when they are
  picked (a mention of it goes to nobody), and its follows are gone, so
  nothing is addressed to it as a follower;
- is removed, with what is here of it, at once on the fediverse and within
  a minute on Bluesky (below).

Stopping refusing it lets it be looked up and followed again. Nothing that
was removed comes back.

## Removing what is here

Both services have it (`forget`): the follows, the profile's pictures, the
node with its posts and replies and every edge (graph-grpc-service
`DeleteRemoteActor`), and the service's own row. Two things are added, which
removal for any reason then has too:

1. **The pictures of their posts** are removed from storage, as when one
   post of theirs is deleted (`deleteRemoteNoteFiles`), and their
   `photo_files` rows.
2. **The other services are told**, with the event a purge sends
   (`node.actor.<service>.person.purged.v1`): abuse-report-service closes
   the cases about them and removes the reports they filed, and
   recommendation-service forgets what was chosen about them.

On the fediverse the service that is asked removes them at once. On Bluesky,
atproto-service removes the refused accounts it keeps within a minute, by a
look at the list every half minute, and asks the stream for them no more.

## The list

One table for both networks, `federation_account_blocks`:

| Column | What |
| --- | --- |
| `id` | Its own number |
| `network` | `activitypub` or `atproto` |
| `identifier` | The actor's address, or the DID. Unique with `network` |
| `handle`, `name` | How it was shown when refused, for the list: the account is gone |
| `reason` | Why, for the administrators who come after. Never sent to it |
| `created_by`, `created_at` | Who refused it, and when |

Read by both services through `anahita-libs/federation` (`RefusedAccounts`,
`IsRefused(network, identifier)`), remembered for half a minute as the
servers' list is, since the inbox asks it of every message.

## The API

Served by activitypub-service, beside the servers' list
(`/activitypub/blocks`), which is for both networks too: refusing
`bsky.social` refuses Bluesky. No new path for the gateway.

| Request | Who | Does |
| --- | --- | --- |
| `GET /activitypub/refused-accounts?q=&limit=&start=` | administrators | A page of the list, newest first, with `pagination.total`. `q` narrows it to names, handles and reasons |
| `POST /activitypub/refused-accounts` `{ "actor_id": 123, "reason": "…" }` | administrators | Refuses the profile kept here as `actor_id`, and removes it |
| `POST /activitypub/refused-accounts` `{ "address": "@bob@example.net", "reason": "…" }` | administrators | Refuses an account by its handle or address, kept here or not: `@user@server`, an actor's address, a Bluesky handle or a DID, looked up to find what does not change |
| `DELETE /activitypub/refused-accounts/:id` | administrators | Stops refusing it |

`409 is_local` for a profile here, `409 already_refused` for one on the
list, `404` for one that cannot be found.

## The web app

1. **Site settings → Fediverse**, two tabs: **Servers**, as today, and
   **Accounts**: each refused account with its name and handle, the network
   (Fediverse or Bluesky), why and when, a search, and Stop refusing
   beside each; **Refuse an account** opens a dialog for a handle or an
   address and the reason, as Refuse a server does.
2. **A profile from elsewhere**, in its ⋮ menu, for administrators:
   **Refuse on this site**, a dialog with the reason that says what it
   does, then back to where the viewer came from. **Remove as spam**, which
   such a profile cannot have (the purge is for profiles here), is not
   offered on it.
3. **A report** about a profile from elsewhere, or a post of one:
   **Refuse account** beside Open, which refuses it and resolves the report
   (outcome `refused`, new).

Strings in the five base languages.

## Parts

| Part | What | Where |
| --- | --- | --- |
| 1 | The table (migration, `init.sql`, `upgrade.sql`) and `anahita-libs/federation` `RefusedAccounts` | services |
| 2 | activitypub-service: the checks, the removal with its pictures and its event, the API | services |
| 3 | atproto-service: the checks by DID, `Wanted`, the minute's removal, the event | services |
| 4 | abuse-report-service: the outcome `refused` | services |
| 5 | The web app: the tabs, the dialog, the profile's menu, the report's action | web |
| 6 | Tests (both services, against a real database where the removal is) and docs: `federation.md`, `permissions.md`, `api.md`, the API reference | both |

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-29 |
| Parts 1 to 6 | done 2026-10-29, tried on the development stack: a fediverse account refused and removed at once, a Bluesky one within thirty seconds, refusals again and of a profile here turned away, the list searched, both let back |
| Who sees the list | administrators open Site settings for the Fediverse tab's Accounts alone; the servers stay super administrators' (the user, 2026-10-29) |
