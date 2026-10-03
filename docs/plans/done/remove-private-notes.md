# Remove private notes

*Roadmap item 1, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
Legacy Anahita let a person post a private note on another person's profile, visible only to the two of them, and carry on the conversation in its comments. We don't want to support DMs: moderating them is a big responsibility, and they don't fit the fediverse plans.

## Fediverse check (why removing is the right call)
- **Mastodon / ActivityPub:** "direct" posts are ordinary Notes addressed only to the mentioned actors. They aren't encrypted, admins on every server involved can read them, and one wrong mention leaks the post. Supporting them would mean addressing, delivery and access control per recipient: a whole feature, not a flag.
- **Bluesky / AT Protocol:** DMs are not part of the open protocol. They run through Bluesky's own centralized chat service (`chat.bsky.*`), which a third-party server can't federate with in any simple way.
- **Conclusion:** neither network has a solid, easy protocol for this. Remove the feature. Worth checking again if AT Protocol ever gets a federated, end-to-end-encrypted messaging spec.

## What exists today
**anahita-web:** only a dead toggle.
- `src/containers/media/Composer/Forms/Note.jsx:39,65-75`: `canPrivatePost` plus a "Private" `<Switch name="is_private">` (label hard-coded in English).
- `src/containers/media/Composer/Default/index.jsx:51-55`: `handleOnChange` saves `medium.is_private`, but `handleOnSubmit` (:76-85) sends only `name` and `body`, so the flag never reaches the API.
- No rendering, state, translations or tests.

**anahita-services:** no private-note code at all, only legacy data.
- Legacy rows are ordinary notes (`nodes.type = 'node.medium.text-service.note.v1'`) whose `access` holds a comma list of two person ids instead of a keyword.
- Identifying condition: `type = 'node.medium.text-service.note.v1' AND access REGEXP '^[0-9]+(,[0-9]+)*$'`.
- Local dev DB counts:

  | Rows | Count |
  | --- | --- |
  | Private notes | 348 |
  | Comments on them | 604 |
  | Edges on the notes and their comments | about 700 |
  | Notifications | 140 |

- Nothing can create new ones. `text-service/requests/note.go:54,60` accepts only keyword access values.
- **Security leak found while exploring:** the feed access gate `graph-grpc-service/data/repositories/access_feed.go:70-72` checks only `object.access`, which is the parent's access. For a top-level post `object.access IS NULL`, so the post's own `access` is never checked. About 309 of the private notes, and **every restricted top-level post** (followers-only, myself, etc.), show up in profile and home feeds, even to logged-out visitors.

## Implementation
The feed leak is split out as Item 0. Item 1 doesn't depend on it, but Item 0 should ship first, because it covers every restricted post.

**Step 1: data-deletion migration** (anahita-services)
- New goose file `migration-job/migrations/2026MMDD00000N_delete_private_notes.sql`. Follow the stored-procedure pattern used in `20260913000002_repair_legacy_edge_types.sql`.
- Delete in this order (the schema has no foreign keys, so nothing cascades):
  1. `notifications` rows where `object_id` is a private note, or `comment_id` is one of their comments;
  2. `edges` where `node_a_id` or `node_b_id` is a note or one of its comments (subscriptions, votes, mentions, hashtags);
  3. comment nodes whose `parent_id` is a private note;
  4. the notes themselves.
- Put the ids into a temporary table first, so every step uses the same set. The delete order mirrors `nodes_purge.go:48-125`.
- Make it idempotent (running it a second time deletes nothing). Down is `SELECT 1;` with a comment saying the deletion is deliberately irreversible.
- Also add the same deletion to `migration-job/legacy/upgrade.sql`, after the type conversion (line 70) and after the notifications copy. Legacy installs stamp goose migrations as applied without running them, so without this they would keep the rows. Then run `python3 migration-job/check_sql_order.py`.
- If hashtag or mention counters are stored on nodes, recount them after the delete. Check this during implementation.
- Before running on production, take a DB backup (operational step, not code).

**Step 2: close the door on the service side**
- `access` already rejects id lists. Add a unit test on `text-service/requests/note.go` that asserts `"1,2"` is rejected, so nobody reintroduces them.
- While there: `owner` is accepted by the validator but `DetermineMediumNodeAccess` (`anahita-libs/cmd/api/handlers/access_helper.go:6-25`) has no case for it and quietly falls back. Either handle it or drop it from `oneof`. It's a small fix.
- Update `docs/permissions.md` with one sentence: access is always a keyword and there are no per-person access lists.

**Step 3: remove the client toggle** (anahita-web)
- `Note.jsx`: remove `canPrivatePost`, the `FormControlLabel` + `Switch` and their imports. Keep the "Posting on {{ name }}'s profile" placeholder, since public notes on other profiles are still allowed.
- `Composer/Default/index.jsx`: remove the `is_private` branch in `handleOnChange`, and the `medium` state if nothing else uses it.
- Check with `npm run lint` and the existing test suite.

## Estimate

| Step | Time |
| --- | --- |
| Step 1: migration + upgrade.sql + checking against the dev DB | 2–3 h |
| Step 2: validator test, `owner` fix, docs | about 1 h |
| Step 3: web toggle removal | about 30 min |
| **Total** | **about 1 session (4–5 h)** |

## Verification
- Before the migration: run the identifying SELECT plus the comment, edge and notification counts on the dev DB and record them (expect 348 / 604 / about 700 / 140).
- After: all counts are 0, a second run deletes nothing, and no rows are left pointing at missing notes (edges or notifications whose node is gone).
- Web: the note composer on another person's profile shows no Private switch, and posting still works.

## Relation to story-service
- Legacy story rows were already dropped by `legacy/upgrade.sql:433`, so there are no stories about private notes to clean up.
- Item 15 decided **not** to build story-service and removes its dead wiring, so there's no ordering constraint left.

## Decision (item 1)
- Delete without notifying users. The feature has been unreachable since the move to Go, and nobody can list these notes now, not even the two participants.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `permissions.md`**, *Access levels* / *Accepted values*: access is always one keyword; there are no per-person access lists and no private messages.
- **services `permissions.md`**, *Accepted values*: the `owner` value is either documented (if handled) or removed from the list.

## What was built (2026-10-03)

- **Migration** `migration-job/migrations/20261003000001_delete_private_notes.sql`, and the same deletion in `legacy/upgrade.sql` after the notifications table is filled. On the reference database both remove 348 notes, 604 comments, 707 edges and 88 notifications (the 140 counted in the plan overlapped), and leave no edge or note notification pointing at a missing node. A second run removes nothing.
- **Validator:** `text-service/requests/note.go` accepted `owner` where every other post type accepts `myself`, so choosing "Only me" for a note was refused. Both note requests now accept `myself`. `TestNoteAccessIsAKeyword` pins that an id list is refused.
- **Web:** the "Private" switch and its dead state are gone from the note composer.
- **Docs:** `docs/permissions.md`, *Seeing posts*, says there is no per-person sharing and no private messaging.

The feed access leak this plan mentions was fixed first, as item 0.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | done 2026-10-03, on branch `m1-safety-cleanup` |
| Applied to the dev database | done 2026-10-03 |
