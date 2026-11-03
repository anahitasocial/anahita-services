# Features

What Anahita does, set out by the 128 questions FediDB compares fediverse
software by (https://fedidb.com/software/features), answered from the code on
2026-11-03 and kept up to date as the roadmap's rows are done. It is what an
operator choosing software reads, and what FediDB's answers for Anahita are
taken from. The same answers are on Anahita's page there.

**Yes** is built; **partly** is done in Anahita's own way, said beside it;
**planned** has a plan; **no** is not built and not planned. What is not built
is gathered at the end, [for planning](#not-built).

## Contents

1. [Accounts and profiles](#accounts-and-profiles)
2. [Posting](#posting)
3. [Media](#media)
4. [Video and audio](#video-and-audio)
5. [Interactions](#interactions)
6. [Privacy and safety](#privacy-and-safety)
7. [Moderation](#moderation)
8. [Discovery](#discovery)
9. [Federation and protocols](#federation-and-protocols)
10. [Apps and API](#apps-and-api)
11. [Running a server](#running-a-server)
12. [Not built](#not-built)

## Accounts and profiles

| Feature | Anahita | |
| --- | --- | --- |
| Move to another server with followers | planned | [Moving](plans/moving.md) |
| Accept followers moving in | planned | [Moving](plans/moving.md) |
| Export your data | yes | [Download your data](data-export.md) |
| Import follows, blocks and mutes | partly, planned | Follows from one's own Bluesky account ([Federation](federation.md)); Mastodon's files in [Moving](plans/moving.md) |
| Delete your own account | yes | Thirty days to undo ([Account lifecycle](account-lifecycle.md)) |
| Two-factor authentication | yes | Codes from an authenticator app ([Signing in](signing-in.md)) |
| Passkeys and security keys | yes | [Signing in](signing-in.md) |
| Sign in with an external provider | no | |
| Profile metadata fields | no | One website link, and pronouns |
| Verified profile links | planned | [Verified domains](plans/verified-domains.md) |
| Profile header image | yes | A cover photo |
| Bot accounts | no | |
| Several identities under one login | partly | An admin of a website replies as it ([Websites](websites.md)) |

## Posting

| Feature | Anahita | |
| --- | --- | --- |
| Edit published posts | yes | |
| Content warnings | planned | [Content warnings](plans/content-warnings.md) |
| Formatted text in posts | yes | |
| Write in Markdown | yes | Posts are kept as Markdown |
| Posts over 500 characters | yes | Up to 8,000 |
| Admin-configurable post length | no | |
| Polls | no | |
| Scheduled posts | no | |
| Drafts | yes | Articles and topics |
| Quote posts | yes | [Permissions](permissions.md#quotes) |
| Control who can quote you | yes | [Permissions](permissions.md#quotes) |
| Control who can reply | yes | [Permissions](permissions.md#replies) |
| Custom emoji | no | |
| Post language | yes | [The language of a post](post-language.md) |
| Pin posts to a profile | yes | |
| Local-only posts | yes | Anything not public stays on the server |
| Automatically delete old posts | no | |
| Long-form articles | yes | Articles, and topics |
| Hashtags | yes | [Hashtags and locations](hashtags.md) |
| Link previews | yes | [Link previews](link-previews.md) |
| Translate posts | planned | [Translation](plans/translation.md) |

## Media

| Feature | Anahita | |
| --- | --- | --- |
| Upload JPEG images | yes | |
| Upload PNG images | yes | |
| Upload animated GIFs | yes | Played as video |
| Upload WebP images | no | |
| Upload HEIC images | no | |
| Upload AVIF images | no | |
| Upload MP4 video | yes | [Videos](videos.md) |
| Upload WebM video | yes | [Videos](videos.md) |
| Upload audio | no | |
| Several attachments per post | yes | [Photo posts](photos.md) |
| More than four attachments per post | yes | Up to 20, set by the operator |
| Image descriptions (alt text) | yes | |
| Image focal point | no | |
| Sensitive media | planned | [Content warnings](plans/content-warnings.md) |
| Removes location data from photos | yes | Every image is written out again without its metadata |
| Albums or collections | planned | [Collections](plans/collections.md) |
| Photo filters | no | |

## Video and audio

| Feature | Anahita | |
| --- | --- | --- |
| Video transcoding | yes | [Videos](videos.md) |
| Adaptive streaming (HLS) | no | |
| Live streaming | no | |
| Subtitles and captions | no | |
| Automatic captions | no | |
| Peer-to-peer video delivery | no | |
| Video chapters | no | |
| Playlists | no | |
| Podcast feeds | no | |

## Interactions

| Feature | Anahita | |
| --- | --- | --- |
| Likes | yes | |
| Boosts | yes | Reposts |
| Emoji reactions | no | |
| Downvotes | no | |
| Direct messages | planned | [Direct messages](plans/direct-messages.md) |
| Approve followers manually | yes | Follow requests |
| Follow hashtags | yes | And locations |
| Lists | no | Custom feeds instead ([Custom feeds](feeds.md)) |
| Bookmarks | yes | Saved posts |
| Join fediverse groups | no | |
| Host groups or communities | yes | [Groups](groups.md) |
| Interact from another server | no | |
| Events | yes | [Events](events.md) |

## Privacy and safety

| Feature | Anahita | |
| --- | --- | --- |
| Followers-only posts | yes | [Privacy](privacy.md) |
| Unlisted posts | no | |
| Block accounts | yes | |
| Mute accounts | no | |
| Block whole servers (personal) | no | |
| Word filters | no | |
| Hide follower and following lists | no | |
| Choose whether posts are searchable | yes | Whether other servers may find them ([Federation](federation.md)) |
| Hide from search engines | no | |
| Signed fetches required (secure mode) | no | |

## Moderation

| Feature | Anahita | |
| --- | --- | --- |
| Report content | yes | [Abuse reports](abuse-reports.md) |
| Forward reports to other servers | no | |
| Block servers (admin) | yes | [Federation](federation.md) |
| Limit servers | no | |
| Allowlist federation | no | |
| Import server blocklists | planned | [Refusing servers from a file](plans/domain-block-import.md) |
| Approve sign-ups | yes | [Registration](registration.md#the-approval-queue) |
| Invite links | yes | [Registration](registration.md#invitations) |
| CAPTCHA on sign up | no | A hidden field and a timing check instead ([Registration](registration.md#protection-against-bots)) |
| Moderation audit log | partly | Each case records who decided it and what was done |
| Appeal moderation decisions | no | |
| Custom content rules | yes | The site's, a group's and an event's rules |

## Discovery

| Feature | Anahita | |
| --- | --- | --- |
| Full-text search | yes | [Search](search.md) |
| Trending content | yes | [Hashtags and locations](hashtags.md#trending) |
| Profile directory | yes | |
| Local timeline | no | |
| Federated timeline | no | |
| RSS feeds | no | |
| Relay support | no | |

## Federation and protocols

| Feature | Anahita | |
| --- | --- | --- |
| NodeInfo | yes | [Registration](registration.md#nodeinfo) |
| RFC 9421 HTTP signatures | no | The older draft signatures Mastodon uses |
| Object integrity proofs | no | |
| Group federation (FEP-1b12) | no | Put off on 2026-10-17: a group's posts are its members', each needing their consent |
| Fetches missing replies | no | |
| Diaspora protocol | no | |
| Nomadic identity | no | |
| Webmentions | no | |
| ActivityPub client-to-server API | no | |

## Apps and API

| Feature | Anahita | |
| --- | --- | --- |
| Mastodon-compatible API | no | Decided against on 2026-10-12: Anahita's apps are built for all of Anahita ([CLI client](plans/cli-client.md)) |
| Official mobile app | planned | Anahita Mobile, once the web app has settled |
| Installable web app | yes | |
| Push notifications | no | |
| Webhooks | no | |
| Documented API | yes | [The API](api.md) |
| Supports OAuth for the API | yes | [OAuth](oauth.md) |

## Running a server

| Feature | Anahita | |
| --- | --- | --- |
| Official Docker image | yes | [Deploying](deploying.md) |
| S3-compatible object storage | yes | |
| PostgreSQL | no | |
| MySQL or MariaDB | yes | |
| SQLite | no | |
| Single-user mode | no | |
| Handles on a different domain | yes | `PUBLIC_DOMAIN` ([Configuration](configuration.md)) |
| Custom themes or CSS | yes | [Customising](customising.md) |
| Cleans up cached remote media | yes | `REMOTE_MEDIA_DAYS` ([Federation](federation.md)) |
| Plugins or extensions | partly | Extended by services: optional ones left out or replaced, and one's own calling the others over gRPC ([Services over gRPC](grpc.md)) |

## Not built

Every **no** and **planned** above, gathered for planning.

**Planned,** each with its plan:

1. Moving to another server with followers, followers moving in, and
   importing follows and blocks: [Moving](plans/moving.md)
2. Content warnings and sensitive media:
   [Content warnings](plans/content-warnings.md)
3. Direct messages: [Direct messages](plans/direct-messages.md)
4. Translating posts: [Translation](plans/translation.md)
5. Albums or collections: [Collections](plans/collections.md)
6. Verified profile links: [Verified domains](plans/verified-domains.md)
7. Importing server blocklists:
   [Refusing servers from a file](plans/domain-block-import.md)
8. An official mobile app, Anahita Mobile

**Decided against, or put off, with why:**

1. Mastodon's client API: Anahita's apps show all of Anahita, which
   Mastodon's do not have a place for (2026-10-12).
2. Group federation: a group's posts are its members', each needing their
   consent and an author on the fediverse (put off 2026-10-17).

**Not built, and open to plan:**

1. **Accounts:** signing in with another provider; profile fields; bot
   accounts.
2. **Posting:** polls; scheduled posts; a post length the operator sets;
   custom emoji; deleting old posts automatically.
3. **Media:** WebP, HEIC and AVIF images; audio; a focal point; photo
   filters.
4. **Video:** adaptive streaming (HLS); captions, and automatic ones; live
   streaming; chapters; playlists; podcast feeds; peer-to-peer delivery.
5. **Interactions:** emoji reactions; downvotes; lists; joining groups on
   other servers; interacting from another server.
6. **Privacy and safety:** unlisted posts; muting; blocking a server for
   oneself; word filters; hiding one's follower lists; hiding from search
   engines; requiring signed fetches.
7. **Moderation:** forwarding reports to other servers; limiting servers;
   federating only with a list; appeals.
8. **Discovery:** a local timeline; a federated timeline; RSS feeds of
   profiles; relays.
9. **Federation:** RFC 9421 signatures; object integrity proofs; fetching
   missing replies; Diaspora; nomadic identity; Webmentions; ActivityPub's
   client-to-server API.
10. **Apps:** push notifications; webhooks.
11. **Running a server:** PostgreSQL; SQLite; single-user mode.
