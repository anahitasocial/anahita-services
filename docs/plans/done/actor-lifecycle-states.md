# Actor lifecycle states — disable and archive

> **Status, 2026-09-28: done.** `archived_at`, archive, enable and disable routes, audit rows and
> events exist for all four transitions, and the web Danger zone offers them. Archiving media
> (see *Scope*) remains deferred. Operator documentation is planned in
> [actor-hard-delete.md](../actor-hard-delete.md) as `docs/account-lifecycle.md`.

Written 2026-09-04, split out of `actor-profile-management.md` so the two
states can be designed together rather than one at a time.

Both land in the Danger zone alongside deletion, and **both apply to every
actor** — people included. Archive was originally scoped to non-person actors;
that was reversed on 2026-09-06 once the case for it was the one that matters
most, somebody leaving the network or dying. See *Decisions taken*.

Spans two repositories. Items are marked **[services]** or **[react]**.

---

## Decisions taken — 2026-09-06

Confirmed when this became Phase 5. These settle most of the "To decide"
list further down; what they do not settle is called out there.

**The three states, restated by the person who has to live with them:**
archive is not reversible, enable/disable is reversible, delete is permanent.
That matches the table below, and it is worth noting that it was arrived at
independently — the ordering is not an artefact of how the table was drawn.

**All three go in the Danger zone.** Including enable/disable, which is
reversible. It sits there not because it is severe but because it is where
somebody looks for it, and grouping the reversible one beside the two
one-way doors is also what makes the difference between them legible.

**Hard delete is super-administrator only, and its purpose is site
cleanup** — removing outdated accounts, not moderating live ones. That
purpose is worth writing down because it decides the surface: a tool for
clearing out stale accounts wants a list and a filter, not a button on one
profile. It is not an escalation of the ordinary delete and should not be
reachable from the same place.

### Archive applies to people too — settled 2026-09-06

Confirmed, with the case that settles it: **somebody leaves the network, or
dies.** Deleting their profile is the wrong answer to both. Archive is what
lets the account stop being live without the content stopping existing.

That reverses the earlier "archive for non-person actors only", and it is the
better call — the person case is arguably the *stronger* one, because a group
that is finished can simply go quiet, while a person's account keeps looking
active until something says otherwise.

**An archived person cannot sign in.** The two entry points below make this
unavoidable: one of them is used when the account holder is dead. An archived
account that still authenticates is an account somebody could be signed into,
which is precisely what archiving is meant to end.

#### Two entry points, one state

They need different ceremony and different copy, and conflating them would get
one of the two badly wrong.

- **Self-service** — "I am leaving." The person archives their own profile.
  Step-up, typed confirmation, and copy that says plainly it cannot be undone
  and that they will not be able to sign in again.
- **Administrative** — memorialisation. The account holder cannot do it; that
  is the whole situation. A super administrator archives on request from
  family or an executor. This is not an escalation of the self-service path
  and should not share its button.

The administrative path is the one that does not exist for groups, and it is
the reason archive-for-people is not just "the group flow with a different
type filter".

---

## What archived content should do — the visibility question

The question as posed was: hide archived material behind a search checkbox, or
show it only to administrators? **Neither**, and the second is worth arguing
against explicitly.

### The industry splits two ways

| | what happens to the content | discoverable? |
|---|---|---|
| Facebook, Instagram, LinkedIn — *memorialisation* | preserved, visible to its original audience | pulled from suggestions, recommendations, ads, birthday reminders; reachable by direct link |
| GitHub, most forums — *archive* | preserved, read-only, prominent badge | fully visible and fully searchable |

The two agree on the part that matters — **nothing is hidden, and nothing is
destroyed** — and differ only on discovery. Neither takes the content away
from the people who came looking for it.

### The rule to use: push always stops; search depends on the type

Revised 2026-09-06 after the actual use case surfaced, and it is worth stating
what changed because the first version of this section got it half wrong.

**Two requirements are sharing one word, and they want opposite things.**

- **Memorial.** Somebody died. Their profile must stay findable — a memorial
  nobody can find is not a memorial, and this is the case archive-for-people
  exists to serve.
