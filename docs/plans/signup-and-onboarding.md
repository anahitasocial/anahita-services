# Signup, registration modes, and agreements

## Context

Signup is the last credential flow still living in the web app. Login, password
reset, invite-accept, email-change confirmation and account restore are all
server-rendered by auth-service; `POST /signups` is a JSON endpoint the SPA
calls. That asymmetry costs more than tidiness:

- **`REACT_APP_SIGNUP_CLOSED` is not enforced.** It hides a tab.
  `SignupPermissions.CanAdd` returns true for any guest, so `POST /signups`
  works regardless of what the client was told.
- **Email is never verified.** `signupHandler.Confirm` is a stub returning 202
  and is not routed. An account is fully usable with an address nobody
  controls.
- **Signup is not rate limited.** The limiter has five scopes, all password
  flows.
- Anti-bot measures that want a server-rendered form — honeypot, timing,
  proof-of-work, a CSRF-bound submission — cannot be done honestly in a React
  bundle an attacker can read.

**Outcome:** signup is server-rendered by auth-service, the installation
chooses how open it is, and what a person has to supply depends on that choice.
Agreements are accepted in the web app after login, checked on every sign-in so
a revision re-prompts by itself, and collected in exactly one place. Profile
completeness is onboarding, not a gate.

---

## Status

| item | state | commit |
| --- | --- | --- |
| 1. `REGISTRATION_MODE` + NodeInfo | ⬜ | |
| 2. Signup moves to auth-service | ⬜ | |
| 3. Email verification | ⬜ | |
| 4. Anti-bot baseline | ⬜ | |
| 5. Approval queue (backend only) | ⬜ | |
| 6. Agreements checked at every sign-in | ⬜ | |
| 7. Display name becomes optional | ⬜ | |
| 8. Onboarding | ⬜ separate plan | |

---

## Registration modes

Two independent questions: **how somebody starts** — they find you, or an
administrator invites them — and **whether a human decides**. The mode answers
both.

| mode | self-serve | invites |
| --- | --- | --- |
| `open` | anybody, instantly | work |
| `approval` | anybody may apply; an administrator decides | work, **skipping the queue** |
| `invite` | none | work |
| `closed` | none | **do not work** |

Default `closed`. An installation that has not thought about it should not be
open to the internet.

**`invite` and `closed` are not the same thing, and that is why both exist.**
`invite` is the ordinary running state for a private community: no strangers,
but administrators can let people in. `closed` is a kill switch — nothing gets
in, and **outstanding invite tokens stop working too**. That distinction only
matters on the day it matters: locking down during an abuse incident, or
freezing an archived community, where "stop issuing invites" is not enough
because the ones already sent are still live.

**An invitation is already an approval.** In `approval` mode an invited person
does not join the queue: an administrator decided when they sent it, and
queueing would mean approving the same person twice — once by sending, once by
clearing them from a list. Administrators should not have to approve people
they personally invited.

That holds only while invites are administrator-only, which they are today —
the permission calls them "inherently a managing people operation". If invites
are ever opened to ordinary members, an invitation becomes a recommendation
rather than a decision, and queueing it would be the right call. Worth
revisiting then, not before.

### Enforced in three places

One is not enough, because each answers a different question.

1. `SignupPermissions.CanAdd` — the handler refuses whatever rendered the form.
2. The signup page — renders the reason rather than a form that will fail.
3. **NodeInfo** (`/.well-known/nodeinfo` → `openRegistrations`) — the standard
   other fediverse servers already read, and what the web app reads to decide
   whether to show a "Sign up" link at all. It replaces
   `REACT_APP_SIGNUP_CLOSED`, which was only ever a client-side opinion.

---

## What signup asks for, per mode

Minimal by default. The principle is that an installation should ask for what
it needs to make a decision, and nothing more.

| | `open`, and invite-accept | `approval` |
| --- | --- | --- |
| handle | ✅ | ✅ |
| email | ✅ | ✅ |
| password | ✅ | ✅ |
| display name | ❌ | ✅ |
| bio | ❌ | ✅ |
| website URL | ❌ | ✅ |
| terms acceptance | ❌ — asked at first sign-in | ❌ — same |

**Neither Mastodon nor Bluesky asks for a display name at signup.** Both take
the handle, email and password — Bluesky also a date of birth — and offer the
display name during a skippable onboarding afterwards, where an empty one falls
back to the handle. Anahita is currently the outlier.

