# Actor lifecycle states — disable and archive

Written 2026-09-04, split out of `actor-profile-management.md` so the two
states can be designed together rather than one at a time.

Both land in the Danger zone alongside deletion. **Disable applies to every
actor; archive applies to non-person actors only** — a group, an event, a
photo album, anything that holds content but does not sign in.

Spans two repositories. Items are marked **[services]** or **[react]**.

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

## Archive — for non-person actors

A group, event or album that is *finished* rather than regretted. It and
everything it owns drop out of feeds and search, nothing is destroyed, and
the content stays readable at its own URLs forever.

### The cascade is cheaper than it looks

`medium.OwnerID` is the actor a medium is posted **to**, not its author —
`created_by` holds the author. So a group owns its posts, photos and topics
outright, and `GetBaseSelectQuery` already carries
`LEFT JOIN nodes AS owner ON node.owner_id = owner.id`.

That makes the filter a single predicate on an existing join —
`owner.archived_at IS NULL` — with no flag to propagate to owned nodes and
nothing to backfill or keep in sync. Prefer that to denormalising a flag
onto every child.

### To decide

- **Where the filter belongs.** Feeds and search each build their own
  queries; adding the predicate in `GetBaseSelectQuery` catches everything
  at once, but also catches the paths that *should* still see archived
  content — the actor's own profile, and the admin views.
- **How much ceremony.** Permanence forces this question. Deletion got a
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
- **Federation.** An archived actor probably still serves its ActivityPub
  actor object, read-only — unlike a deleted one, which should eventually
  answer `410 Gone`.
- **Members.** Archiving a group with active members changes what they can
  do without removing anything. They should be told.

---

## Both states want the audit log

`node_deletion_logs` is keyed on a node and its `action` column is a varchar
precisely so a new action costs nothing. Disabling, enabling, archiving and
unarchiving should all write there rather than each growing its own table.

That does make the name narrower than the contents. **Decide when the first
of these lands whether it becomes `node_lifecycle_logs`** — cheap to rename
while the table is still unshipped, not cheap afterwards.

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
