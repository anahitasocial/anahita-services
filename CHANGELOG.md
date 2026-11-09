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

1. **Content warnings** (roadmap row 122, phase 1): any post can carry one,
   of five kinds, `adult`, `suggestive`, `graphic`, `nudity` or `other` in
   its author's words; taken as `warning` and `warning_text` when a post or
   a reply is made, changed with `PATCH /…/:id/warning`, and sent with every
   post as `warning`. A link pasted elsewhere shows nothing of such a post,
   and a person's download keeps it ([Content warnings](docs/content-warnings.md)).
   Migration `20261122000017_content_warnings`.
2. **A GIF, a video or a sound in a thread's notes** (roadmap row 143):
   `POST /videos/:owner_id/uploads` takes `thread_after`, the notes after
   it, and `after_upload`, the upload before it in the same thread; the
   worker makes the notes once the file is ready, and a file that fails
   stops the thread there (`thread_stopped`, `thread_notes`). Migration
   `20261122000018_thread_media`.

### Fixed

1. **A thread's chain in a list** is counted through photo, video and audio
   posts too: a photo in a thread's note ended it.

## v1.2.3 Birth, 2026-11-09

A maintenance release of the services alone, for Anahita Web v1.2.2.

### Added

1. **Two switches for sound:** `AUDIO_UPLOADS`, `off` for no audio posts
   at all, chosen or recorded (on unless said, where videos may be), and
   `AUDIO_RECORDING`, `on` for the web app to offer recording a sound with
   the microphone (off unless said). NodeInfo says both, `audioUploads` and
   `audioRecording`. Recording people, and posting what one has no right
   to, are a site's to allow.

### Fixed

1. **`AUDIO_MAX_BYTES` and `AUDIO_MAX_SECONDS` reach the services,** in
   Kubernetes and docker-compose: they were documented and never passed,
   so the defaults always held.

## v1.2.0 Birth, 2026-11-08

### Added

1. **Videos from other networks played here** (roadmap row 120): a post
   from Bluesky or the fediverse with a video says where it is
   (`media_elsewhere`), and the first time somebody presses play
   (`POST /videos/elsewhere/:id`), video-service's worker copies it here as
   one file, at 720 or smaller, kept as it is when it already plays
   everywhere, and it plays from this site's storage, never from its own
   server. A copy is removed `REMOTE_VIDEO_DAYS` (7) after it was last
   played; `REMOTE_MEDIA_MAX_BYTES` (100 MB) and `REMOTE_MEDIA_MAX_SECONDS`
   (an hour) hold what is copied. A new table, `media_elsewhere`.
2. **Audio posts** (roadmap row 137): a sound is a post of its own kind,
   `node.medium.video-service.audio.v1`, sent through the video uploads and
   made by video-service's worker into one M4A with the cover its file
   carried; up to an hour and 500 MB (`AUDIO_MAX_SECONDS`,
   `AUDIO_MAX_BYTES`), posted where videos may be. `audio` on a post;
   `GET /videos/:owner_id/?type=audio`; an Audio tab on profiles. To the
   fediverse as Mastodon's `Audio`, to Bluesky as a card leading here. A new
   column, `video_uploads.kind`.

3. **Pictures in a thread's notes** (roadmap row 143, phase 1): each part
   of `POST /notes/:owner_id/thread` and `POST /notes/:id/replies/thread`
   may have `uploads`, as a photo post's, and is then a photo post in the
   thread, its pictures checked by the same rules (`anahita-libs/photos`,
   moved from photo-service) and set in the thread's own transaction, so a
   thread is still all or none. The first part's are uploaded for the
   profile it is on and take `allow_download`; the rest are the writer's.

### Changed

1. **Short posts of 500 characters** (roadmap row 144): a note, a reply, a
   thread's note, and a photo's, video's or sound's words hold 500, as on
   Mastodon and Threads, when written and when edited (`post_length`, from
   `anahita-libs/posts`); a link counts as 23 however long, a Markdown link
   as its words. Articles, topics and an event's details keep 8,000.
   NodeInfo says it: `postMaxLength`, `postLinkLength`. To Bluesky, a link
   standing alone is shown short, at most 23 characters, with the whole
   address beside it. Posts already longer stay as they are; an edit is
   held to 500.

### Fixed

