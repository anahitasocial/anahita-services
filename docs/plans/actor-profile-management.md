# Actor profile management — open work

Written 2026-09-04, at the end of the session that rebuilt profile deletion.

Spans two repositories. Items are marked **[services]** or **[react]**; a few
need both.

---

## Where things stand

Profile deletion was rebuilt from a hard delete into a reversible one:

- Deleting marks `nodes.deleted_at` and clears `enabled`, destroying nothing.
- auth-service revokes credentials on the deletion event — passkeys, refresh
  tokens, IdP sessions — which previously outlived the account entirely.
- A person restores themselves by signing in; a group's administrators restore
  it from the group's settings page.
- A sweeper job purges after 30 days, keeping a tombstone so the handle stays
  reserved (see *Federation* below).
- Deletion is audited in `node_deletion_logs`: scheduled, purged, restored.
  Keyed on a node rather than an actor, so media and comments can write to it
  when they need the same record — actors are just the first user.

What follows is what was deliberately left, plus what an end-of-session audit
turned up.

**Disable and archive have moved** to `actor-lifecycle-states.md`. Both land
in the same Danger zone and both hinge on the same question — what `enabled`
is allowed to mean — so they are designed together there rather than as one
more section here.

---

## 1. The phases never built

Phases 2 and 3 continue the approved deletion plan and neither is started.
Phase 4 opens a new front, on the flow next door.

### Phase 2 — self-service friction **[react]** + **[services]**

The Danger zone currently states the consequences and takes a typed alias. The
plan added two more gates:

- **Consequence counts.** Name what is actually lost — posts, comments,
  followers, memberships, member since — from a new count endpoint, rather than
  describing it in the abstract. This is the piece that makes people stop.
- **Step-up before deleting.** `/reauth/*` and the `StepUp` container already
  exist and are used by the password, email, username and 2FA-disable flows.
  Deletion is the only credential-grade action still gated on nothing but a
  session — a stolen unlocked laptop currently deletes a profile.
- **Email confirmation link**, modelled on `email_change.go`. Was chosen at
  planning time; worth revisiting now that a 30-day window already makes the
  action reversible.

### Phase 3 — admin tiering **[react]** + **[services]**

The same card serves self-deletion and an administrator deleting somebody else
(`sections.js` has `viewerOnly: false` on the Danger item), with identical
ceremony for both.

- Server classifies the target as `empty` / `dormant` / `active` from real
  signals and returns the tier. The admin cannot talk themselves into the fast
  path — the data decides.
- `empty` / `dormant`: one confirmation plus step-up. `active`: the full
  ceremony.
- **Sole-administrator guard.** Deleting a person who solely administers a group
  silently orphans it — the admin edge goes with the purge and nothing checks.
  No count query exists; `ActorAdminRepoInterface` exposes only Browse/Add/
  Delete.


### Phase 4 — password reset **[services]**

Reviewed 2026-09-04. **The handler logic is sound and is now covered by
tests; the delivery underneath it is not.** Two findings, either of which is
enough on its own to make the flow look broken to somebody using it, and
neither of which produces an error anybody sees — every notifier is
fire-and-forget and only logs.

**The SMTP provider cannot send any notification at all.**
`mail-grpc-service/providers/smtp.go` implements `SendFromTemplate` as
`return "", errors.New("not implemented for smtp mailer")`. Every notifier
in auth-service — reset, email change, username change, 2FA, invite,
deletion — goes through `SendFromTemplate`. So with `MAILER_PROVIDER=smtp`
nothing is delivered, the person is shown "check your email", and the log
line goes past unread. This is the first thing to check.

**The Mailgun path depends on a template stored at Mailgun.** The notifiers
pass `"message_action"`, which the provider hands to `msg.SetTemplate` — it
is a Mailgun-hosted template, not a file in this repository, so nothing here
can tell you whether it exists on the sending domain. If it does not, the
send is rejected. The same applies to every other template name the
notifiers use.

Then, in rough order of how likely each is to bite:

- **`start_time` and `created_at` come from different clocks.**
  `tokens.Add` does not name `start_time`, so MySQL fills it from
  `CURRENT_TIMESTAMP` in the *session* time zone, while `created_at` is
  written from Go. The DSN sets `parseTime=true` with no `loc`, so the
  driver reads everything back as UTC. If MySQL is not on UTC, every reset
  link's fifteen-minute window is shifted by the offset — either dead on
  arrival or alive well past its expiry. Confirm with
  `SELECT @@session.time_zone` before assuming it is fine, and consider
  writing `start_time` explicitly rather than relying on the default.
