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

`⬜ todo` · `🚧 in progress` · `✅ done` · `⛔ blocked`

| item | state | depends on | commit |
| --- | --- | --- | --- |
| 1. `REGISTRATION_MODE` + NodeInfo | ✅ | — | `e7d47c2` |
| 2. Signup moves to auth-service | ✅ | 1, 8 | `225b72c` |
| 3. Email verification | ✅ | — | `3ab5bef` |
| 4. Anti-bot baseline | ⬜ | 2 | |
| 5. Approval queue (backend only) | ⬜ | 1, 2, 3 | |

**`approval` refuses to boot until item 5 lands.** The predicates already
answer correctly and signup already treats an applicant as permitted — which is
right, and is exactly why it cannot ship yet: with nowhere for an application to
go, an operator asking for approval would get open registration on an
installation they believe is gated, and nothing would look wrong. The guard is
in `validateBootstrap` and its test says to remove both when the queue exists.
| 6a. Invite rate limits | ✅ | — | `3f9c978` |
| 6b. `INVITES_FROM` + pending cap | ✅ | 1 | `cd4a520` |
| 7. Agreements checked at every sign-in | ⛔ | real ToS text | |
| 8. Display name becomes optional | ✅ | — | `d6ce390` (react) |
| 9. Onboarding | ⬜ | separate plan | |

Landed alongside, because they stood on their own: configurable agreement URLs
(`4534831`), and the two invite bugs that item 6a's work exposed — sending an
invitation always returned 500, and a permission failure answered 402.

### Order, and why

**~~1~~ → ~~8~~ → ~~2~~ → ~~3~~ → ~~6b~~ → 4 → 5 → 7**

- **1 first** because 2, 5 and 6b all read the mode.
- **8 before 2.** Signup stops collecting a display name in item 2, so the
  render fallback has to exist first — otherwise every account created in
  between renders as "Unknown" with `??` for initials.
- **3 before 5**, so an application reaches the queue already verified and
  administrators are not moderating unverified junk.
- **4 moved ahead of 5.** The honeypot and timing checks want the same form
  item 5 will branch, and doing them first means the approval path inherits
  them rather than needing them retrofitted.
- **4 after 2**, because honeypot and timing need the server-rendered form to
  put them on.
- **7 is blocked**, and not on code: the Terms and Privacy Policy are still
  boilerplate with `[Your Organisation]` in them. Shipping the gate first
  records 822 people as accepting a template.

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

The test is **who the field is for**, not how useful it would be to have.

| | `open` | `invite` | `approval` |
| --- | --- | --- | --- |
| handle | ✅ | ✅ | ✅ |
| email | ✅ | ✅ | ✅ |
| password | ✅ | ✅ | ✅ |
| display name | ❌ | optional | ✅ required |
| bio | ❌ | optional | ✅ required |
| website URL | ❌ | optional | ✅ required |
| pronouns | ❌ | ❌ | ❌ |
| terms acceptance | ❌ — asked at first sign-in | ❌ | ❌ |

**Neither Mastodon nor Bluesky asks for a display name at signup.** Both take
the handle, email and password — Bluesky also a date of birth — and offer the
display name during a skippable onboarding, where an empty one falls back to
the handle.

**In `approval` they are the application, so they are required.** A human is
being asked to decide, and deciding on an email address alone is not deciding.
Mastodon does this with a "why do you want to join?" box; these three do the
same job and have the advantage of being the profile itself rather than a note
discarded after the decision. Nobody rejected has written anything they would
not have written anyway.

**In `invite` they are optional, not required.** The reason `approval` needs
them does not apply — an administrator already decided, so nobody is judging.
Requiring them at the door buys a worse version of what onboarding does a
minute later, on a plain server-rendered form rather than a live editor with an
avatar uploader. A required bio is also how a community ends up full of bios
that say ".". Offered and skippable gets the profiles from people who want to
write them and blocks nobody.

**Pronouns are never collected at signup, in any mode**, and this is the one
field where the answer is not simply "it depends on the mode":

