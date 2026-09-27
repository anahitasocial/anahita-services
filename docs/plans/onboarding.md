# Onboarding

## Context

Signup now asks for the least it can: username, email, password. Everything
that makes a profile worth visiting — avatar, display name, bio, pronouns — and
everything that makes the first feed worth reading — who to follow — is left to
this plan.

An account with no avatar, no bio and no follows lands on an empty dashboard
and a profile nobody recognises. That is the worst first impression the product
can make, and it is also where most existing members already are:

| measured on the live data | of 822 people |
| --- | --- |
| no avatar | 488 |
| no bio | 716 |
| no display name | 0 |
| missing an avatar or a bio | 728 (89%) |
| people with 10+ followers | 38 |

**Outcome:** after the agreements, a signed-in person with an incomplete profile
is walked through filling it in and through following the accounts the
installation features, then lands on the dashboard. Existing members with empty
profiles get the same walk-through on their next sign-in.

The web app's half is `anahita-web/docs/plans/onboarding.md`.

---

## Phases

1. **Profile** — avatar, display name, bio, pronouns. Fixes the 89% on its own.
2. **Featured accounts** — the installation marks accounts as featured; onboarding
   offers them, with the inviter first.
3. **Suggestions from interests** — hashtags and locations the person picks,
   matched against active, established accounts. **Deferred to a recommendation
   engine** (see the end of this plan); onboarding will be its first consumer.

## Status

`⬜ todo` · `🚧 in progress` · `✅ done` · `⛔ blocked`

| item | phase | state | depends on | commit |
| --- | --- | --- | --- | --- |
| 1. Onboarding state on the account | 1 | ✅ | — | `064dc08` |
| 2. `invited_by` on the account | 2 | ✅ | — | `845ad76` |
| 3. `featured_at` on actor nodes | 2 | ⬜ | — | |
| 4. Featuring an actor | 2 | ⬜ | 3 | |
| 5. Browsing featured actors | 2 | ⬜ | 3 | |
| 6. `GET /onboarding` | 2 | ⬜ | 1, 2 | |

---

## The flow

```
sign in → agreements → avatar → name, bio, pronouns → featured accounts → dashboard
```

1. **Agreements** — already built (`AgreementsGate`). Mandatory, and always
   first: nothing below is offered until they are accepted.
2. **Avatar** — upload through the existing avatar endpoint.
3. **Display name, bio, pronouns** — pre-filled with whatever is already there.
   Somebody who applied through `approval` mode arrives with name and bio from
   their application and moves straight through.
4. **Featured accounts** — the accounts the installation features (items 3–5).
   When the person was invited, **the inviter is shown first**.
5. **Dashboard.**

### An empty list skips its step

A step that would show an empty list is not shown. A fresh installation with no
featured accounts and no inviter goes avatar → profile → dashboard. A failed
request counts as empty, so onboarding can never trap anybody.

### Nothing is followed automatically

No step creates a follow on the person's behalf, the inviter included. A follow
is public — the edge is public, `follower_count` is public — and
`AllowFollowRequest` exists so an actor can require approval first, which an
automatic follow would have to either honour or bypass. Every follow is a click
the person makes. Neither Mastodon nor Bluesky follows an inviter automatically
either.

### Skipping is possible, but not encouraged

Every step can be skipped, and the link to do so is deliberately quiet: a small
text link, below and away from the primary action, never a button of equal
weight. Skipping one step moves to the next; it does not abandon the flow.

### Existing members are included

The gate is the same for somebody who joined a minute ago and somebody who
joined in 2012: an incomplete profile and no record of having been through
onboarding sends them there.

### A gate on the interface, not on the API

Like `AgreementsGate`, this decides what the web app shows. Tokens are
unchanged and API clients are unaffected.

---

## Work items

### 1. Onboarding state on the account — phase 1

The gate needs to know whether somebody has already been through onboarding,
or a person who skipped everything is sent back on every sign-in — and whether
their profile is incomplete, without a second request.

- `auth_accounts.onboarded_at datetime DEFAULT NULL` — set when the person
  reaches the dashboard through the flow, **whether they completed the steps or
  skipped them**. `NULL` means "has not been through it". Set once; a profile
  that later becomes incomplete does not re-open onboarding (the web app's
  dashboard nudge covers that).
- `/oauth/userinfo` gains, beside `tos_agreement` / `privacy_agreement`:
  - `onboarded_at` — the timestamp, or `null`;
  - `has_bio` — whether `nodes.body` has anything in it but whitespace.
    `avatar_urls` is already there (and absent when there is no avatar); the
    bio was not. `userColumns()` already joins `nodes`, so this is one more
    selected column, not another request.
- `PATCH /onboarding` sets `onboarded_at` for the signed-in viewer if it is not
  already set. No id in the path, same as `/agreements`. `Users.MarkOnboarded`,
  a single-purpose method like `MarkEmailVerified`.
- nginx: a `location /onboarding` copied from `/agreements`, with the
  `auth_request` lines from the start, allowing `GET, PATCH, OPTIONS`.
- Belongs on the account rather than the node: it is web-app state about a
  person's own session, never shown to anybody else, and must not travel on
  `graphpb.Node`.

### 2. `invited_by` on the account — phase 2

