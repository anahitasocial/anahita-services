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

## Added after the plan was written

Agreed on 2026-10-03, while building abuse reports (item 4b):

- When a person who filed abuse reports is purged, their reports are removed
  and the counts on their cases lowered. Reports are not nodes, so the graph's
  purge does not reach them; abuse-report-service does it when it hears of the
  purge.
- The network address stored with a report is blanked after 90 days.
- There is no button for deleting a report or a case.

## What was built

### Part A: a complete purge (2026-10-03)

**The set that is removed.** Everything the actor owns, everything the actor
wrote, and everything underneath either by `parent_id`. "Wrote" stops at
content: a person is the `created_by` of the groups, hashtags and places they
started, and those stay. The set is one piece of SQL, `purgeContent` in
`graph-grpc-service/data/repositories/nodes_purge_steps.go`, used by both the
purge and `ReadDeletionCounts`, so the confirmation dialog counts what is
removed.

**Batches that can be picked up again.** A new table, `purge_work`, lists what
is left to do for a purge in progress. `purge.Account` now runs:

1. `PreparePurge`: lists the nodes to delete, and the nodes that stay but
   whose counters must be counted again (people on the other end of a follow,
   posts the actor liked or subscribed to, posts that lose one of the actor's
   comments or reposts).
2. `NextPurgeBatch` and `DeletePurgeBatch`, 500 nodes at a time, each batch in
   its own transaction: the batch's files, then its edges, the notifications
   about it, the nodes, and its rows in the list.
3. `FinishPurge`: the edges and notifications that point at the actor itself,
   then the recounts, then the list is cleared.
4. `TombstoneActor`, which now also sets `nodes.purged_at`.

Every step can run twice. A purge that is interrupted leaves the rest of the
list, and the next call for that actor carries on from it.

**`nodes.purged_at`.** The sweeper's queue is now "deleted and not yet purged".
Before, a tombstone stayed in the queue for good, and after a hundred of them
the sweeper would never have reached a new deletion.

**Other services.**
- auth-service, on the purged event, also removes the signup request the
  account began as and the invitations it sent that nobody used.
- abuse-report-service removes the reports the person filed, and blanks
  network addresses older than 90 days (hourly, in each replica).

**Also fixed.** `UpdateSubscriberCount` wrote into `vote_up_count` and passed
two values for three placeholders, so it failed on every call.

**Where it differs from the plan.**
- **Notifications are deleted by the graph's purge, not by
  notification-service.** The plan had notification-service listening for the
  purge and a nightly sweep for what it missed. A purge publishes nothing per
  post, and the bus can drop what is published, so that needed two mechanisms
  and still left a gap. The purge already has the exact list of nodes, so it
  deletes the notifications about them in the same transaction as the nodes.
  This is the one place the graph writes to that table.
- **Files are deleted before their rows, not after.** The plan's order guarded
  against losing files for rows that still exist. With batches that resume,
  the opposite order is the safe one: rows whose files are gone are still
  listed and go on the next run, while files whose rows are gone can never be
  found again.
- **No `purge_started_at` column.** The rows in `purge_work` are the marker.
- **No 409 was needed on the group purge.** The last-super-administrator check
  only ever applies to a person.
- **Appointing an administrator to a group left without one already works**:
  `CanAddAdmin` allows any site administrator.

**Tests.** `purge_integration_test.go` runs whole purges against MySQL: a
person with posts on their own profile, in a group and on somebody else's
profile, comments both ways, votes, subscriptions, follows both ways,
notifications, and a group, hashtag and place they created. It also purges an
account of 5,200 posts after stopping it at the first batch, and checks that
the last super administrator is refused. auth-service and abuse-report-service
each have real-MySQL tests for their part. The migration was run on the fresh,
upgrade and legacy paths.

**Not checked.** A purge through the running stack with files in object
storage: the tests use a stand-in for the file service. And a lost purge event
is not retried, so auth-service or abuse-report-service can miss a purge.

### Part B: the purge queue and batch purge (2026-10-04)

**Asking and purging are separate.** A new table, `purge_requests`, holds one
row per account a super administrator has asked to be removed. A request
checks every account, marks each one deleted so it is gone from the site at
once, queues it, and answers with a batch id. The purge itself runs in the
background.

**Endpoints**, the same on `/people` and `/groups`:

| Endpoint | Answer |
| --- | --- |
| `POST /people/purge-batch` with `{ "ids": [...], "confirm": "PURGE 37" }` | 202 with `batch_id`, `queued`, `skipped`; 409 with the reason for each account that stopped it; 400 if the confirmation does not name the count |
| `GET /people/purge-batch/:batch_id` | `total`, `queued`, `done`, `failed` and each account's state; 404 for an unknown batch |
| `DELETE /people/:id/purge` | the same, as a request for one |

Super administrators only, with one proof of identity per request. A request
names at most 500 accounts.