- **A reset can be issued for a deleted or suspended account.**
  `Users.ReadByEmail` filters on neither `deleted_at` nor `enabled`, so a
  profile inside its 30-day deletion window still receives reset mail and
  can complete a reset. Harmless in that login still refuses it — the
  restore flow is what such a person actually needs — but mailing a deleted
  account is its own problem, and the reset page says nothing about the
  deletion.
- **One validation message covers two different failures.** A `POST
  /password/reset` that fails validation always reports "Password must be at
  least 15 characters long", including when the real failure was the token
  field. Rare, and baffling when it happens.
- **`GetAuthURL()` returns `http://localhost` outside prod and staging**, so
  reset links in a development environment point at the developer's own
  machine. Correct for local work, wrong for any shared dev deployment.

**What the tests now cover**, so the next change to this flow has something
holding it: enumeration parity between registered and unregistered
addresses; token type, recipient, 15-minute expiry and 128-character body;
that the mail carries a link containing the token; both forgot throttles;
every rejection path on the reset link (missing, unknown, wrong type, used,
expired); that a successful reset rehashes the password, consumes the link,
and revokes passkeys, refresh tokens and IdP sessions; that reusing the
current password changes nothing and does *not* consume the link; that the
probe limiter bites and that ordinary form errors do not spend its budget;
and that a reset never establishes a session. The authenticated change and
the step-up markers are covered alongside them.

Left uncovered, and worth adding when each is next touched: the repositories
themselves, which need a database rather than a double; and the notifier
bodies, which are strings.

---

## 2. Hard delete for super administrators

Requested. A cleanup path that skips the 30-day window and purges immediately —
for spam, abuse, and test accounts where waiting a month serves nobody, and as
the mechanism behind a verified erasure request.

Everything needed already exists in the sweeper: `PurgeActorContent`,
`TombstoneActor`, the blob deletion, the audit row and the `purged` event. The
work is mostly making that reachable from a request rather than only from a
schedule.

- **Extract the purge.** `purgeAccount` is unexported in `graph/sweeper`. Lift it
  so both the job and an endpoint call one implementation — two purge paths that
  drift is exactly the trap the two restore implementations are already in
  (section 3).
- **Super administrator only.** `IsSuperAdmin`, not `IsAdminOrSuperAdmin`. The
  2FA-disable widening to ordinary admins was right because it recovers an
  account; this destroys one.
- **Gate it properly.** Step-up, and a typed alias. There is no undo and no
  grace period — this is the one action in the system where the ceremony should
  be heavier than for ordinary deletion, not lighter.
- **Two variants to decide between.** "Purge now" keeps the tombstone, so the
  handle stays reserved. "Purge and release" scrubs the tombstone too and frees
  the handle, which is what an erasure request may require and what section 5
  describes. They are different actions with different consequences; do not
  collapse them into one button.
- **Audit is non-negotiable here.** A super administrator destroying an account
  with no waiting period is precisely the event the log exists for. Record which
  variant was used.

### Work **[services]** + **[react]**

An exported purge in `graph/sweeper`; an actor gRPC `Purge` RPC (the proto
regenerates cleanly — `protoc` and `protoc-gen-go` are installed at the versions
that produced the committed files); routes in person-service and group-service;
a super-admin-only card in the Danger zone.

---

## 3. Correctness gaps from the audit

### Soft-deleted actors leak into the "disabled" filter **[services]**

Nothing filters on `deleted_at` in browse or search. Exclusion relies entirely
on `enabled = 0`, and the actors browse UI has a `disabled` filter — so an
administrator using it sees deleted profiles mixed in with ones they disabled
deliberately, indistinguishable from each other.

Either filter `deleted_at IS NULL` in the browse queries, or make the distinction
visible. The second is probably more useful: a "recently deleted" view is also
where an administrator would go to restore something.

### Group deletion notifies nobody **[services]**

group-service publishes no event and sends no mail. A group's other
administrators get no word that it is scheduled — only the audit row records it.
People get a notification through the auth-service consumer; groups have no
equivalent, and they are the case where somebody *else's* decision destroys
*your* content.

### No administrator undo for a person **[services]** + **[react]**

A person restores themselves at login, so an administrator who deletes the wrong
person cannot reverse it — the person has to sign in, and may not know to.
Recoverable, but not by whoever made the mistake.

The restore branch in `actors/Forms/Delete.jsx` is currently gated to groups
because of this. Adding `POST /people/:id/restore` mirroring the group route
would let the branch open up for both.