The inviter is shown first in the featured step. The invitation token has
`CreatedBy`, but tokens are not a durable record.

- `auth_accounts.invited_by bigint unsigned DEFAULT NULL`.
- Written at invite-accept from `token.CreatedBy` (not the looked-up inviter,
  which falls back to a stub with id 0), and at signup-request approval from
  `signup_requests.invited_by`, for member-vouched applications.
- Self-serve signups leave it `NULL`.

### 3. `featured_at` on actor nodes — phase 2

The installation's picks. A flag on the actor rather than a list in
configuration or a file:

- changing it is a click on the profile, not a redeploy;
- it cannot go stale — a renamed, disabled or deleted account drops out through
  the same query that reads the flag;
- it is reusable beyond onboarding (an Explore section, for instance).

`nodes.featured_at datetime DEFAULT NULL`, indexed. A timestamp rather than a
boolean: it is the flag and the default order at once (most recently featured
first). People and groups can both be featured.

Carried like `deleted_at`: `graphpb.Node.featured_at`, both scans in
`graph-grpc-service/data/repositories/helpers.go`, `AnahitaNode.FeaturedAt`,
`nodeToGrpc` / `grpcToGraphNode`, and `featured_at` on `ActorNodeItem` so the
web app can show the toggle's state.

### 4. Featuring an actor — phase 2

- `PATCH /people/:id/featured` and `PATCH /groups/:id/featured` with
  `{ "featured": true | false }`. **Super administrators only** — it is a
  statement by the installation, not by an administrator of one group.
- Same shape as `SetEnabled`: `ActorService.SetFeatured` in `actor.proto`,
  `ActorsGRPC.SetFeatured` in anahita-libs, a single-purpose repository method
  (`UPDATE nodes SET featured_at = …`), 200 when changed, 204 when not.
- A disabled, archived or deleted actor cannot be featured (409).

### 5. Browsing featured actors — phase 2

- `GET /people/?featured=1` and `GET /groups/?featured=1`, through the existing
  browse handlers, which already fill in the viewer's relationship
  (`is_leading_viewer` and so on), presign avatars and exclude actors blocked
  either way.
- `featured` added to `ActorBrowseRequest` (HTTP and gRPC). When set, the graph
  query adds `node.featured_at IS NOT NULL`, applies `SetArchiveQuery`, and
  orders by `featured_at DESC` unless an ordering is given.
- Accounts the viewer already follows are not excluded: for a new member there
  are none, and for an existing one "Following" is the honest thing to show.

### 6. `GET /onboarding` — phase 2

What the flow needs about the viewer that is not on userinfo:

```json
{ "invited_by": "alias-of-inviter" }
```

`invited_by` is `null` when there is no inviter, or when the inviter no longer
resolves. The web app reads the inviter's profile through `/people/:alias`, so
blocks and disabled accounts are handled there, not here.

---

## Deferred to other plans

- **Suggestions from interests (phase 3)** — the person picks hashtags and
  locations (both already have `_browse/trending` and `_browse/ordered_by_degree`);
  suggested accounts are those posting under them who clear a quality bar: an
  avatar and a bio, at least N followers, at least M posts, a post in the last
  12 months; excluding the viewer, blocks either way, disabled, archived and
  deleted actors, and anybody already featured. Ranked by how many picks they
  match, then followers. This is the first generator of the recommendation
  engine, not a one-off query.
- **Interests on the profile** — storing chosen hashtags against the person and
  using them in the feed; see `feed-generators.md`.

## Towards a recommendation engine

`feed-generators.md` already chooses the shape: rankers are unbundled from
storage and access control. A generator returns a **skeleton** — an ordered list
of ids, with a reason — and hydration applies blocks and access. The same
contract works for recommending actors:

| generator | input | needs follows? |
| --- | --- | --- |
| featured | `nodes.featured_at` | no |
| interests | chosen hashtags and locations | no |
| follows-of-follows | the viewer's leaders' leaders | yes |
| popular nearby | the viewer's locations | no |
| co-engagement | who liked what the viewer liked | yes |

Onboarding is the cold-start case: only generators that need no follows apply.
The ones that need follows belong on the dashboard ("Who to follow") after the
first few follows. Deterministic SQL first; no model.

---

## Verification

- New account in `open` mode: signs in → agreements → onboarding → dashboard;
  `auth_accounts.onboarded_at` set.
- Same account signs out and in: goes straight to the dashboard.
- Skip every step: dashboard reached, `onboarded_at` set.
- `/oauth/userinfo` for a person with a whitespace-only bio: `has_bio` is false.
- Invited account: `GET /onboarding` returns the inviter's alias; the inviter is
  first in the featured step, not pre-selected; no edge exists until the follow
  is clicked.
- Self-serve account: `invited_by` is `null`.
- Existing member with no avatar: redirected on next sign-in.
- Existing member with avatar and bio: not redirected.
- `PATCH /people/:id/featured` as an administrator who is not a super
  administrator: 403.
- Featuring then disabling an actor: gone from `GET /people/?featured=1`.
- A featured actor who blocks the viewer never appears in the list.
- No featured actors and no inviter: the step is skipped, not empty.
- API request with the member's token while onboarding is outstanding:
  unaffected.
- `make test` passes.
