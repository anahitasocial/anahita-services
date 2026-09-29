# Complete hard delete and bulk cleanup of inactive accounts

*Roadmap item 5, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
Super-admins need to remove an actor and everything belonging to it in one go, and to clear out many dormant accounts at once.

**What exists:** `DELETE /people/:id/purge` and `DELETE /groups/:id/purge`.
- Super-admin only, with step-up re-auth, and guarded against removing the last super-admin.
- They run `graph-grpc-service/purge/purge.go#Account`, which the hourly sweeper also uses for accounts soft-deleted more than 30 days ago. It removes:
  - content the actor **owns** and its descendants, plus every edge touching them (`nodes_purge.go:73-118`);
  - the actor's files (via file-grpc);
  - the actor's credentials (auth-service consumes `node.actor.*.purged.v1`, `account_deleted.go:226-317`).
- It leaves a tombstone (the alias stays reserved) and two audit rows.

**Gaps:**
1. Content the person **wrote** elsewhere survives, credited to the tombstone: posts in groups or on other people's profiles (`owner_id` is the profile, `created_by` is the author), comments on other people's posts, and photos uploaded to groups.
2. `notifications` rows are never deleted. notification-service has no consumer for actor events.
3. `signup_requests` rows (email and password hash) and invites the person sent survive.
4. Counters on other nodes go stale: `follower_count` and `leader_count` of the people they followed or were followed by, and vote and comment counts on media.
5. The whole purge is one transaction under the 3 s `DBTimeout` (`anahita-libs/constants/constants.go:16`). A big actor rolls back and sits soft-deleted for 30 days.
6. The group purge handler doesn't map the last-super-admin error to a 409, unlike the person handler (`person.go:625`).
7. There is no web UI for purge, no admin list of people with inactivity filters, and no bulk path. Each purge consumes one step-up proof, and the browse request supports only `q`, `disabled`, `featured` and `created_on`/`modified_on` ordering (`anahita-libs/requests/actor.go:50-62`).

The policy we keep is `docs/plans/done/actor-lifecycle-states.md:30-35`: hard delete is **super-admin only** and is for site cleanup. Bulk cleanup is "a filtered list with a multi-select, reached from administration" (`actor-profile-management.md:448-472`).

## Implementation

**A. A complete purge** (graph-grpc-service, anahita-libs, auth-service, notification-service)
1. **Also walk `created_by`.** The target set becomes nodes owned by the actor **or written by it**, plus all their descendants by `parent_id`. That covers posts anywhere, comments anywhere with the replies under them, reposts and uploaded photos. Extend the recursive CTE in `nodes_purge.go:73-118` and the file collection in `nodes_purge.go:228-276`, and make `ReadDeletionCounts` (`nodes_deletion_counts.go`) count the same set, so the confirmation screen tells the truth.
2. **Chunked and resumable.**
   - Collect the target ids into a temporary table (or a `purge_jobs` row with state), then delete edges, then nodes, in batches of 500, each in its own short transaction under a longer overall context (for example 2 minutes).
   - The actor's `purge_started_at` marks the work in progress. If it's interrupted, the next run (the request retrying, or the sweeper) picks up where it stopped. The delete steps are idempotent.
   - Delete files after the rows, so a crash can't lose files for rows that still exist.
3. **Recompute counters** after deletion, for the actors and media that lost edges. Collect their ids before deleting, then reuse the recount in `socialgraph_counts.go:27-60` and add a similar one for vote and comment counts.
4. **Notifications:**
   - notification-service subscribes to `node.actor.*.*.purged.v1` and deletes rows where the purged actor is the recipient or subject.
   - Rows whose object or comment no longer exists are removed by the same purge's id set, since the purge publishes the list, or by a nightly orphan sweep in the sweeper job.
   - Doing both keeps it correct even though the event bus can drop messages.
5. **auth-service:** on the purged event, also delete `signup_requests` rows for the account's email or username and pending invites the person sent (`account_deleted.go:handlePurged`).
6. **Groups left without an admin** (decided 2026-09-28): the purge goes ahead. The group keeps existing, and a super-admin appoints an admin later.
   - Confirm that `POST /groups/:id/admins` (`group-service/cmd/api/routes.go`) lets a site super-admin add an admin to any group. If not, add that permission.
   - Add a "no admin" filter so these groups can be found (see C).
7. **Group handler:** map the last-super-admin error to a 409, as the person handler does.
8. **Tests:** extend the purge integration tests with a person who wrote a post in a group, a comment on someone else's post with a reply under it, a vote, notifications, a follow in each direction, a signup request and a sent invite. After the purge, everything is gone, the counters are right, and the tombstone plus audit rows remain.

**B. Batch purge** (person-service, group-service, graph-grpc-service)
1. `POST /people/purge-batch` and `POST /groups/purge-batch`, with the body `{ ids: [...], confirm: "PURGE 37" }`.
   - Super-admin only, one step-up proof per batch, and the typed confirmation must match the count.
   - At most 500 ids per batch. Admins and super-admins are refused; demote them first.
