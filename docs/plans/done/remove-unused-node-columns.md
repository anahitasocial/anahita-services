# Remove unused legacy node columns

*Roadmap item 0d, decided and done on 2026-10-02. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md).*

## Context
The `nodes` table came over from the PHP schema with columns the Go services carry through every read and write but never use. The question came up through `status`: the node scanner read it as a plain string, while `TombstoneActor` set it to NULL, so a purged actor's row could not be loaded. Rather than patch a column nothing uses, every unused column was audited and dropped.

## What was dropped
For each of these, no endpoint sets it, no service reads it for a decision, and the web app never shows it.

| Column | Why it existed |
| --- | --- |
| `status`, `status_update_time` | A profile status line in legacy Anahita. The legacy upgrade already blanked every value. |
| `is_default` | Legacy flag, never read. |
| `ordering` | Legacy sort position on a node. `edges.ordering` is a different column and stays. |
| `timezone` | An integer offset, which can't follow daylight saving. Events (item 12) store an IANA zone name instead. |
| `last_comment_id`, `last_comment_by`, `last_comment_on` | Never written by comment-service. Replies (item 7) won't need them. |
| `filesize`, `cover_filesize`, `cover_mimetype` | Never written by the upload paths. |
| `mutual_count` | Never calculated, only reset on purge. |
| `vote_down_count` | There is no downvote, and neither Mastodon nor Bluesky has one. The unused `edge.votedown.*` event names went with it. |

Indexes dropped with them: `type_status_update_time`, `type_default`, `last_comment_by`.

## What was renamed
`hits` is now **`view_count`**, named like the other counters, and `NOT NULL DEFAULT 0`. Media responses return it as `view_count` (it was `hits`). Nothing increments it yet; item 22 (post insights) will.

## What was kept, and why
- **`pinned`:** in use. Media edits set it and media responses return it. Item 15 builds the pin endpoint and ordering on it.
- **`verified`:** kept for a future verified-handle feature in the style of Bluesky, where a person proves they control a domain (a DNS record) and uses it as their handle. Not planned yet.
- **`language`, `start_date`, `end_date`, `quote_count`:** unused today, but items 20, 12 and 25 use them.
- **`subscriber_count`, `excerpt`, `mimetype`:** in use.

## What changed
- **Migration:** `migration-job/migrations/20261002000001_nodes_drop_unused_columns.sql`. Guarded, so it can run twice. Down brings the columns back empty, because the previous version of the code selects them.
- **`migration-job/schema/init.sql`** and **`migration-job/legacy/upgrade.sql`** carry the same change. The upgrade also loses three statements that tidied the dropped columns.
- **Protobuf:** the fields are removed from `Node` and `ReadConnectionStatsResponse` in `anahita-libs/proto/graphpb/node.proto`, with their numbers reserved, and `hits_count` is renamed `view_count` (same field number).
- **Go:** the node entity, the row scanners and column list (`graph-grpc-service/data/repositories/helpers.go`), insert, update and tombstone statements, and the gRPC mappings on both sides.
- **Web:** `mutualCount` removed from the person proptypes.
- **A new integration test,** `TestNodeRoundTripAgainstTheSchema`, adds, edits, reads, batch-reads and tombstones a node against real MySQL. The column lists in those statements are plain strings, so a mismatch compiles and fails only when run; this is the test that runs them. It caught two leftover columns during this work.

## Verification
On a throwaway MySQL 9.6:
- A fresh install from the new `init.sql` loads.
- The migration on the previous schema produces exactly the fresh schema's columns and indexes. Down restores the original columns, a second Up matches again, and running Up twice changes nothing. A NULL `hits` becomes 0 and a value is kept.
- `legacy/upgrade.sql` runs to the end on the real legacy dump (23,058 nodes). Its `nodes` table matches the fresh one except that `service` stays nullable, which was already the case.
- The graph-grpc repository tests pass against all three schemas, and every module's unit tests pass.

## Deploying
Run the migration job before the new service images. The old code selects the dropped columns, so it fails once they are gone; the new code selects `view_count`, so it fails before the migration. A brief window of errors is unavoidable unless the stack is stopped for the migration.

## Documentation
No documentation page mentioned these columns. The plans that did (`post-insights.md`, `events.md`, `quote-posts.md`, `feed-access-leak.md`, and anahita-web's `pinned-posts-and-activity-tray.md`) are updated.

## Status

| Item | State |
| --- | --- |
| Code, migration, tests | done 2026-10-02, on branch `feed-access-leak` |
| Deployed | to do |