1. **The nightly file sweep keeps the copies of videos from other
   networks** (row 120): it read only video posts' files, and would have
   taken the copies for files nothing names.

2. **A post from Bluesky with more than four pictures keeps them.**
   Bluesky sends them as a gallery (`app.bsky.embed.gallery`), which was
   not read, so such a post was kept as words alone. Posts kept before
   this stay so.

### Upgrading

1. **A short post holds 500 characters** (roadmap row 144). A note, a
   reply, a thread's note, or a photo's, video's or sound's words longer
   than that, a link counted as 23, is refused with `400`, when written and
   when edited; it was 8,000. An app or a service of your own that posts
   longer ones has to split them into a thread, or post an article.
   NodeInfo says the limit, `postMaxLength`. Posts already longer stay as
   they are.
2. **Two migrations,** `media_elsewhere` and the `kind` of a video upload,
   run by the migration job as any upgrade's do
   ([Upgrading](docs/deploying.md#upgrading)).
3. **Audio is posted as videos are,** straight to the bucket, so it needs
   the bucket's CORS rule that lets the web app `PUT` there
   ([Videos](docs/videos.md#storage)); a site that takes videos has it
   already.

## v1.1.0 Birth, 2026-11-08

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
3. **A download says where to find each one again** (roadmap row 124):
   every person and post in `following.json`, `blocked.json`,
   `muted.json` and `saved.json` has its `address` (`ana@example.org`) and
   its ActivityPub `uri`, and a mute how it was made; `profile.json` says
   `format: 2`. A new folder, `mastodon/`, holds Mastodon's own four files,
   which Mastodon, Misskey, GoToSocial and others import.
   `import-export-worker` reads `DOMAIN_NAME`, `PUBLIC_DOMAIN` and
   `PUBLIC_API_URL`, as activitypub-service does.
4. **activitypub-service and atproto-service answer the other services**
   on port 8443, which the gateway never routes, guarded by the gRPC
   certificates: find somebody, or a post, on another server and keep it,
   for imports to come (roadmap row 124; `anahita-libs/servicecalls`).
5. **Imports** (roadmap row 124): `POST /imports` reads another Anahita's
   download or a CSV of Mastodon's or Misskey's, and keeps who to follow,
   block and mute and what to save; `POST /imports/:id/start` sets it going
   with the kinds chosen, and `import-export-worker` does each line as the
   person would, a line a second; `GET /imports/:id/failed` lists what
   could not be done, and why. At most 20,000 lines and 20 MB; the gateway
   takes 21 MB on `/imports`. Two tables, `data_imports` and
   `data_import_lines` ([Bring your lists in](docs/imports.md)).

6. **Following somebody who moved** (roadmap row 124, part 2): a `Move`
   from somebody on another server, to an account that names them in its
   `alsoKnownAs`, has whoever follows them here follow the new account (or
   ask to), and stop following the old; blocks and mutes carry over too.
   Each follower is told (`actor_moved`, `actor_moved_asked`); the old
   profile points to the new (`edge.actor.moved.v1`, `moved_to` on the
   profile). The sweep moves any follow left behind.

7. **Moving one's own account** (roadmap row 124, part 3), in and out, as
   Mastodon's people do. In: `PUT /activitypub/aliases` names the old
   accounts this one also is (`alsoKnownAs`), and their `Move` here is
   heeded. Out: `POST /activitypub/move`, after step-up, to an account that
   names this one; its description says `movedTo`, its followers' servers
   are sent a `Move`, its followers here follow the new account, and its
   tokens are minted without `media:write`, `likes:write`,
   `socialgraph:write` and `taxonomy:write`. `DELETE /activitypub/move`
   comes back. Three columns in `activitypub_accounts`: `also_known_as`,
   `moved_to`, `moved_at`.
8. **A video from Bluesky shows its poster:** the still picture Bluesky
   makes of it is kept as the post's picture, with the video's alt text,
   so the post says what it is, and the web app leads from it to the
   video there. Posts kept before keep their line alone.

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
2. **Two certificates are new,** `anahita-grpc-activitypub` and
   `anahita-grpc-atproto`, in `k8s/base/grpc-tls.yml`; cert-manager makes
   them when the release is applied, and the two services wait for them.
   In docker-compose, `make up` makes the development certificate again,
   as it now names both.

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