**Approval mode is the exception, and for a reason.** A human is being asked to
decide, and deciding on an email address alone is not deciding. Mastodon
handles this with a "why do you want to join?" free-text box; the three fields
here serve the same purpose and have the advantage of being the profile itself
rather than a note thrown away after the decision. Somebody rejected has not
had to write anything they would not have written anyway.

---

## Work items

### 1. `REGISTRATION_MODE` and NodeInfo

- `config/bootstrap.go` — `REGISTRATION_MODE`, validated against the four
  values, defaulting to `closed`.
- `SignupPermissions.CanAdd` gains the mode check.
- `/.well-known/nodeinfo` and the NodeInfo 2.1 document, carrying
  `openRegistrations` and `usage.users.total`. New surface; it is also the
  thing that makes this installation legible to the rest of the fediverse, so
  it is worth doing properly rather than as a one-field stub.
- The web app reads it instead of `REACT_APP_SIGNUP_CLOSED`, which goes.

**`closed` must not break the super-administrator seed.** That does not go
through signup at all, and an installation that cannot seed its first
administrator because registration is closed is one nobody can ever administer.

**`closed` must break outstanding invites**, which is the one place it differs
from `invite` and the reason both exist. The token stays in the table and the
accept page refuses it, so lifting the lockdown brings the invitation back
rather than silently destroying it.

### 2. Signup moves to auth-service

- `GET /signup` and `POST /signup`, server-rendered, beside the invite-accept
  pages they will resemble.
- On success: create the actor and the account exactly as now, write the IdP
  session, and redirect into the OAuth flow — the same ending invite-accept
  already has. No separate "now sign in" step.
- `POST /signups` (the JSON endpoint) is retired. **Check `anahita-mobile`
  first**: if it calls that endpoint it needs the browser flow instead, which
  is what OAuth best practice wants from a native app regardless.
- The web app's `/auth/signup` tab becomes a navigation to the auth host, the
  way logout already is.
- Keep the on-blur "is this handle taken" check — roughly fifteen lines of
  plain `fetch` against endpoints that already exist. Losing it would make the
  move a visible downgrade for no gain.

### 3. Email verification

Its own item because it is a gap rather than a refactor, and because it is the
single cheapest thing on this list that stops bots.

- Route `Confirm`, and make it do something: a `signupconfirm` token in the
  existing `tokens` table, a link in the welcome mail, a server-rendered
  landing page beside the others.
- **Unverified accounts cannot sign in.** Anything softer is a verification
  step people learn to ignore.
- In `approval` mode, verification happens **before the application reaches the
  queue**. Otherwise administrators moderate unverified junk, which is the
  work the queue exists to avoid.

### 4. Anti-bot baseline

Ranked by what actually works, not by what is most visible.

| measure | verdict |
| --- | --- |
| invite-only | the most effective measure there is; already built |
| approval queue | very high, human in the loop; item 5 |
| email verification | high, kills volume; item 3 |
| rate limit per IP and per subnet | high; the limiter already exists, signup just has no scope |
| honeypot field + submission timing | free, no user friction, low ceiling |
| proof-of-work (Altcha / mCaptcha shape) | the one to add if `open` is ever used |
| CAPTCHA | last resort |
| disposable-domain blocklists | an MX check gets most of the value without the churn |

In scope here: **rate limiting, honeypot and timing.** All three are a day's
work and need no dependency.

**Proof-of-work over CAPTCHA, when it comes to that.** Commercial solver
services defeat the major CAPTCHAs for about a dollar per thousand, so they now
filter the lazy and tax the legitimate. Proof-of-work costs an attacker per
attempt, needs no third party, and asks the person signing up for nothing —
which also means not handing a view of every sign-up to Cloudflare or Google,
on a platform whose whole direction is federation and self-hosting.

### 5. Approval queue — backend only

**The admin UI is deliberately not here.** It belongs with the other settings
work, in its own plan.

- `registration_applications`: handle, email, password hash, display name, bio,
  website URL, status, decided_by, decided_at, the request's IP and user agent,
  and the email-verification state.
- **Not a node and not an `auth_accounts` row until approved.** Creating a
  disabled actor per application would fill the graph with people who were
  never admitted, consume handles, and — once federating — publish them.
- The handle is checked at apply time and again at approval. The apply-time
  check is courtesy; `uq_unique_alias` at approval is the enforcement, and two
  applications wanting the same handle is a case the second applicant has to
  be told about.
- Approval creates the actor and the account through the same path signup uses,
  so there is one way an account comes into existence.
- Rejection emails the applicant. Silence reads as a broken site, and somebody
  who is never coming back should be told so rather than waiting.

