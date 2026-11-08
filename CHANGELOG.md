# Changelog

What each release of Anahita Services adds, changes, fixes and removes, and
what to do when upgrading to it. The newest first. **Embryo** is `main`, what
the next Birth release will have; when that release is made, its heading
becomes the version and the date, and this is what its notes on GitHub say
([Releases](docs/releases.md)). Each change adds its line here when it is
made.

Releases before v1.0.17 are described on GitHub, under
[Releases](https://github.com/anahitasocial/anahita-services/releases).

## Embryo

### Added

1. **The cluster's own MySQL, RabbitMQ and Garage accounts** are in
   `docs/configuration.md`: `MYSQL_*`, `RABBITMQ_*`, `GARAGE_RPC_SECRET` and
   `GARAGE_ADMIN_TOKEN`, who reads each and how to make it.

2. **Mutes, their API** (anahita-web row 133):
   `GET /socialgraph/:actor_id/mutes/`, and `POST` and
   `DELETE /socialgraph/:target_id/mutes/:source_id`, with
   `hide_notifications` and `for` (`day`, `week`, `month`, or until
   unmuted). A person's own alone; nobody is told. Profiles say
   `is_muted_by_viewer`, `mute_ends_at` and `mute_hides_notifications`.
   While a mute lasts, what the muted person wrote, reposted or had
   reposted, and what is on their profile, is left out of the muter's
   Following feed, custom feeds, a hashtag's and a location's posts and
   search, and their replies and what is under them out of threads; they
   are not suggested. Their own profile, opened, is whole. With
   `hide_notifications`, what they do tells the muter nothing while the
   mute lasts. The download has `data/muted.json` and a Muted section; the
   sweeper deletes mutes that have ended.

### Changed

1. **The overlays pull Anahita's images from GitHub's registry,**
   `ghcr.io/anahitasocial/<image>`, the gateway as `gateway`, in `k3s`,
   `aws` and `generic`; before, they named `anahita/<image>`, which no
   public registry serves. A site that already sets `newName` changes
   nothing.
2. **export-service is import-export-service,** and export-worker
   import-export-worker, for imports to come (roadmap row 124): the module,
   the images, the Kubernetes names and the gateway's upstream. `/exports`,
   the tables and `EXPORT_*` settings are as they were, and archives are
   still stored under `assets/v1/export-service/`.

### Fixed

1. **A person's own list of whom they blocked showed nobody:**
   `GET /socialgraph/:actor_id/blocks/` answered with nobody, as the gate
   that hides people blocked either way was applied to the list of them.
   The list is now gated only by who may read it (anahita-web row 133).
2. **An empty list of profiles is `[]`, not `null`,** in every answer that
   lists them.

### Upgrading

1. **Two images have new names.** In your overlay's `images:`, the lines
   for `anahita/export-service` and `anahita/export-worker` become
   `anahita/import-export-service`, `newName:
   ghcr.io/anahitasocial/import-export-service`, and
   `anahita/import-export-worker`, `newName:
   ghcr.io/anahitasocial/import-export-worker`, in the same change that
   moves `ref=` to this release. The old Deployments and Service,
   `export-service` and `export-worker`, are left behind by
   `kubectl apply`; delete them once the new ones run.

## v1.0.19 Birth, 2026-11-07

### Added

1. **Asking to follow somebody elsewhere who approves their followers**
   (anahita-web row 130): their `manuallyApprovesFollowers` is kept on their
   profile as `allow_follow_request`; following them is a request, sent as
   a `Follow`, made a follow by their `Accept` through the graph's own
   accept, and taken away by their `Reject`; taking it back sends an
   `Undo`; the sweep asks again about a request whose news the bus lost.
   `POST /activitypub/people/:id/refresh` reads somebody elsewhere again
   when what was kept of them is more than an hour old, asked by the web app
   when their profile is opened.
   Profiles say `is_follow_requested_by_viewer`
   ([Federation](docs/federation.md#following-somebody-on-another-server)).

### Changed

1. **Following somebody elsewhere who approves their followers is refused;**
   they are asked instead.
2. **Migration `20261122000012`:** each profile from the fediverse takes
   `allow_follow_request` from what its server last said, and a follow sent
   to one that approves its followers, still waiting for an answer, becomes
   a request; their counts are made again.

## v1.0.18 Birth, 2026-11-07

### Added

1. **Whose feeds, in `GET /feeds/discover`** (anahita-web row 125):
   `whose=everyone`, `following` or `mine`, for somebody signed in, the web
   app's Feeds page's list ([Discover](docs/feeds.md#discover)); without
   it, as before.
2. **`owned_by_followed` in the graph's node list:** only what is owned by
   somebody the viewer follows.
3. **A thread in a list** (anahita-web row 126): in the home feed, a
   profile's and a custom feed, a note that opens a chain of its writer's
   replies carries `thread`, how many notes it has, and `thread_next`, the
   second note.

### Removed

1. **The gRPC methods' names before v1.0.17:** `ListQuotes`, `ListSaved`,
   `ListCitedBy`, `ListAuthored` and the event service's `List`, served
   beside their BREAD names for v1.0.17. A service of one's own calls
   `Browse…`.

## v1.0.17 Birth, 2026-11-07

### Upgrading

1. **Everybody signed in signs in once more.** What a session holds is now
   sealed with AES-GCM; a session sealed before is no longer read, and is
   taken as none, not an error.
2. **The file sweep starts as a dry run.** The new `file-sweeper` CronJob
   counts, each night, the stored files no row names any more, and removes
   nothing until `FILE_SWEEP=on` is in `anahita-config`. Read its log first.
3. **A service of your own calling graph-grpc-service** should call the new
   names: `BrowseQuotes`, `BrowseSaved`, `BrowseCitedBy`, `BrowseAuthored`,
   and the event service's `Browse`. The old names, `List…` and `List`, are
   served in this release and removed in the next.

### Added

1. **The file sweep** (roadmap row 106): `sweeper -files-only`, once a night
   as the `file-sweeper` CronJob, removes stored avatars, covers, photos and
   video files that no row names, a day old, kept unless it is sure; a dry
   run until `FILE_SWEEP=on`, at most 5,000 a night, counted by folder in
   its log ([The file sweep](docs/account-lifecycle.md#the-file-sweep)).
2. **`Browse` in file-grpc-service:** what is stored under a folder of
   `assets/v1/`, a page at a time, and nothing outside it.

### Changed

1. **Sessions sealed with AES-GCM** (row 119), in a cookie and in Redis, where
   AES-CFB hid them and noticed nothing: one that was changed is refused and
   taken as no session.
2. **A token must say when it runs out:** one without `exp` is refused by
   the gateway and by auth-service. Every token auth-service makes has one.
3. **BREAD names** in the code (row 127): thirty functions named `List`,
   `Remove` or `Update` are `Browse`, `Delete` and `Edit`, five of them gRPC
   methods, served under both names for this release.
4. **The API reference** has a section for what every profile has, and one
   for each kind: people, groups, events and websites.

### Docs

New: [Features](docs/features.md), by FediDB's 128 questions; profiles,
people, groups and events, and what each node is. Planned: content
warnings, direct messages, moving, threads that read as threads.