- **Legacy cleanup.** The PHP-era Anahita groups. Preserved, but they should
  stop competing with live spaces in search. That is the entire point of
  archiving them.

That is why this question kept reversing. It was not indecision; it was two
requirements.

#### Privacy does not solve the cleanup case

The reasonable-sounding version of "we need no search flag" is: search already
applies the privacy gates, so restricted actors and their content are filtered
out already. The gates are real — `SetSearchAccessQuery` has an owner gate and
a node gate, both checked.

But the content this is FOR is public. Measured against the development
database:

| | |
|---|---|
| media owned by the four legacy PHP groups | **2,228** of 8,657, or 26% of everything |
| access on those groups | `public` — Anahita Project (897), Anahita Atrium (672), Tribe Support (583), Photos App (76) |
| media visible to a logged-out visitor | 7,972 of 8,657 |

Every one of those 2,228 is public, so no privacy gate touches it. Archiving
without a search rule would preserve the content and change nothing about the
problem it was meant to fix.

#### The rule

- **Push always stops, for everything archived.** Feeds, suggestions, "who to
  follow", notifications — nothing archived is delivered to somebody who did
  not ask for it. This half is not type-dependent and not in question.
- **Pull always works.** Direct URLs resolve forever, for everything archived.
  This is the point of archiving rather than deleting.
- **Search depends on the node type**, which the database already carries, so
  this costs no new flag:
  - an archived **person** stays in search results, marked. Identity is
    something people look for by name.
  - an archived **group** and the media it owns drop out. Finished content
    should not compete with live spaces.

The implementation cost of the group half is one predicate. `medium.OwnerID`
is the actor a medium is posted to, and `GetBaseSelectQuery` already carries
`LEFT JOIN nodes AS owner ON node.owner_id = owner.id`, so it is
`owner.archived_at IS NULL` in the search query and nothing else — no flag to
propagate to 2,228 rows and nothing to keep in sync.

#### The escape hatch

An **"include archived"** toggle on search, off by default. That is the
opposite of the filter first proposed here, and it is the right way round: the
default should be the common case, and the archive is a place you go looking
deliberately.

### Against admin-only

It fails the case that motivated the feature. A dead person's profile visible
only to administrators is invisible to their friends — the people the memorial
is for. It also makes archive strictly worse than deletion from the account
holder's point of view, which means nobody would choose it, which means the
feature does not do its job.

### A badge is not optional

Facebook's "Remembering" prefix is the most transferable part of their design
and the cheapest thing here. An archived profile with no visible marker is
actively distressing: people message someone who has died and wonder why there
is no reply. Whatever the wording, the state must be legible on the profile
itself, not only in an admin view.

### Still open

- **The exact wording.** "Archived" is right for a group that finished and
  wrong for a person who died. These may need to be two labels over one state.
- ~~A filter to exclude archived results~~ — settled above: archived groups
  are excluded by default, and the toggle is "include archived".
- **Who may archive a person.** Self, and super administrators. Whether
  ordinary administrators can is a moderation-policy question.

---

## Archive is not a tombstone

Worth stating plainly, because both are end states that leave something behind
and the two are easy to run together. They are opposites.

| | the content | the profile | ActivityPub |
|---|---|---|---|
| **archived** | preserved, readable at its URLs forever | resolves, read-only, marked | `200` with the actor and its content |
| **tombstoned** | destroyed — posts, comments, edges, all of it | a four-field stub: id, alias, type, `deleted_at` | `410 Gone` with a `Tombstone` |

Archiving the Anahita Project group keeps its 897 posts readable. Hard-deleting
it destroys those 897 posts and leaves a marker saying the handle is taken and
whatever held it is gone.

So a tombstone is not a gentler deletion or a stricter archive — it is **what
remains after destruction**, and its only job is to stop the identifier being
reissued to somebody else. `TombstoneActor` strips the name, body, excerpt,
avatar, cover, geo fields, meta, website and every counter; `PurgeActorContent`
removes everything the actor owned. Nothing readable survives either.