**All or nothing.** One account that cannot be purged refuses the whole
request and nothing is touched. The reasons: `not_found`, `wrong_type`,
`administrator`, `yourself`, `last_super_administrator`. A refused request
does not spend the proof of identity. Accounts already purged or already
queued are skipped and counted.

**Administrators.** A batch refuses if it names an administrator or super
administrator. The single route allows it, since that is a deliberate removal
of one named account, but never the last super administrator and never
yourself.

**Who does the work.** `sweeper.RunRequests`, started by the request inside
graph-grpc-service, under the sweeper's own database lock. With several
replicas the one that gets the lock drains the queue. The scheduled sweeper
drains the queue too, before the expired accounts, so a request whose replica
stopped part-way is finished by the next run. The Kubernetes job now runs
every ten minutes instead of hourly for that reason.

**Failures.** A purge that fails is tried again; after three failures its row
is marked `failed` with the last error, and the accounts behind it are still
reached. Asking again for a failed account replaces the row.

**Audit.** Each account gets its `purged` row and a `hard-deleted` row naming
the super administrator who asked.

**graph-grpc-service now has `FILE_SERVICE_ADDR`** in compose and Kubernetes.
Without it a purge asked for through the API cleared the database and left
the uploaded files in storage.

**Where it differs from the plan.**
- No `purge_requested_at` column on `nodes`; the queue is its own table, which
  also carries the progress and the requester.
- A batch with a problem is refused whole. The plan listed failures after the
  fact.
- The old `Purge` RPC is gone. auth-service used it to remove an account whose
  credentials could not be written during signup; those four places now ask
  through the queue.

**Tests.** Real-MySQL tests of the queue in
`graph-grpc-service/data/repositories/purge_integration_test.go`: a request
that hides at once and purges in the background, a request refused whole, and
a request that cannot work being marked failed while the one behind it is
done. Handler tests for the gates in group-service.

**Not checked.** The endpoints with a real super administrator's session and
proof of identity: only that they refuse a caller with no token. Two replicas
contending for the lock.

### Part C: the accounts list (2026-10-04)

`GET /accounts` in auth-service lists people or groups for cleanup. Super
administrators only (`actors:read` scope). The filters, the tiers and what
"last active" means are in `docs/account-lifecycle.md`.

It is one query, `auth-service/data/repositories/accounts.go`, built from
named pieces: the rows, the tiers on top of them, and the filters on top of
those. Each filter is one line that is switched off by its first placeholder.

**Where it differs from the plan.**
- **It is in auth-service, not person-service and group-service.** Whether an
  address was verified, whether onboarding was finished and the last sign-in
  are auth-service's own tables, and it already reads `nodes`. One query there
  can filter and page; split across services it could not.
- **It is its own endpoint**, not more parameters on the public browse, so
  nothing about the public lists changes.
- **Content count** is what a person wrote, anywhere, and for a group what was
  posted in it. The plan's "authored plus owned" would have counted a person's
  posts on their own profile twice.
- **Super administrators only**, where the plan said administrators. The list
  exists to choose accounts to purge, which only they can do.
- An `empty` group may have one member, since whoever starts a group follows
  it.

**Found on the development data.** Counters on accounts carried over from the
old site are NULL, not zero; the query treats them as zero. On that data: 823
people, of which 219 are empty, 602 dormant and 2 active; 23 groups, all
dormant, none without an administrator. Each query takes about a tenth of a
second.

**Tests.** The query against real MySQL (tiers, every filter, ordering,
groups), and handler tests for who may ask and how the query string is read.

**Not checked.** The endpoint with a real super administrator's session.

### Part D: the web (2026-10-04)

In anahita-web:

- **An Accounts tab in Administration** (`/admin/accounts`), for super
  administrators only. People or groups, least recently active first, with the
  filters from part C, tick boxes, "select all that match" (up to 500), and
  "Delete permanently".
- **The confirmation dialog** says what will go and how much the selected
  accounts have written, asks for the count to be typed (`PURGE 37`), asks for
  a proof of identity, and then shows the progress until nothing is queued. A
  refused request lists each account that stopped it and why.
- **"Delete permanently" in the Danger zone** of a person's or group's
  settings, last, for super administrators on anybody's profile but their own.
  It asks for the handle to be typed and a proof of identity.
- **Groups without an administrator** carry a link to the group's settings,
  where the existing Admins tab appoints one.

**Where it differs from the plan.**
- The dialog shows how much the selected accounts have written, from the list,
  not the totals of `ReadDeletionCounts`: that is one request per account.
- Appointing an administrator reuses the group's own Admins tab; no second
  picker was built.

**Not checked.** None of the web part has been seen in a browser. It builds,
its files lint clean, and the unit tests pass, but nobody has yet filtered the
list, ticked accounts and removed them through the running stack.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| A. Complete purge | done 2026-10-03 |
| B. Batch purge and the queue | done 2026-10-04 |
| C. Admin list with inactivity filters | done 2026-10-04 |
| D. Web | done 2026-10-04, not yet tried in a browser |
