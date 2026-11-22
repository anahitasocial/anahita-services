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

### Removed

1. **`CORNER_RADIUS`,** with NodeInfo's `cornerRadius`. How round the
   corners are is the web app's to say, not the server's: anahita-web's
   `REACT_APP_CORNER_RADIUS` and `REACT_APP_CORNER_RADIUS_MOBILE` set it
   when the app is built.

### Upgrading

1. A site that set `CORNER_RADIUS` sets `REACT_APP_CORNER_RADIUS` in its
   web app's build instead (anahita-web's `docs/configuration.md`), and
   may take `CORNER_RADIUS` out of `config.env`: nothing reads it now.

## v1.5.0 Birth, 2026-11-21

### Added

1. **Rounder or squarer corners.** `CORNER_RADIUS` sets how round the web
   app's corners are, for everybody who has not chosen their own, by
   Material UI's names: `none`, `sm` (4 pixels), `md` (8), `lg` (16) or `xl`
   (24). Unset, it is `sm`, as before. It is published through NodeInfo, in
   pixels, so a change is a restart.

### Changed

1. **Valkey in place of Redis,** for sessions and rate limits: the `valkey`
   component (`valkey/valkey:9.2-alpine`) in place of `redis`, and in
   docker-compose. Redis 8 is no longer open source alone; Valkey is BSD.
   The settings are `VALKEY_ADDR`, `VALKEY_PASSWORD`, `VALKEY_DB` and
   `VALKEY_TLS`; the old `REDIS_*` names are read for this release, with a
   line in the log. A managed Redis still works.

### Upgrading

1. **Everybody signs in once:** sessions are in the new Valkey, which starts
   empty.
2. In `secrets.env`, rename `REDIS_PASSWORD` to `VALKEY_PASSWORD`, and in
   `config.env` any `REDIS_ADDR` or `REDIS_TLS` to `VALKEY_ADDR` or
   `VALKEY_TLS`. An overlay of your own that names `components/redis` names
   `components/valkey` now.
