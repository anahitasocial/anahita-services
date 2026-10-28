# Account lifecycle

[Documentation](README.md) › Account lifecycle

What can happen to a person's account or a group: disabling, archiving,
deleting and purging. It covers who may do each, what each removes, and how a
super administrator clears out accounts nobody uses.

Its routes are in the API reference: [actor-service](api/index.html#actor-service) for a profile, and the accounts list in [auth-service](api/index.html#getAccounts).

## Contents

- [The four states](#the-four-states)
- [Deleting and the 30 days](#deleting-and-the-30-days)
- [What a purge removes](#what-a-purge-removes)
- [What a purge leaves](#what-a-purge-leaves)
- [Purging now](#purging-now)
- [Removing as spam](#removing-as-spam)
- [The accounts list](#the-accounts-list)
- [The sweeper](#the-sweeper)
- [Groups left without an administrator](#groups-left-without-an-administrator)
- [Where the code is](#where-the-code-is)

## The four states

| | What it does | Undone by | Who may |
| --- | --- | --- | --- |
| **Disable** | The account cannot be used or seen. Nothing is removed | Enabling it again | Site administrators |
| **Archive** | The account stops being live: it cannot sign in or post, and it is no longer suggested to anybody. Everything it posted stays readable at the same addresses | Nothing. It is permanent | The person themselves, a group's administrators, or a super administrator |
| **Delete** | The account disappears at once and is purged 30 days later | Restoring it within the 30 days | The person themselves, a group's administrators, or a site administrator |
| **Purge** | Everything the account owned or wrote is removed for good | Nothing | Super administrators, or the 30 days running out |

Deleting, archiving and purging ask the person doing it to prove who they are
again first, with their password or a passkey.

Nobody can disable, archive, delete or purge the only super administrator. The
answer is 409, and says to make somebody else a super administrator first (see
[Account types](permissions.md#account-types)).

## Deleting and the 30 days

Thirty days unless the installation says otherwise, in `ACCOUNT_GRACE_DAYS`
([Account cleanup](configuration.md#account-cleanup)); the web app says the
site's number.

A deleted account is hidden immediately and its sessions are ended. Nothing it
owns is touched, so restoring it brings it back whole.

Hidden, except from those with an edge to it (the user, 2026-10-18): its
followers and its administrators still find it in their lists of who they
follow and what they look after, and on its profile, labelled **Scheduled for
deletion** with the days left, so an administrator can get back to it and
restore it. Super administrators see every one: in those lists too, and all
together, soonest first, on the **Scheduled for deletion** tab of the web
app's administration (`GET /accounts?state=deleted` for each kind: `person`,
`group`, `event`, `website`). Anybody else no longer finds it. Once purged it is gone from every
list. A disabled account leaves every list until it is enabled again; an
archived one stays.

- A **person** restores their account by signing in within 30 days. They are
  emailed when the deletion is scheduled, with the date it becomes final.
- A **group** cannot sign in, so one of its administrators restores it from
  the group's settings. The group's administrators are emailed when it is
  deleted and when it is restored: each whose own account is enabled, but
  not the one who did it.

After 30 days the [sweeper](#the-sweeper) purges it.

## What a purge removes

Everything the account **owns**, everything it **wrote**, and everything
underneath either:

- its posts, wherever they are: on its own profile, in a group, or on somebody
  else's profile;
- its replies to other people's posts, and whatever was said under those
  replies;
- **other people's replies to its posts**, which go with the posts;
- every like, follow, subscription, block and administrator role it had;
- every notification sent to it, or sent to others about it or its posts;
- its uploaded avatar, cover and photos, from object storage;
- its sign-in details, sessions, passkeys and sign-in history;
- the invitations it sent that nobody used, and the signup request it began
  as, where signing up needed approval;
- the abuse reports it filed.

The numbers other people see are counted again afterwards: the follower and
following counts of everybody it was connected to, and the like, reply and
subscriber counts on posts and replies that stay.

The confirmation shown before a deletion counts this same set, so the figures
there are what will go.

## What a purge leaves

- **A tombstone.** The account's row stays, with the handle and nothing else.
  It is what keeps the handle from being registered again by somebody who
  would then inherit the old account's mentions and followers.
- **What the account started but does not own**: groups, hashtags and locations
  it created. Other people are in the group and other posts carry the tag.
- **The audit trail**: who deleted or purged what, and when
  (`node_deletion_logs`), and the purge request, if a super administrator
  asked for it (`purge_requests`).
- **Abuse cases about it**, closed as `actioned`, `deleted`.

## Purging now

A super administrator can remove accounts without the 30 days. This is for
clearing out accounts nobody uses, not for moderation: for that, disable the
account or decide its [abuse report](abuse-reports.md).

| Endpoint | What it does |
| --- | --- |
| `POST /people/purge-batch` | Removes up to 500 people |
| `POST /groups/purge-batch` | Removes up to 500 groups |
| `DELETE /people/:id/purge`, `DELETE /groups/:id/purge` | Removes one |
| `GET /people/purge-batch/:batch_id`, `GET /groups/purge-batch/:batch_id` | How far a request has got |

The body of a batch request names the accounts and repeats their number:

~~~json
{ "ids": [41, 42, 57], "confirm": "PURGE 3" }
~~~

| Answer | Meaning |
| --- | --- |
| 202 | Queued. The body has the `batch_id` to ask about, and how many were `queued` and `skipped` |
| 200 | Nothing to do: every account named was already purged or queued |
| 400 `confirmation_mismatch` | `confirm` does not name the number of ids |
| 403 `step_up_required` | Prove who you are first |
| 409 `purge_refused` | One or more of the accounts cannot be purged. Nothing was touched. `refused` gives the reason for each |

The reasons an account stops a request:

| Reason | Meaning |
| --- | --- |
| `not_found` | No such account |
| `wrong_type` | Not a person, on the people route; not a group, on the groups route |
| `administrator` | An administrator or super administrator. A batch never removes one: change their account type first ([Account types](permissions.md#account-types)), or remove them by name with the single route |
| `yourself` | The account asking |
| `last_super_administrator` | The only super administrator |

A request does not wait for the work. The accounts are hidden before it
answers, and the purge follows in the background, usually within seconds. The
progress endpoint gives `total`, `queued`, `done` and `failed`, and each
account's state. A purge that fails is tried three times and then marked
`failed` with the reason; asking again for that account tries again.

Nobody is emailed about a purge. What the purged accounts alone had tagged
their posts with, hashtags and locations, goes with the next run of the
[sweeper](#the-sweeper).

## Removing as spam

A moderator's decision, not a cleanup (roadmap row 102): an account found to
be spam is removed at once, as a purge removes it, without the 30 days a
deleted account waits, since by the time those run out it may be too late.

| Endpoint | What it does |
| --- | --- |
| `POST /people/:id/spam` | Removes a person as spam |
| `POST /groups/:id/spam` | Removes a group as spam |

- **Site administrators and super administrators** may, with no proof of
  identity: the web app asks once more, saying it cannot be undone, and
  spam is removed often and in a hurry.
- **Never an administrator's or a super administrator's account,** nor the
  viewer's own: refused with 409 `purge_refused`, `administrator` or
  `yourself`, as a batch purge refuses them.
- **Only a new account,** made in the last 30 days (`SPAM_REMOVAL_MAX_DAYS`):
  spam is new, and one here longer is disabled or deleted instead, with the
  30 days to bring it back (the user, 2026-10-19). An older one is refused
  with `too_old`, and the web app offers the action only while it may.
- **Recorded** in `node_deletion_logs` as `removed-as-spam` with who decided,
  when it is asked for; the purge then writes its own rows (`purged`,
  `hard-deleted`).
- The answer, and how far it has got, are as for a purge (202 with a
  `batch_id`).
- In the web app: **Remove as spam** in the profile's menu, and on a report's
  case, which is then closed as `removed_as_spam`
  ([Abuse reports](abuse-reports.md#what-administrators-do)).

## The accounts list

`GET /accounts` lists the site's people or groups for cleanup, least recently
active first. Super administrators only.

| Parameter | Value |
| --- | --- |
| `kind` | `person` (the default) or `group` |
| `q` | Part of a name or handle |
| `tier` | `empty`, `dormant` or `active` |
| `state` | `active`, `disabled`, `archived` or `deleted` |
| `last_active_before` | A date, `YYYY-MM-DD` |
| `created_before` | A date, `YYYY-MM-DD` |
| `content_max` | At most this many posts and replies. `0` for none |
| `email_verified=false` | People who never verified their address |
| `onboarded=false` | People who never finished the welcome steps |
| `has_admin=false` | Groups nobody administers |
| `sort`, `dir` | `last_active_at`, `created_at` or `name`; `asc` or `desc` |
| `limit`, `offset` | `limit` is 50 by default and at most 500 |

Each account comes with when it was created and last active, how much it has
written, how many follow it, its state and its tier. Purged accounts are not
listed.

**Last active** is the latest of: the last visit, the last sign-in, the last
thing written, and the day the account was created. For a group it is the last
post in it.

**Tiers:**

| Tier | Meaning |
| --- | --- |
| `empty` | Nothing written, follows nobody and has no followers. For a group: nothing posted in it and at most one follower |
| `dormant` | Not empty, and last active more than two years ago |
| `active` | Everything else |

The two years can be changed with `ACCOUNT_DORMANT_DAYS`; see
[Configuration](configuration.md#account-cleanup). A tier only sorts accounts
for somebody to look at. Nothing is removed because of one.

## The sweeper

The `account-sweeper` job does the purging that is nobody's request: every
account whose 30 days have run out. It also finishes any requested purge that
was interrupted, and removes the hashtags and locations no post carries any
more, which is most of what a purge leaves behind
([When one is no longer used](hashtags.md#when-one-is-no-longer-used)).

- On Kubernetes it is a CronJob, every ten minutes, and never two at once.
- With docker-compose it is not started by `make up`. Run it with
  `docker compose run --rm account-sweeper`.
- `sweeper -tags-only` removes the unused hashtags and locations and nothing
  else, and says how many: after a large purge, to see it done at once.

A purge works through an account in batches of 500 posts, and keeps a list of
what is left (`purge_work`). If the job is stopped part-way, the next run
carries on from that list. An account of any size finishes.

Uploaded files are removed only where `FILE_SERVICE_ADDR` is set, for both the
sweeper and graph-grpc-service. Without it the database is cleared and the
files stay in storage.

auth-service and abuse-report-service learn of a purge through an event. If
that event is lost, the sign-in details or the filed reports of that account
stay until somebody removes them by hand; nothing retries.

## Groups left without an administrator

Purging a person who was a group's only administrator leaves the group in
place with nobody running it. Find these with
`GET /accounts?kind=group&has_admin=false`.

A site administrator can then appoint one of its followers with
`POST /groups/:id/admins/`, giving that person's id as `admin_id`.

## Where the code is

| What | Where |
| --- | --- |
| Disabling, archiving, deleting and restoring | `actor-service/cmd/api/handlers/person.go` (`SetEnabled`, `Archive`, `Delete`), `group.go` (`Archive`, `Restore`); in the graph, `graph-grpc-service/data/repositories/nodes_archive.go` (`ArchiveActor`, `SetEnabled`) and `nodes_soft_delete.go` (`SoftDeleteActor`, `RestoreActor`) |
| Proving who you are again | `actor-service/cmd/api/handlers/step_up.go` (`requireStepUp`) |
| The only super administrator | `graph-grpc-service/data/repositories/nodes_super_admin.go` (`GuardLastSuperAdmin`) |
| What a deleted person is emailed, and their sign-in details removed on a purge | `auth-service/cmd/api/handlers/account_deleted.go` (`handlePurged`), `auth-service/cmd/api/notifiers/account_delete.go` |
| What a purge removes and leaves | `graph-grpc-service/purge/purge.go` (`Account`), `graph-grpc-service/data/repositories/nodes_purge_steps.go`, `nodes_purge.go` (`TombstoneActor`), `nodes_deletion_counts.go` (`ReadDeletionCounts`), tables `purge_work` and `node_deletion_logs`. Checked by `purge_integration_test.go` |
| Purging now | `actor-service/cmd/api/handlers/person_purge.go`, `group_purge.go`; `graph-grpc-service/data/repositories/purge_requests.go` (`PurgeRequestMaxAttempts`), table `purge_requests` |
| The filed reports of a purged person | `abuse-report-service/cmd/api/handlers/events.go` |
| The accounts list | `auth-service/cmd/api/handlers/accounts.go`, `auth-service/data/repositories/accounts.go`; `auth-service/config/bootstrap.go` (`GetAccountDormantDays`), setting `ACCOUNT_DORMANT_DAYS` |
| The sweeper | `graph-grpc-service/sweeper/sweeper.go` (`Run`; the days are `anahita-libs/constants` `AccountGraceDays()`), `graph-grpc-service/sweeper/tags.go` and `data/repositories/tags_sweep.go` (unused hashtags and locations), `graph-grpc-service/cmd/sweeper/main.go`, `k8s/base/jobs/sweeper-cronjob.yml`, `account-sweeper` in `docker-compose.yml`; settings `FILE_SERVICE_ADDR` and `ACCOUNT_GRACE_DAYS` |