### Deleting a person does not remove their posts in groups **[services]** + **[react]**

`PurgeActorContent` removes what the actor **owns** — and a medium's owner is the
actor it was posted *to*, not its author. So deleting a person removes what is on
their own profile, while everything they posted into a group stays, authored by a
tombstone.

That may well be the right behaviour: the group keeps its content, and one
member leaving does not gut a shared space. But it is not what the Danger zone
tells them. The card currently says deletion removes "posts, comments, photos,
followers and group memberships", which promises more than it does.

Decide which is true and make them agree — either scope the purge by
`created_by` as well as `owner_id`, or correct the copy to say that content
posted into groups remains. This also feeds the consequence counts in Phase 2,
which have to count the same set the purge will actually take.

### Two restore implementations **[services]**

`Users.Restore` (raw SQL in auth-service, for the login path) and
`RestoreActor` (graph-grpc, for the group path) do the same thing to the same
row. They agree today; nothing keeps them agreeing.

---

## 4. Adjacent things noticed along the way

**`entrypoint.sh` never records the initial schema** **[services]** — it runs
`schema/init.sql` on an empty database and exits without writing
`goose_db_version`. The next start of that same database finds tables, finds no
legacy tables, and runs every migration from zero against a schema that already
has them. Every migration added so far is written with an information-schema
guard to survive this; the entrypoint is the actual fix.

**`ActorNodeItem.Enabled` is never populated** **[services]** — declared in the
response, absent from `MapNodeItem`, so `enabled` has never appeared in the API.
Clients cannot tell an enabled actor from a disabled one.

**OTP scaffolding is committed but unwired** **[services]** — `TokenTypeLoginOTP`,
`Token.Code`, `CodeAttempts`, `GenerateCode`, `IsCodeValid`, `HasAttemptsLeft`
all exist. The `code` and `code_attempts` columns exist in neither the schema
nor the tokens repository, exactly as `new_email` did before it was wired up.

**Profile URLs break on hyphenated usernames** **[react]** —
`containers/actors/Settings/index.jsx` does `params.id.split('-')[0]`, but the
username validator allows hyphens (`^[a-zA-Z0-9\-_]+$`). A person called
`john-doe` resolves as `john`.

**Uneditable metadata is still displayed** **[react]** — the website, contact and
phone fields were removed from the profile forms because no server request ever
accepted them, but `actors/Read/About.jsx` still renders them from
`information`. Legacy profiles migrated from the PHP app may hold real values
there that nobody can now change or remove. Decide with the list-of-links
feature below.

**Two orphaned files** **[react]** — `containers/actors/Settings/Features.jsx`
(panel exists, no tab in either namespace, never imported) and
`containers/auth/Totp/Widget.jsx` (repaired but imported nowhere).

---

## 5. Federation groundwork

Deliberate decisions already taken, with the implementing work still to do.

**Tombstones exist; `410 Gone` does not** **[services]**. Purging keeps a minimal
identity row so the handle stays reserved — which ActivityPub effectively
requires, since remote servers cache actor objects against a permanent URI and a
recycled handle inherits the previous person's followers and mentions. The other
half is that the actor URI should keep resolving and answer `410 Gone`, so remote
instances stop retrying and drop their cache. Nothing serves that yet.

**Scrubbing a tombstone** **[services]**. A verified erasure request may have to
reach the handle itself. That wants a deliberate administrator action which
accepts that the handle becomes reusable — not a default.

---

## 6. The larger refinement: profile management

Deletion is one corner of it. Worth treating together:

**A proper list of links.** The reason the metadata fields came out. Arbitrary
labelled URLs rather than three fixed slots, with verification of the kind
Mastodon does (a `rel="me"` back-link proving ownership) as an obvious follow-on.
Needs somewhere to live in the data model — `information` / `meta` already flow
through reads untouched.

**`enabled` is doing three jobs.** Administrator suspension, deletion and
"unpublished" all collapse into one flag, which is why deleted and disabled
actors are indistinguishable in browse. Separating them properly is part of
`actor-lifecycle-states.md`, because archive would otherwise become a fourth
meaning for the same boolean.

**No moderation surface.** There is no way for an administrator to see suspended
or deleted actors, or to act on them in bulk. Related to the browse-filter gap
and to admin tiering; probably one screen serves all three.

**Group membership is invisible at deletion time.** A group's members are not
told it is scheduled, are not told when it is purged, and cannot see it is on the
way out. The notification gap above is the immediate fix; the broader question is
whether a group with active members should be deletable by one administrator at
all.