- They fail the test. Name, bio and website help an approver judge whether this
  is a real person who belongs here. Pronouns tell them nothing about that —
  they are not decision-relevant, so in `approval` they are being asked for
  because they are nice to have, which is what onboarding is for.
- Requiring them means asking somebody to disclose something personal **to a
  stranger who is judging their application**, before they have decided to
  join. For a person who is not out that is a real cost, and "leave it blank"
  is not available when the field is required.

Mastodon keeps pronouns in optional profile metadata and Bluesky has no
dedicated field. Offer them prominently in onboarding, where the answer is
given to the community rather than to a gatekeeper.

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
| capping and rate-limiting invites | bounds a compromised or careless account; item 6 ✅ |
| email verification | high, kills volume; item 3 |
| rate limit per IP and per subnet | high; the limiter already exists, signup just has no scope |
| honeypot field + submission timing | free, no user friction, low ceiling |
| proof-of-work (Altcha / mCaptcha shape) | a tax on volume; only worth it if `open` is used |
| disposable-domain blocklists | an MX check gets most of the value without the churn |

In scope here: **rate limiting, honeypot and timing.** All three are a day's
work and need no dependency.

**No CAPTCHA.** Not as a fallback, not as a last resort — it is not on the
list at all.

The obvious reason is that the puzzles are solved: vision models read the
image grids, and the audio challenges went first. The less obvious reason
matters more, because it also rules out the "invisible" ones that are not
puzzles. **The defeat is economic, not perceptual.** A solved CAPTCHA is a
transferable token, so there is a market in them — a few dollars per thousand,
solved by whatever mix of models and people is cheapest that week. Making the
challenge harder raises the price a little and changes nothing structural. What
it reliably does is tax the legitimate: the people who fail CAPTCHAs most are
those on screen readers, older devices, shared or VPN addresses, and anyone
whose first language is not the one the puzzle is written in.

**Proof-of-work is a tax, not a wall, and it should be described that way.** It
is better than a CAPTCHA for two specific reasons: the work is bound to the
challenge so it cannot be solved elsewhere and resold, and it asks the person
signing up for nothing at all. It is not a solution — an attacker with a GPU
buys through it — it just makes volume cost something. That is worth having in
`open` mode and pointless anywhere else.

**Which is the real conclusion of this item.** The measures that actually stop
bots on this platform are the registration modes, and they are already the
design: the default is `closed`, and a curated community running `invite` or
`approval` has no bot problem to solve, because there is no unauthenticated
path to an account. Everything in the table above `honeypot` exists for the
installation that chooses `open` — and an installation that never chooses it
never needs proof-of-work either.

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

### 6. Who may invite, and how many

Invites are administrator-only today. Two settings open that up and bound it.

#### Who

`INVITES_FROM`, either `administrators` (the default, and what happens today,
so no installation changes behaviour on upgrade) or `members`.

**Opening this up changes what an invitation means**, and the approval queue
has to change with it. An administrator's invitation is a decision — they are
the person who would otherwise clear the queue. An ordinary member's is a
recommendation: they are vouching, not admitting.

So the rule is not "invites skip the queue", it is **"an invitation skips the
queue when an administrator issued it"**:

| mode | admin invites | member invites |
| --- | --- | --- |
| `open` | straight in | straight in |
| `approval` | straight in | **into the queue, naming the voucher** |
| `invite` | straight in | straight in |
| `closed` | refused | refused |

A member-issued invitation landing in the queue with the inviter's name
attached is a better signal than a cold application, and it is the reason to
allow member invites at all — the member does the finding, the administrator
keeps the decision.

#### How many

**One cap: outstanding invitations.** `INVITE_PENDING_MAX`, how many
unaccepted invitations a person may hold at once. Default 5 for members,
`INVITE_PENDING_MAX_ADMIN` defaulting high for administrators — somebody
onboarding a department should not hit a limit set for members, but not
unlimited either, since the cap is also what stops a compromised administrator
account turning the mail service into a relay.