ActivityStreams has no archive concept at all, and its `Tombstone` type is
explicitly "a content object that has been deleted". The archived state maps to
an ordinary actor that simply never emits anything again — see *Federation*
below.

---

## The three states are not a scale of severity

This is the thing to get right before any of it is built, because the UI
will imply an ordering whether or not one is intended.

| | resolves publicly | in feeds/search | accepts new content | reversible |
|---|---|---|---|---|
| enabled / disabled | yes / no | yes / no | yes / no | **yes**, a toggle |
| archived | yes, read-only | **no** | no | **no** |
| deleted | no | no | no | for 30 days, then no |

Disabling is a reversible switch. Archiving and deleting are both one-way
doors, and the choice between them is *what happens to the content* —
archive keeps it readable forever, delete destroys it.

So archive does not belong between disable and delete on a severity slider.
It belongs next to delete, and the copy must never present it as the softer
option: somebody reaching for "archive" because it sounds gentler than
"delete" has still done something they cannot undo.

---

## `enabled` is already doing three jobs

Administrator suspension, deletion and "unpublished" all collapse into the
one boolean today. That is why a soft-deleted actor is indistinguishable
from a deliberately suspended one in the browse UI's `disabled` filter — the
correctness gap recorded in `actor-profile-management.md`.

Archive would be a fourth meaning, and must not join them. Hence
`archived_at` as its own column, matching `deleted_at`:

- `enabled` — the reversible switch, and nothing else.
- `archived_at` — permanent, content stays readable.
- `deleted_at` — the 30-day countdown to erasure.

Explicit timestamps per state are also what would give a moderation surface
something coherent to filter on. Worth doing the separation properly while
adding archive rather than after, because every query written against the
overloaded flag in the meantime is one more to unpick.

---

## Disable — for every actor

Mostly already built, and mostly a matter of stopping `enabled` from meaning
four things at once.

`nodes.enabled` is enforced at all three login paths, so a disabled person
cannot sign in today. What is missing is everything around it:

- **It is not exposed.** `ActorNodeItem.Enabled` is declared in the response
  and never populated by `MapNodeItem`, so `enabled` has never appeared in
  the API at all — no client can tell an enabled actor from a disabled one.
  **[services]**
- **There is no deliberate control for it.** Disabling happens as a side
  effect of deletion. A Danger zone card that disables and re-enables is the
  actual feature. **[react]**
- **Nobody is told.** A disabled group's members, or a disabled person, get
  no notification and no explanation on the profile. **[services]**
- **It is not audited.** See the log section below. **[services]**

Because it is reversible, disable wants materially less ceremony than the
other two: a confirmation, not a typed alias. The friction should track how
hard something is to undo, and this one is a toggle.

---

## Archive — for every actor

A group, event or album that is *finished* rather than regretted — or a person
who has left, or died. It and everything it owns drop out of feeds and
suggestions, nothing is destroyed, and the content stays readable and findable
at its own URLs forever.

See the visibility rule above for what "drops out" means precisely — push
surfaces, not search.

### Derive from the owner; do not copy onto the children

The question is whether archiving an actor should write `archived_at` onto
every medium it owns, or whether reads should keep referring to the owner.

**Derive.** And the reason is not performance — it is that copying destroys
information you need in order to undo it.

#### Privacy already settled this, and 532 rows depend on it

Access works exactly this way today. `SetSearchAccessQuery` builds an owner
gate and a node gate and ANDs them:

```go
sb = sb.Where(ownerGate).Where(nodeGate)
```

So the effective visibility is the INTERSECTION: a post marked `public` inside
a `registered` group is not visible to a logged-out visitor, because the owner
gate fails. Nothing rewrote the post's own access to say so.

That is not hypothetical. Measured against the development database:

| | |
|---|---|
| media marked `public` whose owner is more restrictive | **532** |
| — in `registered` groups | 442 |
| — in `admins` groups | 86 |
| — in `followers` / `mutuals` groups | 4 |
| media a logged-out visitor would see on the node gate alone | 8,060 |
| media they actually see, both gates applied | 7,528 |

