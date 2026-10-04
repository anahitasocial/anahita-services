# One name for timestamps: `_at`

*Roadmap item 28, planned 2026-10-03. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
Timestamps are named three ways. The PHP schema used `_on` and `_date`; everything written in Go since uses `_at`. A reader has to remember which table is which, and the API has the same split.

**The rule (decided 2026-10-03):** a moment, a date with a time, ends in `_at`. `_on` is kept for a calendar date with no time, a `DATE` column, of which there are none today.

## What exists
Thirteen columns in six tables do not follow it. Every other timestamp column (33 of them) already ends in `_at`.

| Table | Column | Becomes |
| --- | --- | --- |
| `nodes` | `created_on` | `created_at` |
| `nodes` | `modified_on` | `updated_at` |
| `nodes` | `last_visit_date` | `last_visited_at` |
| `nodes` | `start_date` | `starts_at` |
| `nodes` | `end_date` | `ends_at` |
| `edges` | `created_on` | `created_at` |
| `edges` | `modified_on` | `updated_at` |
| `edges` | `start_date` | `starts_at` |
| `edges` | `end_date` | `ends_at` |
| `notifications` | `created_on` | `created_at` |
| `notifications` | `modified_on` | `updated_at` |
| `auth_logs` | `created_on` | `created_at` |
| `node_deletion_logs` | `created_on` | `created_at` |
| `tokens` | `start_time` | `started_at` |

`modified_on` becomes `updated_at`, not `modified_at`: `updated_at` is what every Go-era table already calls it.

**How far the names reach:**
- **Go:** about 170 references outside tests (`created_on` 87, `modified_on` 39, `last_visit_date` 15, `start_date` 14, `end_date` 12, `start_time` 3), most in graph-grpc-service's repositories, plus the ordering parameters the API accepts (`created_on`, `modified_on`).
- **The API:** node responses carry `creation_time` and `update_time`; newer responses carry `created_at`. Request parameters include `ordering=created_on`.
- **The web app:** 39 files read `creationTime`, `updateTime`, `createdOn` or `modifiedOn`.
- **Protobuf:** the graph messages have their own field names for the same values.

## Implementation
1. **Migration** (goose, guarded, mirrored in `schema/init.sql` and `legacy/upgrade.sql`): `ALTER TABLE … RENAME COLUMN` for each of the fourteen. A rename is a metadata change in MySQL 8 and later, so it is instant at any table size. Indexes follow the column; index names that contain the old column name are renamed too.
2. **Go:** rename in the repositories, entities and requests, one table at a time, with the real-MySQL tests run after each. Ordering parameters accept the new names.
3. **The API:** node responses use `created_at` and `updated_at`. No old names are kept alongside: no site is live and the only client is the web app, changed in the same pass. The mobile app has not been started.
4. **Protobuf:** rename the fields, keeping their numbers, and regenerate.
5. **Web:** `creationTime` → `createdAt`, `updateTime` → `updatedAt`, and the ordering constants.
6. **A guard,** so it stays this way: a test that reads `schema/init.sql` and fails on a `datetime` or `timestamp` column that does not end in `_at`.

## Order
After items 4b and 5, which both write queries against these tables, and before M4, whose items add new timestamp columns and would otherwise each have to choose.

It could instead wait for item 27 (the one clean schema), where the migration half is free. The Go, API and web halves cost the same whenever they are done, and they get more expensive with every item that adds code reading the old names, which is the argument for doing it now.

## Estimate

| Part | Time |
| --- | --- |
| Migration in three places, tested on a throwaway database including the legacy path | 1–2 h |
| Go: repositories, entities, requests, ordering, protobuf, tests | 3–4 h |
| Web: field names and ordering constants | 1–2 h |
| Guard test, docs | 1 h |
| **Total** | **about 6–9 h, 2 sessions** |

## Verification
- The real-MySQL test suites pass (graph access, node round trip, auth tokens, abuse reports).
- A legacy upgrade on the reference data produces the new names.
- The guard test fails when a column named `something_on` with a `datetime` type is added to `init.sql`.
- The web app's lists still sort by newest, and posts show their dates.
- `grep` finds no `created_on`, `modified_on`, `creation_time` or `update_time` outside migrations and this plan.

## Risks
- **A missed reference fails at run time, not compile time:** column names are strings in SQL. The real-MySQL tests cover the main paths; the guard and the final `grep` cover the rest.
- **Sorting parameters in bookmarks** (`?ordering=created_on`) stop working. Unknown values fall back to the default order, which is newest first.

## Documentation

A phase is finished only when these pages match the code.

- **services `architecture.md`**: the naming rule, in one paragraph.
- Any page that shows a column or response field by its old name.

## What was built (2026-10-04)

**The database.** Migration `20261006000001_timestamps_end_in_at.sql` renames
the fourteen columns, and the twelve indexes that were named after an old
column (`type_modifed_on` loses its spelling mistake on the way). Each rename
happens only where the old name is there and the new one is not. `init.sql`
has the new names; `legacy/upgrade.sql` does the renames as its last step,
since everything above it reads the names the PHP application used.

**Go.** One scripted pass over 108 files: column names in SQL, entity and
response fields (`CreationTime` and `CreatedOn` are both `CreatedAt` now,
`UpdateTime` and `ModifiedOn` are `UpdatedAt`, `StartDate`, `EndDate`,
`LastVisitDate` and the token's `StartTime` follow), JSON tags, and the
ordering values the API accepts.

**The API.** Responses carry `created_at` and `updated_at` where they carried
`creation_time` and `update_time`. Lists sort with `ordering=created_at` or
`updated_at`; the old values are refused with 400. Nothing is kept under an
old name.

**Protobuf.** Fields renamed, numbers kept. The node message carried the two
timestamps twice: fields 64 and 66 (`creation_time`, `update_time`) were
written and never read, and are now reserved.

**Web.** `creationTime` is `createdAt`, `updateTime` is `updatedAt`,
`lastVisitDate` is `lastVisitedAt`, in 41 files, and comments are sorted with
`created_at`.

**The guard.** `TestTimestampColumnsEndInAt` in
`graph-grpc-service/data/repositories/schema_names_test.go` reads `init.sql`
and fails on a `datetime` or `timestamp` column that does not end in `_at`, or
a `date` column that does not end in `_on`. A second test feeds it the old
names to show it objects.

**Plans for work not yet done** (events, invitations, feeds, recommendations,
video) now propose the new names.

**Found on the way, and fixed separately.** Comparing the three ways a
database is built showed `edges.ordering` missing from `init.sql`. It had been
removed by mistake on 2026-10-02 with the unused `nodes` columns. Every edge
is written with it, so on a site installed fresh since then nobody could
follow, like or subscribe. Migration `20261006000002_edges_ordering.sql` puts
it back where it is missing, and `TestEdgeRoundTripAgainstTheSchema` adds an
edge the way the service does, which no test did before.

**Checked.**
- The migration on a fresh database, an upgraded one (up, up again, down, up)
  and the legacy reference data. All three end with the same columns and the
  same indexes, and no date-and-time column that does not end in `_at`.
- Every module vets and its tests pass, the real-MySQL ones against the
  renamed schema.
- On the rebuilt development stack, the public lists (people, groups, feeds,
  notes, photos, articles, topics, hashtags, places, comments, followers,
  search) answer 200 with the new names, and the logs show no query naming a
  missing column.

**Not checked.** Anything behind a sign-in on the running stack: the home
feed, notifications, posting, following. The web app's pages in a browser.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-03 |
| Implementation | done 2026-10-04 |