It **self-heals**. A slot frees when an invitation is accepted, expires, or is
revoked, so an honest member is never permanently stuck and nobody has to
administer a reset.

**Deliberately no lifetime cap.** It was considered and dropped:

- It is not a safety control. The pending cap bounds how much damage one
  account can do at any moment, and the rate limit below bounds how fast — a
  lifetime total adds nothing to either.
- It is a rationing mechanic — the Gmail-beta, early-Bluesky device for making
  growth deliberate. That is a product decision about how fast a community
  should grow, not an abuse control, and both of those examples dropped it once
  they opened up.
- It is the only one of the three with **no self-service remedy**. A member who
  has brought in twenty good people over two years hits a wall and has to ask
  an administrator, who has no obvious basis for saying yes or no.

It also takes a rule with it: with a lifetime count, revoking an invitation
could not refund it, or the cap would be bypassed by sending, revoking and
resending forever. With only a pending cap, revoking simply frees a slot and
there is nothing to reason about.

If rationing is ever wanted, it is a separate feature with a quota that
administrators can grant — not a constant in the configuration.

#### Rate limits are a separate thing, and also needed

A cap bounds the **stock** of outstanding invitations; a rate limit bounds the
**flow**. Somebody with a pending cap of 5 can still send five, have them
accepted, and send five more, all within a minute — which is exactly the shape
of a mail flood, and the pending cap does not touch it. Add an invite scope to
the limiter that already exists.

This is the pairing that makes a lifetime cap unnecessary rather than merely
unpopular: stock and flow are both bounded, and a lifetime total is a third
thing that constrains neither.

#### What this needs from the repository

`tokens.Count(tokenType, req)` **takes a request and ignores it** — it counts
every token of that type, unscoped to anybody. Any caller passing a filter
gets a number that answers a different question, silently. Counting one
person's pending invitations needs a real method, and that signature should
stop lying while the file is open.

Pending is computable from what is already there: type `invite`, `created_by`
the person, not used, not past `start_time + expired_after`.

#### The client needs to know

The web app has to decide whether to show "Invite someone" at all, and whether
to say "you have 2 of 5 left". That is instance configuration plus a per-viewer
count, and it belongs with the surface item 1 introduces rather than as another
one-off endpoint.

### 7. Agreements checked at every sign-in — in the web app

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

### 8. Display name becomes optional

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

### 9. Onboarding — separate plan

Post-login, skippable, and **not a gate**. Avatar, display name, bio, website,
pronouns, topics.

**Pronouns belong here rather than at signup**, and the component is already
built: `SelectPronouns` takes its options from the language files rather than a
constant, so each locale ships the set that makes sense in its own language and
one with nothing to offer ships none. "Unset" is a real option and the default,
which is exactly the property that makes it wrong to require at signup and fine
to offer here — the answer is given to the community, not to a gatekeeper
deciding whether to admit you.

The same fields `approval` mode required, `invite` mode offered and `open` mode
never asked for all converge here. Somebody who filled them in at the door
arrives with them pre-filled and skips ahead; somebody who did not is asked
once, kindly, and may decline.

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
- **Onboarding** — item 9.

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
- `INVITES_FROM=members`, mode `approval` — a member's invitee lands in the
  queue naming the voucher, while an administrator's goes straight in. Testing
  only one of those leaves the whole distinction untested.
- Send invitations up to `INVITE_PENDING_MAX`; the next is refused with a
  message naming the cap. Revoke one and the next succeeds. Let one expire and
  the same. Accept one and the same.
- `INVITES_FROM=administrators` — a registered member's attempt is refused by
  the server, not merely hidden in the client.
- A person with no acceptance on record is bounced to the interstitial after
  login, and the row afterwards carries the version, the timestamp and a real
  IP and user agent.
- Bump `TOS_VERSION`: the same person is asked again.
- A person with no display name renders as their handle everywhere, with
  initials from the handle rather than `??`.