The owner gate is the only thing keeping those 532 out.

#### Why copying is the lossy option

Copy the owner's value onto the children and you merge two different facts
into one column: *this was set here* and *this was inherited*. Nothing can
tell them apart afterwards, and both operations need to.

- **Privacy.** Reopen a `registered` group to `public`. Which of its 442
  public-marked posts were deliberately restricted on their own merits? The
  answer was in the column you overwrote.
- **Archive.** Unarchive a group. Which of its media had been archived
  individually, before the group ever was? Same loss. The unarchive either
  resurrects posts that were meant to stay archived, or leaves archived ones
  that should have come back — and there is no way to pick.

Deriving keeps the two facts in two places, which is where they belong: the
child column means "set on this node", the owner column means "set on the
container", and the read takes the intersection.

#### The read cost is already paid

`GetBaseSelectQuery` already carries
`LEFT JOIN nodes AS owner ON node.owner_id = owner.id`, for the owner gate
above and for hydrating the owner into responses. So the archive filter is one
more predicate on a row that is already being fetched, by primary key:

```sql
owner.archived_at IS NULL
```

There is a subtlety in that LEFT JOIN worth keeping: the predicate is TRUE
both when the owner is not archived AND when there is no owner row at all.
That is the correct reading — a node with no owner is not owned by an archived
actor — and it comes free. A denormalised flag would need "no owner" handled
separately.

#### The write cost of the alternative

Archiving Anahita Project would write 897 rows, Anahita Atrium 672, and the
four legacy groups 2,228 between them. Each has to be transactional with the
actor's own update or the two drift, and every one of those writes is a fact
that can now disagree with its source.

#### When denormalising WOULD be right

Worth naming so this is a decision and not a reflex: if a hot path ever needs
to filter on archived-ness without joining the owner — a covering index, say —
then materialise it. As a measured optimisation, with the owner still the
source of truth and the copy understood as a cache.

Federation makes that less attractive rather than more: a remote owner may not
be a local row at all, so a derived read has somewhere sensible to fail while
a copied flag has nothing to copy from.

### To decide

- **Where the filter belongs.** Feeds and search each build their own
  queries; adding the predicate in `GetBaseSelectQuery` catches everything
  at once, but also catches the paths that *should* still see archived
  content — the actor's own profile, and the admin views.
- **How much ceremony.** Permanence forces this question, and it is now
  confirmed permanent rather than assumed. Deletion got a
  30-day window precisely because one-way-by-misclick is unacceptable in a
  shared space, and an archive with no undo has the same shape. At minimum
  it wants what deletion has: a statement of what changes, a typed
  confirmation, and a step-up. Decide whether it also wants a grace period
  of its own, or whether super-administrator unarchive is the escape hatch —
  the hard-delete section of `actor-profile-management.md` already adds
  super-admin-only recovery tooling for this class of problem.
- **Who can archive.** Given it cannot be undone by whoever does it, this is
  probably not an ordinary group-admin action on the same footing as editing
  a description.
- **Whether it is offered beside deleting.** It is a genuine choice — keep
  the content readable forever, or destroy it — so presenting them together
  helps somebody pick correctly. See the warning about copy above.
- ~~**Federation.**~~ Settled as far as it needs to be — see *Federation*
  above. An archived actor serves its actor object read-only; a deleted one
  answers `410 Gone`.
- **Members.** Archiving a group with active members changes what they can
  do without removing anything. They should be told.

---

## Federation — what to keep in mind now

Not being built. ATProto and ActivityPub services are intended eventually,
so the point of this section is only to avoid decisions that would be
expensive to unpick — not to design for either protocol today.

**The one that matters here: keep the states in separate columns.** This is
already the recommendation above, on ordinary correctness grounds, and
federation is a second and independent reason for it. Each state maps to a
*different* wire behaviour, and none of them can be derived from a shared
boolean:

