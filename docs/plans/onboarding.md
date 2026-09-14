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
is walked through filling it in and through choosing who to follow, then lands
on a dashboard with something in it. Existing members with empty profiles get
the same walk-through on their next sign-in.

---

## Status

`⬜ todo` · `🚧 in progress` · `✅ done` · `⛔ blocked`

| item | state | depends on | commit |
| --- | --- | --- | --- |
| 1. Onboarding state on the account | ⬜ | — | |
| 2. Post count on actor nodes | ⬜ | — | |
| 3. Suggested actors by hashtag | ⬜ | 2 | |
| 4. Picked actors configuration | ⬜ | — | |
| 5. `OnboardingGate` | ⬜ | 1 | |
| 6. Steps: avatar, profile | ⬜ | 5 | |
| 7. Steps: hashtags, suggested follows | ⬜ | 3, 4, 5 | |
| 8. Dashboard nudge after skipping | ⬜ | 1 | |

---

## The flow

```
sign in → agreements → avatar → name, bio, pronouns → hashtags
        → our picks → accounts matching your hashtags → dashboard
```

1. **Agreements** — already built (`AgreementsGate`). Mandatory, and always
   first: nothing below is offered until they are accepted.
2. **Avatar** — upload and crop, through the existing avatar endpoint.
3. **Display name, bio, pronouns** — pre-filled with whatever is already there.
   Somebody who applied through `approval` mode arrives with name and bio from
   their application and moves straight through. Pronouns use the existing
   `SelectPronouns`, where "unset" is a real option and the default.
4. **Hashtags** — choose topics of interest from a list. The list comes from
   the existing `hashtags/_browse/trending` and `hashtags/_browse/ordered_by_degree`
   endpoints; no backend work.
5. **Our picks** — a list of actors chosen by the installation (item 4). When
   the person was invited, **the inviter is added to the top of this list**.
6. **Accounts matching your hashtags** — actors active under the chosen
   hashtags who clear the follower and post thresholds (item 3). **If none
   qualify, the step is skipped** and the person goes straight to the dashboard.
7. **Dashboard.**

### Nothing is followed automatically

No step creates a follow on the person's behalf, the inviter included. A follow
is public — the edge is public, `follower_count` is public — and
`AllowFollowRequest` exists so an actor can require approval first, which an
automatic follow would have to either honour or bypass. Every follow in steps 5
and 6 is a click the person makes. Neither Mastodon nor Bluesky follows an
inviter automatically either.

### Skipping is possible, but not encouraged

Every step can be skipped, and the link to do so is deliberately quiet: a small
text link, below and away from the primary action, never a button of equal
weight. The intent is that people fill their profiles in; somebody who wants to
do it later still can.

- The primary action ("Continue", "Upload", "Follow selected") is the only
  prominent control on each step.
- "Skip for now" is a plain text link in secondary colour.
- Skipping one step moves to the next; it does not abandon the flow.

### Existing members are included

The gate is the same for somebody who joined a minute ago and somebody who
joined in 2012: an incomplete profile and no record of having been through
onboarding sends them there. At 89% of the current membership this is most
people's next sign-in, which is the point — and the reason skipping has to
exist at all.

### A gate on the interface, not on the API

Like `AgreementsGate`, this decides what the web app shows. Tokens are
unchanged and API clients are unaffected.

---

## Work items

### 1. Onboarding state on the account

The gate needs to know whether somebody has already been through onboarding,
or a person who skipped everything is sent back on every sign-in.

- `auth_accounts.onboarded_at datetime DEFAULT NULL` — set when the person
  reaches the dashboard through the flow, **whether they completed the steps or
  skipped them**. `NULL` means "has not been through it".
- Exposed on `/oauth/userinfo` beside `tos_agreement` / `privacy_agreement`,
  so it arrives with the viewer and the gate has no extra request to make.
- `PATCH /onboarding` sets it for the signed-in viewer. No id in the path, same
  as `/agreements`. nginx block needs the `auth_request` lines from the start.
- Belongs on the account rather than the node: it is web-app state about a
  person's own session, never shown to anybody else, and must not travel on
  `graphpb.Node`.

Open: should a later profile regression (somebody deletes their avatar) re-open
onboarding? Proposed: no. `onboarded_at` is set once; item 8 covers the rest.

### 2. Post count on actor nodes

`nodes` stores `follower_count`, `leader_count`, `subscriber_count` — but no post
count, and the threshold in item 3 needs one.

Choose between:

- **a stored `post_count` column**, maintained on create/delete of notes,
  articles, photos and topics, with a one-off backfill — cheap to filter and
  sort on, one more counter to keep honest;
