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
Agreements are accepted after login, in the web app. Profile completeness is
onboarding, not a gate.

---

## Status

| item | state | commit |
| --- | --- | --- |
| 1. `REGISTRATION_MODE` + NodeInfo | ⬜ | |
| 2. Signup moves to auth-service | ⬜ | |
| 3. Email verification | ⬜ | |
| 4. Anti-bot baseline | ⬜ | |
| 5. Approval queue (backend only) | ⬜ | |
| 6. Agreements after login | ⬜ | |
| 7. Display name becomes optional | ⬜ | |
| 8. Onboarding | ⬜ separate plan | |

---

## Registration modes

Modelled on Mastodon, which is the convention the fediverse already knows.

| mode | who can get in | notes |
| --- | --- | --- |
| `open` | anybody | the anti-bot baseline matters most here |
| `approval` | anybody may apply; an administrator decides | the strongest filter that still allows strangers |
| `invite` | only somebody holding an invite token | the flow already exists and works |
| `closed` | nobody | the seed still runs — it is not a signup |

`REGISTRATION_MODE`, defaulting to `invite`. A default of `open` means an
installation that has not thought about it is open to the internet.

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

| | `open` / `invite` | `approval` |
| --- | --- | --- |
| handle | ✅ | ✅ |
| email | ✅ | ✅ (invite carries it) |
| password | ✅ | ✅ |
| display name | ❌ | ✅ |
| bio | ❌ | ✅ |
| website URL | ❌ | ✅ |

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
  values, defaulting to `invite`.
- `SignupPermissions.CanAdd` gains the mode check.
- `/.well-known/nodeinfo` and the NodeInfo 2.1 document, carrying
  `openRegistrations` and `usage.users.total`. New surface; it is also the
  thing that makes this installation legible to the rest of the fediverse, so
  it is worth doing properly rather than as a one-field stub.
- The web app reads it instead of `REACT_APP_SIGNUP_CLOSED`, which goes.

**`closed` must not break the super-administrator seed.** That path does not go
through signup, and an installation that cannot seed its first administrator
because registration is closed is one nobody can ever administer.

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

### 6. Agreements after login — in the web app

**Decided: client-side, after login.** The alternative was gating token
issuance in auth-service, which would be enforcement rather than UX; the
trade-off is understood and the decision is made.

The split that still holds: **the client collects the acceptance, the server
records it.** Version, timestamp, IP, user agent and OS have to be taken from
the request server-side, or the record is a claim the browser made about
itself — and those columns exist precisely so it is not that.

- `/oauth/userinfo` gains the acceptance state, so the viewer knows to gate.
  The repository already reads `tos_accepted` and `tos_accepted_version` back,
  and `entities.Consent.NeedsAcceptance` already answers the question
  including the version-moved-on case.
- A new authenticated endpoint records the acceptance through
  `Users.EditConsent`, taking the version from config and the audit trio from
  the request.
- `AuthenticatedRoute` renders the agreements interstitial instead of the
  requested route while anything is outstanding.

**Assumed, and easily overturned: signup drops its checkboxes.** With the
interstitial catching everybody, keeping them means acceptance happens in two
places and the signup form grows again. The consequence is that an account
exists for the span of one redirect having agreed to nothing, which is how most
platforms work. If you would rather they stay, say so — it is a small change
either way, but the two have to be decided together.

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
  NodeInfo reports `openRegistrations: false`, and the super-admin seed still
  runs on a fresh database.
- `REGISTRATION_MODE=invite` — the same, and an invite link still completes.
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