| state | ActivityPub | ATProto |
|---|---|---|
| disabled | stop delivering; actor still resolves | account `deactivated` |
| archived | actor resolves read-only; emits nothing; rejects inbound. ActivityPub has no memorialisation concept, so this is the closest honest mapping | no direct equivalent — closest is a deactivated repo that still serves records |
| deleted | `Delete` activity, then `Tombstone` / `410 Gone` | tombstone the DID document |

An `enabled` flag carrying four meanings cannot answer "what do we serve a
remote server", because the four meanings have four different answers. So
the split is the federation-relevant decision, and it is already the plan.

**Two things worth not foreclosing**, both cheap now and awkward later:

- **A soft-deleted actor must stay distinguishable from one that never
  existed.** A remote server needs `410 Gone` rather than `404`, and hard
  delete destroys the ability to say which. `node_deletion_logs` is already
  the right place for that record; the thing to avoid is a hard delete that
  removes the log row along with the node.
- **Identity is going to need somewhere to live.** Both protocols carry an
  identifier the local database does not have today — a DID for ATProto, an
  actor URI for ActivityPub — and they belong to *federated objects*
  generally, not only to actors. Adding a nullable column later is easy; the
  part that is not easy is that by then every query joins on the assumption
  that a node's identity is its integer id. Nothing to build now. Worth
  knowing that the id is a local handle and not an identity.

Deliberately not decided here: whether archived actors federate at all.
That is a policy question about a service that does not exist.

---

---

## Both states want the audit log

`node_deletion_logs` is keyed on a node and its `action` column is a varchar
precisely so a new action costs nothing. Disabling, enabling, archiving and
unarchiving should all write there rather than each growing its own table.

That does make the name narrower than the contents. **Decide when the first
of these lands whether it becomes `node_lifecycle_logs`** — cheap to rename
while the table is still unshipped, not cheap afterwards.

---

## The email and handle stay reserved, forever

**Confirmed 2026-09-07, and it already holds** — recorded because it is a
property that is easy to break later without noticing.

An archived person cannot sign in, and nobody else may take their address:

- Archiving writes `nodes.archived_at` and nothing else. It never touches
  `people_people`, so the credentials row — which holds the email and the
  username — survives intact.
- `Users.IsEmail` and `IsUsername` count that table with no filter at all, so
  both answer "taken" for an archived account exactly as they do for a live
  one.
- Nothing sweeps archived rows. Deletion has a thirty-day sweeper; archive has
  no equivalent and should not get one.

So the reservation is permanent, which is stronger than the deletion case,
where the credentials row goes when the grace period expires and the address
becomes available again.

**This matters when signup and invite acceptance are built.** Both check
`IsEmail` before creating an account, so both already refuse an archived
person's address. The thing to avoid is a later job that "tidies up" archived
accounts by removing their `people_people` rows: it would free the address of
somebody who deliberately left, and a new account could then receive mail meant
for them and inherit a handle people still associate with them.

---

## Scope — actors first, media later

**Decided 2026-09-07.** Archive is being built for ACTORS only.

Media nodes will get it too, and the reason it is not in this pass is not
effort in the archive itself — it is that archived media needs somewhere to be
seen. An archived post drops out of feeds and search by the same rule as
everything else, which leaves it reachable only by direct link. That is fine
for a profile, which has a URL somebody remembers; it is not fine for a post,
which nobody has bookmarked.

So media archiving needs **a browse view per actor profile listing that
actor's archived nodes** — the place you go to find what you put away. Without
it, archiving a post is indistinguishable from losing it.

The read filter built for actors is the same predicate media will need
(`owner.archived_at IS NULL`, derived rather than copied — see above), so the
second pass is the browse view plus the per-node control, not a rework.

---

## Work

**[services]** `nodes.archived_at`, with a guarded migration and the matching
`legacy/upgrade.sql` entry, following
`20260903000002_nodes_deleted_at.sql`. Archive and unarchive on the actor
gRPC service, and routes on group-service. The read filter. Populating
`ActorNodeItem.Enabled`. An enable/disable route that is not a side effect of
deleting. Audit rows and notifications for all four transitions.

**[react]** Danger zone cards for disable and archive, weighted to match how
reversible each one is. Copy that does not present archive as a softer
delete.