**A queue with no UI is not usable**, so this item ships behind
`REGISTRATION_MODE=approval` and that mode stays undocumented until the
settings plan lands. Worth considering as a bridge: approve and reject by
tokenised link in the notification email to administrators, which reuses the
invite machinery and makes the feature usable on day one.

### 6. Agreements checked at every sign-in — in the web app

**Decided: client-side, after login.** The alternative was gating token
issuance in auth-service, which would be enforcement rather than UX; the
trade-off is understood and the decision is made.

The split that still holds: **the client collects the acceptance, the server
records it.** Version, timestamp, IP, user agent and OS have to be taken from
the request server-side, or the record is a claim the browser made about
itself — and those columns exist precisely so it is not that.

- `/oauth/userinfo` gains the acceptance state **per document** — accepted
  flag and version for each — so the viewer knows which cards to show.
  The repository already reads `tos_accepted` and `tos_accepted_version` back,
  and `entities.Consent.NeedsAcceptance` already answers the question
  including the version-moved-on case.
- Two authenticated endpoints, one per document, each recording through a
  split `EditConsent` — taking the version from config and the audit trio from
  the request, never from the client.
- `GET /agreements` already publishes the version and the URL of each, from
  configuration (`TOS_VERSION`/`TOS_URL`, `PRIVACY_VERSION`/`PRIVACY_URL`), so
  the interstitial has one place to ask what is in force and where to fetch
  the text from.
- `AuthenticatedRoute` renders the agreements interstitial instead of the
  requested route while anything is outstanding. It already runs on every
  session resolution, so "every sign-in" comes for free.

#### What the interstitial looks like

Modelled on an implementation of the same flow that already works elsewhere.

- **One card per outstanding document**, and only the outstanding ones. A
  document still current shows as already accepted rather than being hidden,
  so the page explains itself rather than appearing arbitrary.
- **The text is rendered in the page**, in a scrollable box — not linked. A
  link is not a presentation, and "they could have clicked it" is a weaker
  thing to have to argue than "it was on the screen they accepted from".
- **Accepted independently, one call each, each carrying its own version.**
  This is the part worth copying: a revision to the Privacy Policy alone must
  not re-ask for the Terms. `Users.EditConsent` writes both at once today and
  needs splitting, or the two versions cannot move apart.
- A Continue button appears once nothing is outstanding.

**Compare versions in order, not for equality.** `Consent.NeedsAcceptance`
currently does string inequality, which re-prompts when a version goes
*backwards* — a rollback, or an operator fixing a typo, and everybody is asked
again for text they have already accepted. Parse the dotted parts and compare
numerically, so only a genuine advance re-prompts. Empty still means "never
accepted".

**Decided: nothing collects acceptance except the interstitial.** It is
checked on every sign-in, and the check is version-aware, so a revision
re-prompts everybody automatically rather than needing a migration or a
one-off sweep. That is the argument for putting it here rather than at account
creation: creation happens once, and the terms change more than once.

"Checked every sign-in" is not "asked every sign-in".
`Consent.NeedsAcceptance(current)` answers true only when nothing is on record
or the recorded version is not the one in force, so somebody current is never
interrupted.

**Three places stop collecting it**, all of them shipped in the
`people_people` refactor's item 10:

- the signup form — the four request fields go with it;
- the invite-accept form — same;
- the set-password page, where the seeded super administrator accepted. That
  administrator is now asked at first sign-in like everybody else, and
  `password.go` loses a branch it should never have needed.

What that item built is still the substrate and none of it is wasted: the
twelve columns, `entities.Consent`, `Users.EditConsent`, the version check and
the audit trio all stay. Only the three collection points move into one.

**Worth keeping on the signup form even without a checkbox:** the sentence
"By creating an account you agree to the Terms of Service and Privacy Policy",
with both linked. It costs nothing, it is what Bluesky does, and it means
nobody can say they were never shown them.

**This item must not ship before the Terms and the Privacy Policy are real.**
They are currently the boilerplate in `src/assets/default/pages/`, with
`[Your Organisation]` and `[jurisdiction]` still in them. Switching the gate on
first would record all 822 existing members as having accepted a template, and
the acceptance record — the entire point of the version and IP columns — would
be worth nothing.

### 7. Display name becomes optional

Follows from the field table. The interesting part is what "absent" means.

`nodes.name` is left **empty** rather than defaulted to the handle, and
rendering falls back. That is what Mastodon and Bluesky store, and what both
protocols expect: an ActivityPub consumer falls back from `name` to
`preferredUsername`, ATProto from `displayName` to the handle.

The reason it matters here is onboarding: if `name` is set to the handle, there
is no way to tell "never set one" from "chose their handle as their name", and
the onboarding step has no signal to act on.

