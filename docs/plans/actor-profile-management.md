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

---

## 1. Deletion — the two phases never built

Both were in the approved plan and neither is started.

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

---

## 2. Archive mode for non-person actors

Requested. A group, event or other non-person actor can be **archived**: it and
everything it owns drop out of feeds and search, but nothing is destroyed. The
state for something that is finished rather than regretted — an event that
happened, a group that wound down.

**Archiving is permanent.** That is the deciding property, and it puts archive
next to deletion rather than next to disabling:

| | resolves publicly | in feeds/search | accepts new content | reversible |
|---|---|---|---|---|
| enabled / disabled | yes / no | yes / no | yes / no | **yes**, a toggle |
| archived | yes, read-only | **no** | no | **no** |
| deleted | no | no | no | for 30 days, then no |

So the three are not a scale of severity. Disabling is a reversible switch;
archiving and deleting are both one-way doors, and the choice between them is
*what happens to the content* — archive keeps it readable forever, delete
destroys it. Neither is the safe option relative to the other.

### The cascade is cheaper than it looks

`medium.OwnerID` is the actor a medium is posted **to**, not its author —
`created_by` holds the author. So a group owns its posts, photos and topics
outright, and `GetBaseSelectQuery` already carries
`LEFT JOIN nodes AS owner ON node.owner_id = owner.id`.

That makes the filter a single predicate on the existing join —
`owner.archived_at IS NULL` — with no flag to propagate to owned nodes and
nothing to backfill or keep in sync on unarchive. Prefer that to denormalising a
flag onto every child.

### To decide

- **Where the filter belongs.** Feeds and search each build their own queries;
  adding the predicate in `GetBaseSelectQuery` catches everything at once but
  also catches the paths that *should* still see archived content — the group's
  own profile, and the admin views.
- **How much ceremony archiving needs.** This is the question permanence forces.
  We spent this session giving deletion a 30-day window precisely because
  one-way-by-misclick is unacceptable for a shared space — and an archive with
  no undo has the same shape. At minimum it wants what deletion has: a statement
  of what changes, a typed confirmation, and a step-up. Worth deciding whether
  it also wants a grace period of its own, or whether a super-administrator
  unarchive is the escape hatch for the misclick case — section 3 already adds
  super-admin-only recovery tooling for exactly this class of problem.
- **Who can archive.** Given it cannot be undone by whoever does it, this is
  probably not an ordinary group-admin action on the same footing as editing a
  description.
- **Whether archiving is offered beside deleting.** It is a genuine choice —
  keep the content readable forever, or destroy it — so presenting them together
  helps somebody pick correctly. It is *not* a softer alternative, and the copy
  must not imply that it is.
- **Federation.** An archived actor probably still serves its ActivityPub actor
  object, read-only — unlike a deleted one, which should eventually answer
  `410 Gone` (section 6).
- **Members.** Archiving a group with active members changes what they can do
  without removing anything. They should be told.

### Work **[services]** + **[react]**

`nodes.archived_at` plus a guarded migration and the legacy `upgrade.sql` entry;
an archive action on the actor gRPC service and the group route; the read
filter; a card in settings carrying the same weight as the delete card, since
the action is equally final.

Archiving should write to `node_deletion_logs` — the `action` column is a
varchar precisely so a new action costs nothing. That does make the table's
name narrower than its contents, so decide then whether it becomes
`node_lifecycle_logs`. Cheap to rename while both are still unshipped; not
cheap later.

---

## 3. Hard delete for super administrators

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
  (section 4).
- **Super administrator only.** `IsSuperAdmin`, not `IsAdminOrSuperAdmin`. The
  2FA-disable widening to ordinary admins was right because it recovers an
  account; this destroys one.
- **Gate it properly.** Step-up, and a typed alias. There is no undo and no
  grace period — this is the one action in the system where the ceremony should
  be heavier than for ordinary deletion, not lighter.
- **Two variants to decide between.** "Purge now" keeps the tombstone, so the
  handle stays reserved. "Purge and release" scrubs the tombstone too and frees
  the handle, which is what an erasure request may require and what section 6
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

## 4. Correctness gaps from the audit

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

## 5. Adjacent things noticed along the way

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

## 6. Federation groundwork

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

## 7. The larger refinement: profile management

Deletion is one corner of it. Worth treating together:

**A proper list of links.** The reason the metadata fields came out. Arbitrary
labelled URLs rather than three fixed slots, with verification of the kind
Mastodon does (a `rel="me"` back-link proving ownership) as an obvious follow-on.
Needs somewhere to live in the data model — `information` / `meta` already flow
through reads untouched.

**`enabled` is doing three jobs, and archive makes four.** Administrator
suspension, deletion and "unpublished" already collapse into one flag, which is
why deleted and disabled actors are indistinguishable in browse. Archive
(section 2) is a fourth state that must not join them — hence `archived_at` as
its own column rather than another meaning for `enabled`.

Worth doing the separation properly while adding archive rather than after:
suspension is a reversible toggle, archival and deletion are one-way doors, and
one boolean cannot hold three answers to three different problems. Explicit
timestamps per state — `archived_at`, `deleted_at`, with `enabled` left as the
reversible switch it already is — would also give the moderation surface below
something coherent to filter on.

**No moderation surface.** There is no way for an administrator to see suspended
or deleted actors, or to act on them in bulk. Related to the browse-filter gap
and to admin tiering; probably one screen serves all three.

**Group membership is invisible at deletion time.** A group's members are not
told it is scheduled, are not told when it is purged, and cannot see it is on the
way out. The notification gap above is the immediate fix; the broader question is
whether a group with active members should be deletable by one administrator at
all.