2. The endpoint **doesn't purge inline**. It marks each actor `purge_requested_at = now` (soft-deleting it at once, so it disappears from the site immediately) and returns 202 with a batch id.
3. Extend the sweeper (`graph-grpc-service/sweeper/sweeper.go`) to also process `purge_requested_at IS NOT NULL`, ahead of the 30-day-grace set, using the same resumable `purge.Account`.
   - For cleanup batches, run it every few minutes (or as a separate cronjob). It already has `concurrencyPolicy: Forbid` and the advisory lock.
   - Each actor still gets its `hard-deleted` audit row naming the super-admin who requested it.
4. `GET /people/purge-batch/:id` returns progress: queued, done, failed with reasons.
5. The single-actor purge routes also switch to this path, as a batch of 1. That removes the timeout risk.

**C. Admin people list with inactivity filters** (person-service, group-service, graph-grpc-service)
- Extend the admin browse (admin callers only) with these filters:
  - `last_active_before`: the latest of `last_visit_date`, the latest `auth_logs.created_on` and the latest authored node. `last_visit_date` alone can't be trusted: for 822 legacy people it holds the join date (`nodes_last_visit.go:9-12`).
  - `created_before`
  - `email_verified=false`
  - `onboarded=false`
  - `content_max=N`: authored plus owned nodes
  - `state=active|disabled|deleted`
  - for groups, `has_admin=false`
- Each row returns: last active, created, verified, onboarded, content count, followers.
- A computed `tier`:
  - `empty`: no content and no follows;
  - `dormant`: no activity for 2 years, configurable;
  - `active`: everything else.

  This is the classification proposed in `actor-profile-management.md:88-92`.
- Computing last activity on every request is fine at a few hundred actors. Add a `last_active_at` column refreshed by the sweeper later if it gets slow.

**D. Web (anahita-web)**
1. **Admin "Accounts" tab** (item 4a area, super-admin only): `containers/admin/Accounts/`.
   - Filter bar: the tier chips (Empty, Dormant, Active), last active before, created before, unverified, never onboarded, max content, and state. People and Groups sub-tabs, with a "no admin" filter on Groups.
   - A table with checkboxes and a "select all matching" option.
   - **Delete permanently**, which opens a dialog listing the count, the totals from `ReadDeletionCounts`, and a typed confirmation (`PURGE 37`), then step-up. Step-up UI already exists in `containers/actors/Settings/Delete.jsx`; reuse it.
   - Then a progress view that polls the batch status. Failures are listed with reasons, for example that someone was an admin.
2. **Single profile:** add "Delete permanently" to the Danger zone (`containers/actors/Settings/sections.js`, DANGER block), visible to super-admins only, with a typed alias and step-up. It calls the single purge, which is now a batch of 1.
3. **Groups without an admin:** from the Groups "no admin" filter, an "Appoint admin" action picks a member and calls the admins endpoint.
4. i18n in en-GB and fr-FR, inside the `admin` namespace from item 4a.

## Estimate

| Part | Time |
| --- | --- |
| A. Complete purge: `created_by` walk, chunked/resumable, counters, notifications consumer + orphan sweep, auth cleanup, group 409, tests | 8–10 h |
| B. Batch endpoints + sweeper queue + progress | 4–5 h |
| C. Admin filters, tiers, has_admin | 3–4 h |
| D. Web: Accounts tab with bulk delete + progress, Danger zone button, appoint-admin | 6–7 h |
| Docs | 1 h |
| **Total** | **about 22–27 h, about 6 sessions** |

A alone (about 2½ sessions) is worth shipping first, because it also fixes the 30-day sweeper's gaps.

## Verification
- **Integration test (A.8)** passes. Purging a person with more than 5,000 nodes succeeds without timing out, and a purge killed halfway finishes on the next run.
- **Manual:**
  - In the Accounts tab, filter "Empty + never onboarded", select all, delete permanently with a typed confirmation and step-up. Progress reaches 100%.
  - Those profiles 404, their aliases stay reserved, and `node_deletion_logs` has one `hard-deleted` row per account naming you.
  - Someone who followed a purged person sees their following count drop.
  - A group whose only admin was purged shows up under "no admin", and appointing a member works.
- **Negative cases:** an ordinary admin gets 403 on purge and batch, a batch with an admin in it is refused, and a wrong typed confirmation gives 400.

## Decisions (item 5)
- Hard delete stays super-admin only.
- A purge removes everything the person wrote anywhere, including replies under their comments.
- Groups left without an admin stay. A super-admin appoints an admin later, using the "no admin" filter.

## Dependencies
- The Accounts tab needs the admin area (item 4a).
- A is independent and can go early. It makes the sweeper correct for everyone who deletes their own account.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **New services page `account-lifecycle.md`**: disable, archive, delete, purge and bulk cleanup in one place for operators (what each removes, who may do it, the 30-day sweeper, the purge queue). It absorbs what operators need from the finished `plans/done/actor-lifecycle-states.md`.
- **services `configuration.md`**: sweeper interval, dormant threshold, batch limits.
- **services `permissions.md`**: hard delete is super-admin only; appointing admins to groups left without one.
- **web `architecture.md`**: the admin Accounts tab and the Danger-zone "Delete permanently".

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