3. After applying, remove the old Redis, which `kubectl apply` leaves:
   `kubectl -n anahita delete statefulset/redis service/redis pvc/data-redis-0`
   ([Database, Valkey and RabbitMQ](docs/deploying.md#database-valkey-and-rabbitmq)).
   With docker-compose, `docker compose up -d --remove-orphans`; `db-data/redis`
   may then be deleted.

### Fixed

1. **A new profile or cover photo reaches the fediverse.** Mastodon kept
   showing the old one: it downloads a picture again only when its address
   changes, and the address was the same every time. The address now names
   the picture. A connected Bluesky account's profile stays the person's own,
   as before.

## v1.4.1 Birth, 2026-11-19

### Changed

1. **A quote stays on the network of the post it quotes.** Consent to be
   quoted on Mastodon is not consent to be carried to Bluesky, nor the other
   way round. A quote of a post from the fediverse goes to the fediverse
   alone, and one of a post from Bluesky to Bluesky alone; a quote of a post
   here goes to the networks that post is on. A quote is always shown here.
   No line or card of a quoted post goes anywhere else.
2. **On the fediverse, a quote waits for its author's yes.** Quoting a post
   from Mastodon asks its author first, and the quote goes out only when they
   say yes. When they say no, nothing goes out, and their post is taken out
   of the quote here, which keeps your words.
3. **Quote is offered only where its author lets you.** A post from the
   fediverse says who may quote it (`canQuote`), and a post from Bluesky
   whether it may be quoted at all (its postgate); both are read and followed
   when they change. A post from the fediverse that says nothing cannot be
   quoted, as on Mastodon: posts kept before this release say nothing until
   they are changed there.

## v1.4.0 Birth, 2026-11-18

### Added

1. **What you liked.** `GET /feeds/actor/<your id>/?filter=liked` lists the
   posts you liked, the latest liked first, for you alone: nobody else is
   sent it, the site's administrators included. It is the Likes tab on your
   own profile in Anahita Web.
2. **An example of a site on EKS,** with RDS, ElastiCache, S3 and SES:
   [Deploying on EKS](docs/deploying-eks.md), and `k8s/eks/` for eksctl,
   the default disk and Traefik. An example to adapt, not run on EKS; no
   Helm.

### Changed

1. **Your own reposts are no longer in your Following feed.** A repost is
   for others to see; yours are on your profile, under Reposts. Your own
   posts, and what the profiles you follow repost, stay in it.
2. **The `aws` overlay goes through Traefik,** as the `k3s` one does, in
   place of ingress-nginx, retired in March 2026. A site on it installs
   Traefik (`k8s/eks/traefik.yml`) and applies again.

### Upgrading

1. **A site built on the `aws` overlay** is reached through Traefik now,
   not ingress-nginx, and its certificates come through Traefik's Ingress
   class. Before applying the new release:
   1. Install Traefik once: `kubectl apply -f k8s/eks/traefik.yml`, from
      this release's checkout ([Deploying on EKS](docs/deploying-eks.md#traefik-and-cert-manager)).
   2. Point `api.` and `auth.` of your domain at the new load balancer
      Traefik's Service makes.
   3. Apply your overlay; then remove ingress-nginx and its load balancer.

   A site on the `k3s` overlay, or on the `traefik` component, as the
   recipes are, has nothing to do.
2. **No migrations.**

## v1.3.8 Birth, 2026-11-17

### Fixed

1. **A thread's first part on its author's Bluesky profile.** Every part
   said it was written in the same second, and Bluesky lists only one post
   of the same moment, the last: the first part was in neither the Posts
   tab nor the Replies tab. Each post now carries its own millisecond.

## v1.3.7 Birth, 2026-11-17

### Fixed

1. **Deleting a thread takes back every part of it,** on Bluesky and the
   fediverse, not its first note alone: the later parts stayed there as
   replies to a post that was gone. Each note of a deleted thread is now
   said to be gone, one event for each, and the fediverse's sweep takes
   back a note deleted with its thread, which it failed to read before.

## v1.3.6 Birth, 2026-11-16

### Fixed

1. **The card of a quoted post on Bluesky** (roadmap row 156), when the
   quoted post is not on Bluesky: its title in plain words, not its
   Markdown; whose it is, by name and handle, not the host alone; and its
   first picture, a post from the fediverse's too. A link whose words are
   only its address is shown short in what goes to Bluesky, as an address
   alone is.

## v1.3.5 Birth, 2026-11-16

### Fixed

1. **A browser that opens a profile's or a post's ActivityPub address
   lands on its page,** as on Mastodon, not on its JSON: `/ap/users/<uuid>`
   and `/ap/notes/<uuid>` send a request for `text/html` on to the page in
   the web app, the actor's or the note's `url`. Software that links to a
   profile by its address, as Bonfire does, leads to the profile. Servers
   asking for ActivityPub get the JSON as before
   ([Federation](docs/federation.md#what-other-servers-are-told)).

## v1.3.4 Birth, 2026-11-16

### Added

1. **Each reader chooses what they see of content warnings** (roadmap row
   122, phase 2): Show, Warn or Hide for each kind, read and changed at
   `GET` and `PATCH /people/:id/content-preferences`, the person's own
   alone. Every kind starts at Warn
   ([Content warnings](docs/content-warnings.md#what-each-reader-chooses)).
2. **The administrators can give a post a content warning** (roadmap row
   122, phase 4): a report can ask for one ("Should have a content
   warning", `needs_warning`); from the case, which is closed as `warned`,
   or from any post's or reply's menu, through `PATCH /…/:id/warning` by a
   site administrator who did not write it. Its author is told
   (`medium_warned`) and may not take it off
   ([Content warnings](docs/content-warnings.md#a-warning-the-administrators-gave)).
3. **Made with AI, a sixth kind of warning** (roadmap row 122, phase 5):
   `ai`, which readers start at Show; sent to neither network, and hiding
   nothing where a link is pasted. A report can ask for it ("Made with AI,
   not marked", `needs_ai_label`)
   ([Made with AI](docs/content-warnings.md#made-with-ai)).

### Upgrading

1. **One migration,** `content_preferences`: a table for each reader's
   choices, run by the migration job as any upgrade's are
   ([Upgrading](docs/deploying.md#upgrading)).

## v1.3.3 Birth, 2026-11-15

### Added

1. **Quotes on Mastodon are quotes** (roadmap row 156, FEP-044f): a quote
   names the post it quotes, with the stamp that its author let it, and
   Mastodon shows it inside. For a post here, the stamp is this
   installation's, served at `/ap/quotes/:uuid`; for a post elsewhere, its
   author is asked with a `QuoteRequest`, and the quote is sent again with
   their stamp when they accept, or without the quote when they refuse.
   Every post says who may quote it (`interactionPolicy.canQuote`), by its
   quote policy, and somebody elsewhere who asks to quote a post here is
   answered by it.
2. **Quotes from Mastodon are kept as quotes** (roadmap row 156): approved
   by a stamp, or of their own author's post, they show the quoted post
   inside, read and kept here if it was not.

### Upgrading

1. **One migration,** `activitypub_quotes`: two tables, for the quotes
   asked of other servers and the stamps given here, run by the migration
   job as any upgrade's are ([Upgrading](docs/deploying.md#upgrading)).

## v1.3.2 Birth, 2026-11-15

### Fixed

1. **A quote of a post that is on Bluesky is a quote there** (roadmap row
   156): one from Bluesky, or one of ours written there, is embedded as a
   record, Bluesky's own quote, beside the post's own pictures, video or
   card; the "↪" line and card are for a quoted post that is not on
   Bluesky.
2. **A quote's card on Bluesky has the quoted post's picture** (roadmap
   row 156): a photo post's first picture, an article's cover, a video's
   poster, a sound's cover, or the picture of a note's link card. It had
   its words alone.

## v1.3.1 Birth, 2026-11-15

### Fixed

1. **A quote reaches Mastodon and Bluesky as one** (roadmap row 156,
   part 1): a post that quotes ends with "↪" and the quoted post's
   address, its page here when it is public, its own address when it came
   from elsewhere, and no line when nobody signed out may see it. Mastodon
   makes a card of it; on Bluesky the post carries a card of it when it
   carries nothing else. It went out as the note around it alone.

## v1.3.0 Birth, 2026-11-15

### Added

1. **Custom feeds that match any, or all** (roadmap row 149): `match` in a
   feed's definition, `any` or `all`. With `any`, a post from its sources,
   or with one of its hashtags, keywords or locations, or in its area, is
   in; kinds of post and profile, language and origin still narrow, and
   what is excluded is still left out. A definition without it is `all`,
   as every feed was, and `all` is stored as nothing.
2. **Events hosted by a channel** (roadmap row 147, phase 5): `POST
   /events/` takes a channel's id in `host_group_id`, from one of its
   admins, whose admins are carried onto the event as a group's are; `GET
   /channels/:id/events/` lists the events a channel hosts.
3. **A channel's feed, connected in its settings** (roadmap row 147,
   phase 4): `GET`, `PUT` and `DELETE /channels/_connect/feeds/:channel_id`,
   for its admins. A site's domain is proved as before (`POST
   /channels/_connect`, `…/:id/check`), then its feed connected to a
   channel already made: one feed a channel (`has_feed`). Disconnecting
   keeps the notes it brought, and connecting the same feed again posts
   none of them twice.
4. **A channel's admins post as it** (roadmap row 147, phase 3): notes,
   photos, videos and sounds made on a channel are the channel's own, its
   author shown and sent; the admin who wrote each is kept as
   `written_by`, sent only to its admins and the site's administrators,
   as `written_by` on a post or a reply. Articles and topics are offered
   off, for its admins to turn on in its features. Only its admins write
   on it, whatever its features say. A post's `editor` is no longer sent
   for a channel's posts. The notes a feed makes are marked `from_feed` in
   their meta (migration `20261122000022`), and only those keep their
   site's words; what its admins write is theirs to change.
5. **Making a channel without a feed** (roadmap row 147, phase 2): `POST
   /channels/`, by whoever `CHANNELS_FROM` allows, now read by actor-service
   too; a channel has a group's privacy (`/access`, follow requests), who
   sees its follows (`/graph-access`), features, archiving and spam removal.
   A channel that lets people ask is asked to follow.
6. **Whose notifications** (roadmap row 146): what is yours, or a website's
   you look after, tells you of everyone; what you take part in tells you
   of the people you follow. Replying and reposting subscribe you to the
   post; a post's subscribers are told of likes, reposts and replies by
   people they follow; a website's admins are told as its author. Post
   notifications carry `notify_off` while the reader is told of the
   profile; profiles say `is_subscribed_by_viewer` to whoever follows them.
7. **Blocking a server yourself** (roadmap row 152): `GET`, `POST
   /socialgraph/:actor_id/blocked-servers`, `DELETE …/:domain`. Nothing
   from the server, or one under it, is shown to the person; no
   notifications; follows with people there end both ways, their server
   told as for any follow that ends, and new ones are refused. Bluesky is
   refused (`server_on_bluesky`). In the download as `domain_blocks.csv`.
   Migration `20261122000020_person_domain_blocks`.
8. **Who sees a profile's followers and whom it follows** (roadmap row
   151): `PATCH /people|groups|events/:id/graph-access`, an access level or
   none, never wider than the profile; the lists answer 403 to anybody
   else, the counts stay, and recommendations name only those who let the
   viewer see whom they follow. Migration `20261122000019_graph_access`.
9. **Mailpit in the development stack:** every message is caught and read
   at http://localhost:8025, nothing sent anywhere. Point `.env` at it
   (`SMTP_HOST="mailpit"`, `SMTP_PORT="1025"`, no user or password); see
   Getting started, Mail in development.
10. **Removing a follower who is on another server tells their server**
   (roadmap row 150), with a `Reject` of their follow, so that it stops
   delivering one's posts to them. It is not a block, and they may follow
   again.

### Changed

1. **A channel is made first, its feed connected after** (roadmap row 147,
   phase 4): `POST /channels/_connect/:id`, which made a channel of a
   proved feed, is gone, and `suggested_name` and `suggested_alias` with
   it. Make the channel with `POST /channels/`, then connect its feed.
2. **Websites are channels** (roadmap row 147, phase 1: the rename alone;
   nothing else about them changes yet). Migration
   `20261122000021_channels` renames the type to
   `node.actor.actor-service.channel.v1` everywhere it is stored, the
   tables to `channel_feeds` and `channel_feed_items`, and
   `suggest_websites` to `suggest_channels`, and rewrites custom feeds that
   name the kind. What an operator renames:
   - `WEBSITES_FROM` → `CHANNELS_FROM`, `WEBSITE_HOURLY_CAP` →
     `CHANNEL_FEED_HOURLY_CAP`, `WEBSITE_READ_EVERY` →
     `CHANNEL_FEED_READ_EVERY`; the old names are still read for this
     release, and say so in the log (on Kubernetes, rename the ConfigMap's
     keys);
   - `website` → `channel` among actor-service's kinds in `instance.json`,
     still read as channel for this release;
   - a site's own `website.json` beside the defaults → `channel.json`.

   The API's `/websites/…` addresses are `/channels/…`, connecting one
   included (`/channels/_connect`, and WebSub's callbacks at
   `/channels/_websub/…`: a hub given the old address before the upgrade
   is asked again when its lease runs out); NodeInfo's `websitesFrom` is
   `channelsFrom`; the kind is `channel` in custom feeds, recommendations
   and the accounts list. A web app from before this release does not work
   with it.

### Fixed

1. **A channel's post sent to the fediverse once** (roadmap row 147,
   phase 6): one its admins wrote was found twice by the sweep for what
   did not go out, as a channel's note and as a profile's own post.
2. **A new sound posted (`audio_add`)** can be turned off with the other
   posts, under `posts`.
3. **A sensitive post from another server keeps its pictures** (roadmap
   row 122): its warning is read, a Note's `summary` in its author's words
   or "Sensitive", or a Bluesky self-label as its kind, and its pictures,
   video or sound are kept behind it, as a post written here is. They were
   left out before.
4. **Search finds a photo post by the alt text of any of its pictures,**
   the second, third or fourth as well as the first; a whole word there
   counts as one in the post's words.
5. **A photo post with several pictures** found by search, on a hashtag's
   or a location's page, or among liked posts, is answered with all of
   them, as in a feed, not its first picture alone.

### Upgrading

1. **Websites are channels,** and three settings have new names:
   `WEBSITES_FROM` is `CHANNELS_FROM`, `WEBSITE_HOURLY_CAP` is
   `CHANNEL_FEED_HOURLY_CAP` and `WEBSITE_READ_EVERY` is
   `CHANNEL_FEED_READ_EVERY`. With Docker Compose the old names are still
   read for this release. On Kubernetes, rename the keys in your
   `config.env`: the manifests pass only the new ones, and a site that set
   `WEBSITES_FROM` would otherwise fall back to administrators.
   `CHANNELS_FROM` is now read by actor-service too, which makes channels.
2. **In `instance.json`,** `website` among actor-service's kinds becomes
   `channel` (still read for this release), and a site's own
   `website.json` beside the defaults becomes `channel.json`.
3. **The API's `/websites/…` addresses are `/channels/…`,** NodeInfo's
   `websitesFrom` is `channelsFrom`, and the kind is `channel` in custom
   feeds, recommendations and the accounts list. `POST
   /channels/_connect/:id` is gone: a channel is made with `POST
   /channels/`, and its feed connected with `PUT
   /channels/_connect/feeds/:channel_id`. A web app from before this
   release does not work with it: run Anahita Web's next release with it.
4. **Four migrations** run by the migration job, as any upgrade's do
   ([Upgrading](docs/deploying.md#upgrading)): who sees one's follows
   (`graph_access`), servers a person blocked (`person_domain_blocks`), the
   rename to channels (`channels`, which rewrites stored types, two tables
   and custom feeds that name the kind), and a channel's feed notes marked
   as such (`channel_feed_marks`). Take a backup first, as before any
   upgrade.

## v1.2.9 Birth, 2026-11-15

### Fixed

1. **Signed out after a renewal that was cut short:** a request renewing a
   session that was cancelled half way (a client giving up, a proxy's
   timeout) left the old refresh token spent and no new one, and the
   retries were taken for a stolen copy, signing the person out of that
   app on every device. Once begun, a renewal is finished.
2. **"New sign-in" emails for no sign-in:** a browser given new tokens
   because its sign-in session was still alive was emailed as a sign-in
   once half an hour had passed, or its address or browser had changed.
   The email is sent after a sign-in with a password, a code or a passkey,
   and only then.
3. **A token forwarded in its last second:** auth-service took an access
   token as valid through its whole last second, which the services
   refuse. Its expiry is compared to the instant, and `/oauth/validate`
   renews a session's token when it has under a minute left.

## v1.2.8 Birth, 2026-11-15

### Added

1. **Replies without notes:** `reply` is a kind of post in
   `instance.json`, offered where it is listed or where `note` is. A site
   of photos alone lists `{"service": "text-service", "node_types":
   ["reply"]}`: replies under its photos, and no notes of their own
   ([What a site offers](docs/instance-features.md)). Files that list
   `note` keep their replies; nothing to change.

### Fixed

1. **A thread of notes where notes are off** was refused with a 500; it is
   refused with 403 `{"error": "off"}`, as a note is.

## v1.2.7 Birth, 2026-11-09

### Fixed

1. **A profile elsewhere that approves its followers showed + Follow:**
   saving its profile and cover photos wrote its profile again without
   that, so it was kept as open, and following it seemed not to ask. Its
   lock is kept now; a profile already kept is set right the next time it
   is opened, once what was kept is an hour old.

## v1.2.6 Birth, 2026-11-09

### Added

1. **What a site offers** (roadmap row 139, its first part): an optional
   `instance.json` beside the actor feature files says which kinds of
   profile (`group`, `event`, `website`) and of post (`note`, `article`,
   `topic`, `photo`, `video`, `audio`) the site offers. Every profile has
   only those; the services refuse the rest with 403 `{"error": "off"}`,
   and graph-grpc-service will not save one; NodeInfo says what is offered
   in `metadata.offers`; the graph's gRPC service answers `ReadInstance`.
   No file is everything, as before ([What a site offers](docs/instance-features.md)).

## v1.2.5 Birth, 2026-11-09

### Added

1. **Content warnings go out with posts** (roadmap row 122, phase 3, its
   first half): to the fediverse as `sensitive: true`, which blurs a
   post's pictures and shows its words, with the warning as its `summary`
   too for a post of words alone; to Bluesky as the self-label of its kind
   (`porn`, `sexual`, `graphic-media`, `nudity`; Other has none). A
   warning changed later reaches the fediverse as an `Update`. Posts went
   without one before: a warned photo showed plainly on Bluesky and
   Mastodon ([Content warnings](docs/content-warnings.md#on-the-fediverse-and-bluesky)).

## v1.2.4 Birth, 2026-11-09

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

3. **WebP, HEIC and AVIF photos** (roadmap row 135): taken in everywhere a
   picture is, uploads, profile and cover photos and pictures from other
   servers, and kept as JPEG, or PNG where see-through. HEIC and AVIF are
   read by `heif-convert` (`libheif-tools`), now in the images of
   photo-service, identity-image-service, activitypub-service and
   atproto-service.

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