- **a count at query time** over `nodes.owner_id` restricted to medium types —
  always correct, and fine while suggestions are computed for one person at a
  time.

Proposed: count at query time first, and store it only if item 3 is slow.

Decide which types count as a post. Proposed: notes, articles, photos, topics;
not comments, not revisions.

### 3. Suggested actors by hashtag

A graph query, exposed through an HTTP endpoint the web app can call:

> actors who have tagged posts with any of these hashtags, with at least
> `ONBOARDING_SUGGEST_MIN_FOLLOWERS` followers and
> `ONBOARDING_SUGGEST_MIN_POSTS` posts, excluding the viewer, anybody they
> already follow, anybody who blocks them or is blocked by them, and disabled,
> archived or deleted actors — ordered by how many of the chosen hashtags they
> match, then by follower count.

- Thresholds are configuration, not constants. 38 people clear 10 followers on
  the reference data; a new installation clears nothing. The step is skipped
  when the result is empty, so a threshold set too high degrades to a shorter
  onboarding rather than a broken one.
- Capped result (proposed 20).
- People and groups both qualify, or people only? Proposed: both, labelled.

### 4. Picked actors configuration

The installation's own picks for step 5.

- For now: `ONBOARDING_PICKS`, a comma-separated list of aliases, read by the
  service that serves the step. Unknown or disabled aliases are dropped
  silently, so a renamed account does not break onboarding.
- Later: an administration screen in the settings plan replaces the variable.
- The inviter is prepended at request time for somebody who accepted an
  invitation. That needs the inviter recorded against the account — the
  invitation token has `CreatedBy`, but tokens are not a durable record. Add
  `auth_accounts.invited_by bigint unsigned DEFAULT NULL`, written at
  invite-accept (and at approval, for member-vouched applications).
- The inviter and the picks go through the same exclusions as item 3: not the
  viewer, not blocked, not disabled.

### 5. `OnboardingGate`

Same shape as `AgreementsGate` and nested inside it, so agreements always come
first.

- Sends a signed-in viewer to `/onboarding` when `onboardedAt` is empty **and**
  the profile is incomplete (no avatar, or no bio).
- A viewer with a complete profile and no `onboardedAt` is not sent; the
  follow steps alone are not worth interrupting somebody who is already set up.
  Proposed; revisit if the follow suggestions prove valuable enough.
- Reachable while outstanding: `/onboarding`, `/agreements`, `/legal`,
  `/support`, `/oauth/callback`.

### 6. Steps: avatar, profile

- `containers/onboarding/` with a stepper; each step a component.
- Reuses the existing avatar upload and `people/Settings/InfoForm` field
  components rather than duplicating them.
- A step whose data is already present is shown pre-filled, not skipped, so the
  person can see what others will see.

### 7. Steps: hashtags, suggested follows

- Hashtags: chips, multi-select. Selection is held in the flow, not saved to the
  profile — there is no "interests" field, and adding one is not part of this
  plan.
- Follows: a list with a follow button per actor plus "Follow selected". The
  inviter is shown first, **not pre-selected**.
- Step 6 is skipped when item 3 returns nothing.
- Reaching the dashboard calls `PATCH /onboarding`.

### 8. Dashboard nudge after skipping

Somebody who skipped still has an empty profile. A small, dismissible "Complete
your profile" card on the dashboard while avatar or bio is missing, linking to
the profile settings. Dismissal is per-browser (`localStorage`); it is a
convenience, not a record.

---

## Deferred to other plans

- **Settings UI** — administering `ONBOARDING_PICKS` and the thresholds.
- **Interests on the profile** — storing chosen hashtags against the person and
  using them in the feed; see `feed-generators.md`.

---

## Open questions

1. Should a profile that later becomes incomplete re-open onboarding? Proposed
   no (item 1).
2. Stored post count or query-time count (item 2)?
3. Groups in suggestions, or people only (item 3)?
4. Should a complete profile with no `onboardedAt` still see the follow steps
   once (item 5)?

---

## Verification

- New account in `open` mode: signs in → agreements → onboarding → dashboard;
  `auth_accounts.onboarded_at` set.
- Same account signs out and in: goes straight to the dashboard.
- Skip every step: dashboard reached, `onboarded_at` set, nudge card shown.
- Invited account: inviter first in step 5, not pre-selected; no edge exists
  until the follow is clicked.
- Existing member with no avatar: redirected on next sign-in.
- Existing member with avatar and bio: not redirected.
- Thresholds above every actor's counts: step 6 is skipped, not empty.
- A blocked actor never appears in either list.
- API request with the member's token while onboarding is outstanding:
  unaffected.