- `getPersonName` returns `actor:unknown` for an empty name today, and
  `getActorInitials` returns `??`. Both fall back to the alias instead.
- About eight further places render `actor.name` directly; they route through
  the helper.
- `PersonEditRequest.Name` is `required,max=100` and becomes `omitempty`, or
  people can set a display name and never clear it.

### 8. Onboarding — separate plan

Post-login, skippable, and **not a gate**. Avatar, bio, pronouns, topics.

**Accepting an invitation does not create a follow.** Invites are admin and
super-admin only — the permission calls them "inherently a managing people
operation" — so the inviter is usually somebody doing a job, not a friend.
Auto-following would build a graph centred on staff as an artifact of the
mechanism: an administrator who onboards fifty people collects fifty followers
they never chose, and every new member's first relationship is with whoever
processed them.

It is also a public claim made on somebody's behalf. `follower_count` and
`leader_count` are public, the edge is public, and `AllowFollowRequest` exists
precisely so an actor can require approval first — which an automatic follow
would have to either honour, dragging a pending-request state into signup, or
deliberately bypass.

Neither Mastodon nor Bluesky creates a follow from an invite.

The problem underneath is real, though: an account with no follows has an empty
feed, which is the worst possible first impression. That is what this item is
for. **Put the inviter at the top of the suggested follows, pre-ticked** — the
relationship is the best signal available — and let one click decline it. If
invites are ever opened to ordinary members the friend case becomes common and
the argument weakens, but a pre-ticked suggestion is still the right shape: the
click is the difference between suggesting and deciding for somebody.

Measured on the live data, hard-gating on profile completeness would bounce
most of the member base out of the app on their next login:

| | missing, of 822 |
| --- | --- |
| display name | 0 |
| avatar | 488 |
| bio | 716 |

Blocking on those blocks the API, breaks integrations, and teaches people to
upload a grey square to get past the wall. Agreements are a precondition for
service; a bio is a product preference, and the two want different enforcement.

---

## Deferred to other plans

- **Settings UI** — the approval queue's administration screen, alongside the
  other settings work.
- **Onboarding** — item 8.

---

## Open questions

1. **Date of birth.** Bluesky collects one for age assurance; we collect
   nothing, and the boilerplate Terms carry a `[minimum age]` placeholder that
   nothing enforces. Age assurance is getting more demanding under the UK
   Online Safety Act and the DSA, not less. This is the one place where
   "minimum information" and legal exposure pull against each other, and it is
   a decision rather than a technical call.
2. **`/auth/is/email` is a membership oracle.** Forgot-password goes to real
   lengths to be enumeration-safe — an identical response either way — and
   this endpoint then hands out the same fact directly to anybody. Either the
   convenience is worth it and the forgot-password care is theatre, or it is
   not and this should go. Mastodon does not expose it.
3. **Does invite-accept also drop the name field?** Somebody vouched for the
   invitee, so there is less to judge; the argument for minimal applies there
   too, but it is a separate form.
4. **Re-application after rejection.** Silently allowed, blocked, or
   rate-limited?

---

## Verification

- `REGISTRATION_MODE=closed` — `POST /signup` refuses, the page says so,
  NodeInfo reports `openRegistrations: false`, **an outstanding invite link is
  refused**, and the super-admin seed still runs on a fresh database.
- `REGISTRATION_MODE=invite` — self-serve refuses, and the same invite link
  completes. This pair is the whole reason both modes exist, so test them
  together or the difference is untested.
- Lift `closed` back to `invite`: the invitation refused a moment ago works
  again. The token was suspended, not destroyed.
- `REGISTRATION_MODE=approval` — an invited person **skips the queue** and
  lands with an account, while a self-serve applicant waits in it.
- `REGISTRATION_MODE=open` — a signup completes end to end and lands signed in,
  and the same address cannot sign up twice.
- Sign up, then try to sign in before clicking the confirmation link: refused.
- Submit the signup form with the honeypot filled: refused, and indistinguishable
  from success to the submitter.
- Hammer signup from one address: rate limited.
- `REGISTRATION_MODE=approval` — an application lands in the table with no node
  and no `auth_accounts` row; approving creates both; rejecting emails the
  applicant and leaves no actor behind.
- A person with no acceptance on record is bounced to the interstitial after
  login, and the row afterwards carries the version, the timestamp and a real
  IP and user agent.
- Bump `TOS_VERSION`: the same person is asked again.
- A person with no display name renders as their handle everywhere, with
  initials from the handle rather than `??`.
