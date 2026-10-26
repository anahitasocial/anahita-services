# Configuration

[Documentation](README.md) › Configuration

Create a `.env` file at the project root before the first run. It is not
committed, and neither are the signing-key and TLS `.pem` files it points at.
`docker-compose.yml` is the reference for which service reads which variable.

On Kubernetes the same variables go in an overlay's `config.env` and
`secrets.env` instead: see [Deploying](deploying.md).

## Contents

- [Shared](#shared)
- [Client addresses](#client-addresses)
- [Internal calls](#internal-calls)
- [Site identity](#site-identity)
- [Rules and promises](#rules-and-promises)
- [Registration](#registration)
- [Privacy of the installation](#privacy-of-the-installation)
- [First super administrator](#first-super-administrator)
- [OAuth signing and encryption](#oauth-signing-and-encryption)
- [Storage](#storage)
- [Locations](#locations)
- [Mail](#mail)
- [Abuse reports](#abuse-reports)
- [Account cleanup](#account-cleanup)
- [Federation](#federation)
- [The web shell](#the-web-shell)
- [Photos](#photos)
- [Video](#video)
- [Download your data](#download-your-data)

## Shared

| Variable | Used by | Value |
| --- | --- | --- |
| `ENV` | all | `dev`, `staging` or `prod`. Decides the default public URLs below, and in `dev` all mail goes to `DEVELOPER_EMAIL` |
| `DSN` | services with a database | MySQL DSN, e.g. `root:root@(mysql:3306)/anahita?parseTime=true` |
| `REDIS_ADDR`, `REDIS_PASSWORD` | HTTP services | Redis for sessions, e.g. `redis:6379` |
| `REDIS_DB` | HTTP services | Optional. The Redis database number, `0` when left out. Useful when several installations share one Redis |
| `REDIS_TLS` | HTTP services | `true` to connect to Redis over TLS, which most managed Redis and Valkey services require. The certificate is checked against the host in `REDIS_ADDR`. Default `false` |
| `SECURE_COOKIE_KEY` | HTTP services | Key that signs session cookies. auth-service also derives from it the key that sign-in codes are hashed with, so changing it invalidates the codes outstanding at that moment |
| `AMQP_ADDR` | publishers and consumers | RabbitMQ, e.g. `amqp://rabbitmq:5672` |
| `OAUTH_ISSUER` | all HTTP services | Issuer URL that tokens are signed with and checked against |
| `OAUTH_JWKS_URL` | all HTTP services | Where public signing keys are fetched. Defaults to auth-service inside the network |
| `WEB_ORIGIN` | nginx, actor-service | The web app's origin, e.g. `https://example.org`. For nginx, the one origin allowed to call the API with the session cookie. For actor-service, where the link in an event's calendar file leads; without it the link is left out. Default `http://localhost:3000` |

### Client addresses

auth-service takes a request's address from `X-Forwarded-For`, reading from the
right and skipping proxies, so an entry a client typed is never used. It treats
loopback, link-local and private addresses as proxies. That fits nginx in
docker-compose and an ingress in a cluster. A proxy on a public address in
front of auth-service would be recorded as the client.

Each proxy has to append to the header, which nginx does with
`$proxy_add_x_forwarded_for`. The address keys the per-address rate limits and
is recorded in the sign-in log.

A sign-in's city and country are looked up at ipapi.co, with a 3 second limit,
which means that third party is sent the address of everybody who signs in.
Set `IP_LOCATION_URL=off` to stop the lookup; the sign-in log then shows no
city or country. Set it to another address to use a service of your own that
answers the same way (`GET <url>/<ip>/json/` returning `city` and
`country_name`).

Private addresses are never sent, and are recorded as `localhost`,
`development`. Seeing that in a deployed sign-in log means the address reaching
auth-service is a proxy's, not the client's.

### Internal calls

The HTTP services call graph-grpc-service, file-grpc-service and
mail-grpc-service over gRPC. Those calls use mutual TLS: they are encrypted,
and each side presents a certificate signed by the installation's own
certificate authority. A caller without one is refused.

| Variable | Used by | Value |
| --- | --- | --- |
| `GRPC_TLS` | all | `on` or `off`. Unset means `on`, except with `ENV=dev`. `docker-compose.yml` sets `on` |
| `GRPC_TLS_DIR` | all | The directory holding `tls.crt`, `tls.key` and `ca.crt`. Default `/run/secrets/grpc` |

- **In docker-compose** `scripts/grpc-certs-init.sh` makes a development
  authority and one certificate in `./grpc-certs`, which every service
  mounts. `make up` and `make up_build` run it.
- **On Kubernetes** cert-manager issues a certificate per gRPC service and
  one for the callers: see [Deploying](deploying.md#certificates-between-the-services).
- **Renewal needs no restart.** A service looks at the files at most once a
  minute and uses a replaced certificate or authority from then on.
- **With `on` and no certificates,** a gRPC service refuses to start and a
  caller refuses to call. Nothing falls back to plain text.
- **`off`** is for an installation where a service mesh such as Linkerd
  encrypts and authenticates these calls instead.

The server's certificate must name the host that was dialled, so the names in
`FILE_SERVICE_ADDR`, `GRAPH_SERVICE_ADDR` and `MAIL_SERVICE_ADDR` have to be
on the certificates.

A service written in another language joins in by loading the same three
files in its gRPC library: present `tls.crt` and `tls.key`, trust `ca.crt`,
and, for a server, require a client certificate.

## Site identity

Read by auth-service, and mail-grpc-service for the branding. `PUBLIC_DOMAIN`
and `PUBLIC_API_URL` are for what speaks to other servers
(`anahita-libs/site`): `activitypub-service`, whose other settings and the
order to do things in are in [Federation](federation.md#setting-it-up).
`atproto-service` reads and writes Bluesky and needs none of them; its
settings are under [Federation](#federation).

| Variable | Value |
| --- | --- |
| `DOMAIN_NAME` | The installation's domain. Public URLs are derived from it on staging and production |
| `PUBLIC_DOMAIN` | Optional. The domain in a person's handle, `ana@example.org`, as other servers will know them. Left out, it is `DOMAIN_NAME` on a live site, `staging.DOMAIN_NAME` on a staging one, or the host of `PUBLIC_WEB_URL` when that is set. **Not to be changed once anybody here has been spoken of to another server:** they would become somebody else there |
| `FEDERATION_PROTOCOLS` | Optional, read by auth-service. The other networks the installation takes part in, with commas between: `activitypub` when `activitypub-service` is run for real, `atproto` when `atproto-service` is. NodeInfo's `protocols` claims the first; both are in `metadata.networks`, which the web app's front page reads to say that the site is part of the fediverse, with Mastodon and Bluesky. Left out, nothing is claimed. See [Federation](federation.md#setting-it-up) |
| `PUBLIC_API_URL` | Optional. The address other servers reach the API at. Left out, `https://api.DOMAIN_NAME` on a live site and `https://api.staging.DOMAIN_NAME` on a staging one. The same warning applies: the address of every person and post, as told to other servers, begins with it |
| `SITE_NAME` | Name shown on auth pages and in email |
| `ANAHITA_INSTANCE_NAME` | How this installation introduces itself to other servers, published as `nodeName` in NodeInfo. Optional — falls back to `SITE_NAME`, then `DOMAIN_NAME` |
| `ANAHITA_INSTANCE_DESCRIPTION` | A sentence about this installation, published as `nodeDescription`. Optional, and left empty rather than generated when unset |
| `SITE_LOGO_URL` | The logo at the top of the sign-in pages and of every email. Optional: unset, the pages show Anahita's mark (`auth-service/web/static/images/logo.svg`) and emails none. An absolute URL to a PNG, since mail clients drop SVG |
| `SYSTEM_EMAIL` | The From address |
| `SUPPORT_EMAIL`, `SUPPORT_PHONE`, `SUPPORT_WEBSITE` | Contact details on the web app's public `/support` page, published through NodeInfo. Phone and website are optional |
| `TOTP_ISSUER` | Optional. The label authenticator apps show; derived from `DOMAIN_NAME` when unset |
| `FRONTEND_URL` | Optional, not in `docker-compose.yml`. The web app's URL used in links. Defaults to `http://localhost:3000` in dev and `https://DOMAIN_NAME` in production |
| `AUTH_URL` | Optional, not in `docker-compose.yml`. auth-service's public URL. Defaults to `http://localhost` in dev and `https://auth.DOMAIN_NAME` in production |

## Rules and promises

What the site says about itself to the people who use it (roadmap row 102):
its **rules**, which everybody agrees to by ticking each one; its
**promises**, on its About page under "Our promises to you"; and the
**rule suggestions** a new group's administrators may start from. They are
the operator's, in a JSON file read by auth-service and served on
`GET /site` (package `anahita-libs/about`).

| Variable | Value |
| --- | --- |
| `SITE_RULES` | `on` unless `off`. On, everybody ticks the site's rules before using it, and again when their version is raised; off, nobody is asked. Whoever installs Anahita makes its creator's pledge either way ([Registration](registration.md#the-sites-rules)). Read by auth-service and published as `metadata.siteRules` in NodeInfo; see [Registration](registration.md#the-sites-rules) |
| `SITE_DEFAULTS_PATH` | Where the file is. `/app/defaults/site.json` in the image, which holds `auth-defaults/site.json`; docker-compose mounts the working tree's copy there. On Kubernetes, mount a ConfigMap over that path to change it |

```json
{
  "rules": {
    "version": "1",
    "items": [
      { "title": { "en-CA": "No hate.", "fr-FR": "Pas de haine." },
        "body": { "en-CA": "Don't attack or put down people for …" } }
    ]
  },
  "promises": [
    { "title": { "en-CA": "Changed your mind?" },
      "body": { "en-CA": "You have {{days}} days to undo deleting your account. Just sign in again." } }
  ],
  "rule_suggestions": [
    { "title": { "en-CA": "Be kind and respectful" } }
  ]
}
```

- **Every text carries its own translations,** by the web app's language
  names. A language left out falls back to another of its own (`fr-CA` to
  `fr-FR`), then to English, so a file may be written in one language.
- **Raising the rules' `version` asks everybody to agree again.** A
  correction that leaves it as it is asks nobody. A version is digits and
  dots, up to three parts.
- **`{{days}}`** is filled in with the days a deleted account can be brought
  back (30), so a promise never says what the site does not do.
- **At most ten rules and ten promises;** `"items": []` is a site with no
  rules.
- **It is read again when it changes,** without a restart. What it leaves
  out is as it comes, and a file that cannot be understood is not taken:
  the built-in one is used, and auth-service says so in its log. With
  docker-compose, change the file in place: an editor that writes a new
  file in its place is not seen through a mount of one file until the
  container is started again.
- The built-in file is `anahita-libs/about/default.json`, and
  `auth-defaults/site.json` is the same; a test keeps them so.

## Registration

Read by auth-service.

| Variable | Value |
| --- | --- |
| `REGISTRATION_MODE` | `open`, `approval`, `invite` or `closed`. **Unset means `closed`**: no self-serve sign-up, and outstanding invitations stop working |
| `INVITES_FROM` | `registered`, `administrators` (default) or `super-administrators` — the minimum role that may invite. Each admits that role and every role above it. In `approval` mode, an invitation from somebody who is not an administrator goes to the approval queue |
| `GROUPS_FROM` | `registered`, `administrators` (default) or `super-administrators` — the minimum role that may make a group. Read by actor-service, which enforces it, and by auth-service, which publishes it in NodeInfo as `groupsFrom`; give both the same value |
| `WEBSITES_FROM` | `registered`, `administrators` (default) or `super-administrators` — the minimum role that may connect a website. Read by syndication-service, which enforces it, and by auth-service, which publishes it in NodeInfo as `websitesFrom`; give both the same value. See [Websites](websites.md#settings), with `WEBSITE_HOURLY_CAP` and `WEBSITE_READ_EVERY` |
| `LOCATIONS_FROM` | `registered`, `administrators` (default) or `super-administrators` — the minimum role that may add a location. Read by location-service, which enforces it, and by auth-service, which publishes it in NodeInfo as `locationsFrom`; give both the same value. Editing and deleting a location stay with administrators. See [Hashtags and locations](hashtags.md#who-may-add-a-location) |
| `EVENTS_FROM` | `registered` (default), `administrators` or `super-administrators` — the minimum role that may make an event of their own. Read by actor-service, which enforces it, and by auth-service, which publishes it in NodeInfo as `eventsFrom`; give both the same value. An event made for a group is for that group's administrators whatever this says |
| `INVITE_PENDING_MAX` | Outstanding invitations one person may have at once. Default `5` |
| `INVITE_PENDING_MAX_ADMIN` | The same for administrators. Default `100` |
| `TOS_URL`, `PRIVACY_URL` | Where the Terms and Privacy Policy are read: a path on the web app or a full URL. Default `/legal/tos` and `/legal/privacy`. Their versions are held by the web app, not here |

## Privacy of the installation

Read by every service, the same way, so they are set once: in `.env` for
docker-compose, in the shared `config.env` for Kubernetes. They take effect on
a restart.

| Variable | Value |
| --- | --- |
| `SITE_READ_ACCESS` | What somebody who is **not signed in** may read: `public` (the default), everything that is public; `preview`, only the start of it; `registered`, nothing, a members-only site |
| `MIN_CONTENT_ACCESS` | The widest audience anything **new** may be given: `public` (the default) or `registered` |

A value that is not understood is treated as `registered`, the strictest, and
the log says so. A typo must not be what makes everything public again.

What each one does, how they combine, and how to check them is in
[Privacy](privacy.md#what-the-installation-allows).

## First super administrator

Read by auth-service.

| Variable | Value |
| --- | --- |
| `SUPER_ADMIN_EMAIL` | Seeds the first super administrator at boot, only when none exists — safe to leave set |
| `SUPER_ADMIN_USERNAME` | Optional. Derived from the email when unset |
| `SUPER_ADMIN_NAME` | Optional display name |

## OAuth signing and encryption

Read by auth-service.

| Variable | Value |
| --- | --- |
| `OAUTH_SIGNER_BACKEND` | `local` (default) or `kms` |
| `OAUTH_PRIVATE_KEY_PATH` | Signing key for `local`, mounted from `./oauth-signing.pem` |
| `OAUTH_KMS_KEY_ALIAS` | Signing key for `kms` |
| `OAUTH_SEED_CLIENT_PATH` | First-party clients seeded at first boot. See [Seeding clients](oauth.md#seeding-clients) |
| `OAUTH_KEY_LIFETIME_DAYS` | Optional, not in `docker-compose.yml`. Signing key rotation, default `90` |
| `CRYPT_BACKEND` | Encryption of secrets at rest: `local` (default) or `kms` |
| `CRYPT_LOCAL_KEY` | Key for `local`: exactly 32 characters. Keep it out of the database and its backups. activitypub-service keeps every person's signing key under it too, and atproto-service the Bluesky tokens of connected accounts, so both must be given the same value: see [Federation](federation.md#signing-keys-and-signed-requests) |
| `AWS_KMS_KEY_ID` | Key for `kms` |
| `CRYPT_KMS_REGION` | Region of that KMS key. Optional: defaults to the AWS region in the environment (`AWS_DEFAULT_REGION`) |
| `MAX_DB_CONNECTIONS` | Optional database pool size |

**What is encrypted at rest.** Two-factor (TOTP) secrets, and the private half
of a locally stored token signing key. Both use `CRYPT_BACKEND`. With the
signing key sealed, reading the database is not enough to mint tokens: the
reader also needs `CRYPT_LOCAL_KEY` (or access to the KMS key).

A signing key written by a version that stored it in the clear is sealed the
first time auth-service starts, and the log says how many were.

**If `CRYPT_LOCAL_KEY` changes or is lost,** auth-service cannot open the
signing key and refuses to start, and existing TOTP secrets become unreadable.
To recover sign-in, delete the rows in `oauth_signing_keys` and restart: a new
key is created, and everybody signs in again. People with two-factor turned on
need it reset by an administrator.

## Storage

Read by file-grpc-service. Uploaded files (avatars, covers, photos) go to an
object store that speaks the S3 API: AWS S3, another provider's, or one you
run yourself such as [Garage](https://garagehq.deuxfleurs.fr/). There is no
option to store files on the local disk.

| Variable | Value |
| --- | --- |
| `STORAGE_ENDPOINT` | Where file-grpc-service reaches the store, e.g. `https://s3.gra.io.cloud.ovh.net` or `http://garage:3900`. **Leave empty for AWS S3**, whose address follows from the region |
| `STORAGE_PUBLIC_ENDPOINT` | The address written into the links browsers load files from. Set it when it differs from `STORAGE_ENDPOINT`, which it does whenever the store is reached by an internal name. Defaults to `STORAGE_ENDPOINT` |
| `STORAGE_REGION` | The region requests are signed for; `AWS_REGION` or `AWS_DEFAULT_REGION` is read when it is left out. A store without regions still expects one: use whatever it is configured with (Garage: `garage`) |
| `STORAGE_BUCKET` | The bucket. It must exist and be private |
| `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY` | Credentials. Leave both empty on AWS to use a role or profile |
| `STORAGE_PATH_STYLE` | `true` to address files as `endpoint/bucket/key` rather than `bucket.endpoint/key`. Self-hosted stores usually need it. Default `false` |
| `STORAGE_SSE` | `AES256` to ask the store to encrypt each file at rest, or `none`. Default `AES256` on AWS, `none` when `STORAGE_ENDPOINT` is set |
| `STORAGE_SEND_ACL` | `true` to mark each upload private with an ACL header. Default `true` on AWS, `false` when `STORAGE_ENDPOINT` is set. Set `false` for an AWS bucket with ACLs disabled, the default for new buckets: see [A bucket on AWS S3](deploying.md#a-bucket-on-aws-s3) |
| `STORAGE_ALLOWED_TYPES` | Comma-separated content types that may be stored. Default `image/jpeg,image/png` |

The two headers default to off for other stores because a store that does not
implement one refuses the upload. Turn them on if yours supports them.

**What may be stored.** file-grpc-service looks at the bytes of every upload,
ignores the type the caller declared, and stores only what is on the allowed
list, under its real type. An HTML page or an SVG named `photo.png` is refused.
A store serves a file back with the type it was given, and a browser acts on
that type, so this is what keeps an upload from running script.

**Links expire.** Files are private. Each link handed to a browser is signed
and valid for 15 minutes.

### Earlier AWS names

Before the `STORAGE_*` names existed, the same settings were read from
`AWS_DEFAULT_REGION`, `AWS_FILES_S3_BUCKET`, `AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY`. They still work when the `STORAGE_*` one is unset, so
an installation on AWS S3 needs no change. `AWS_FILES_S3_ASSETS_DIR` was
required but never used, and is no longer read.

`AWS_PROFILE`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and
`AWS_DEFAULT_REGION` are also read by auth-service when `CRYPT_BACKEND` or
`OAUTH_SIGNER_BACKEND` is `kms`.

### Tested stores

| Store | Settings beyond bucket and keys | Result |
| --- | --- | --- |
| AWS S3 | none (`STORAGE_ENDPOINT` empty) | In use by the reference installation |
| Garage 2.1 | `STORAGE_ENDPOINT`, `STORAGE_REGION=garage`, `STORAGE_PATH_STYLE=true` | Passes the storage test. Also accepts uploads sent with `STORAGE_SSE=AES256` and `STORAGE_SEND_ACL=true` |

To check another store, point the `STORAGE_TEST_*` variables at a bucket on it
and run the test in `file-grpc-service/data/services`
(see `storage_integration_test.go` for the variables):

~~~sh
cd file-grpc-service && STORAGE_TEST_ENDPOINT=https://… go test ./data/services/ -run Real -v
~~~

## Locations

Read by location-service, and by actor-service for an event whose maker asked
for a map of its address. An address is turned into coordinates by Photon
first, then Nominatim, both open-source geocoders built on OpenStreetMap.

| Variable | Value |
| --- | --- |
| `GEOCODER_PHOTON_URL` | Where Photon is reached. Default: the public `https://photon.komoot.io`. `off` leaves it out |
| `GEOCODER_NOMINATIM_URL` | Where Nominatim is reached. Default: the public `https://nominatim.openstreetmap.org`. `off` leaves it out |
| `GEOCODER_CONTACT` | An email address or URL for whoever runs this installation, sent to the geocoders in the `User-Agent`. The public Nominatim's usage policy asks for one |

The public instances are shared services with strict limits, and they see every
address a member types. Both can be self-hosted; point the variables at your
own. At least one has to stay on for location-service, because a location cannot
be saved without coordinates. actor-service starts without either, and events
are then saved with their address and no map. An event's address is sent to
a geocoder only when "Show a map" is ticked on its form.

## Notifications

Read by notification-service. See [Notifications](notifications.md#how-long-they-are-kept).

| Variable | Value |
| --- | --- |
| `NOTIFICATIONS_KEEP_DAYS` | Optional. How many days a notification is kept after it was read. `90` when unset; `0` keeps them for good. An unread one is kept until it is read |

## Trending

Read by hashtag-service and location-service: over what span of time the
hashtags and locations trending are counted. A hashtag or a location trends by
how many profiles used it in that time, each once: the writers of public
posts carrying it, by when the posts were written, and the public profiles
tagged with it, by when they were tagged. See
[Hashtags and locations](hashtags.md#trending).

| Variable | Value |
| --- | --- |
| `TRENDING_WINDOW` | Optional. `daily`, `weekly`, `monthly` or `yearly`. Left out, `daily` when `FEDERATION_PROTOCOLS` names a network, where the fediverse brings enough every day, and `monthly` on an installation only its own members post to |

Whatever it starts from, a window that holds fewer than ten is widened to
the next, a week, a month, then a year, so the list is not empty on a quiet
day; the answer says which window it is of, and the web app shows it. A
client may still ask for a window (`time_window`). See [Hashtags and
locations](hashtags.md).

## Mail

Read by mail-grpc-service.

| Variable | Value |
| --- | --- |
| `SMTP_HOST`, `SMTP_PORT` | The SMTP server |
| `SMTP_USER`, `SMTP_PASSWORD` | Optional. Leave both empty for a relay that needs no authentication |
| `SMTP_SECURE` | `true` for implicit TLS (port 465); `false` (default) for STARTTLS (port 587) |
| `DEVELOPER_EMAIL` | Where every message goes when `ENV=dev`. Required in development — the service refuses to start without it |

Mail goes out over SMTP and nothing else, so any provider works — Mailgun's or
SES's relay, Postmark, an internal MTA, or a local sink such as
[mailpit](https://mailpit.axllent.org/). Message templates live in
`mail-grpc-service/templates/`.

## Abuse reports

Read by abuse-report-service.

| Variable | Value |
| --- | --- |
| `REASONS_PATH` | An installation's own list of reasons people can give when they report something. Optional: unset, the list built into the service is used. See [Using your own reasons](abuse-reports.md#using-your-own-reasons) |
| `SPAM_REMOVAL_MAX_DAYS` | How many days after it was made an account may still be removed as spam. 30 unless said. Read by graph-grpc-service, which refuses an older one, and auth-service, which publishes it as `metadata.spamRemovalMaxDays`. See [Removing as spam](account-lifecycle.md#removing-as-spam) |

Somebody may file at most 20 reports an hour. That limit, like the ones on
signing in, is counted in Redis by the shared limiter in
`anahita-libs/ratelimit`, and is not enforced while Redis cannot be reached.

## Account cleanup

Read by auth-service, for the [accounts list](account-lifecycle.md#the-accounts-list)
a super administrator uses to find accounts nobody uses.

| Variable | Value |
| --- | --- |
| `ACCOUNT_DORMANT_DAYS` | How many days without activity make an account `dormant` in that list. Optional: unset, 730 (two years). It only sorts accounts into tiers; nothing is removed because of it |
| `ACCOUNT_GRACE_DAYS` | How many days a deleted profile can be brought back before it is purged: a person by signing in again, a group or an event by an administrator. 30 unless said. Read by auth-service (the dates it tells people, `metadata.accountGraceDays` in NodeInfo, the site's promise), graph-grpc-service and the account sweeper (when it purges); all three must have the same. Not kept per profile: changing it moves the date of every profile already waiting |

Two things about purging are fixed in the code and are not settings: one purge request names
at most 500 accounts, and a purge works through an account 500 posts at a
time. How often the sweeper runs is its CronJob's schedule, in
`k8s/base/jobs/sweeper-cronjob.yml`.

graph-grpc-service and the sweeper need `FILE_SERVICE_ADDR` to remove a purged
account's uploaded files. Both have it in `docker-compose.yml` and in the
Kubernetes manifests.

Service addresses (`MAIL_SERVICE_ADDR`, `FILE_SERVICE_ADDR`,
`GRAPH_SERVICE_ADDR`) and the defaults directories (`DEFAULTS_PATH`,
`ACTOR_FEATURES_PATH`, `SEARCH_RANKING_PATH`) are set in
`docker-compose.yml` itself and need no entry in `.env`. They default to
`/app/defaults`, `/app/defaults/actor_features` and
`/app/defaults/search_ranking.json`, where the images keep their copies;
docker-compose points them at the working tree so an edit shows without a
rebuild. The last is the weights search orders "Most relevant" by; see
[Search](search.md#tuning-it).

## Federation

Read by `activitypub-service` and `atproto-service`, the two services that
speak to other networks. How to set either up, in order, is in
[Federation](federation.md#setting-it-up) and, for Bluesky,
[Running it](federation.md#running-it).

| Variable | Value |
| --- | --- |
| `FEDERATION_PROTOCOLS` | Read by auth-service: the networks to announce in NodeInfo, `activitypub`, `atproto`, or both with a comma. Left out, none |
| `ACTIVITYPUB_LAB` | Development only, for [the federation lab](federation-lab.md): lets activitypub-service reach the lab's private addresses. Never on a live site |
| `ATPROTO_APPVIEW_URL`, `ATPROTO_JETSTREAM_URL`, `ATPROTO_CDN_URL`, `ATPROTO_PLC_URL` | Where atproto-service reads Bluesky from; all optional, Bluesky's own when left out. See [Running it](federation.md#running-it) |
| `FEDERATION_LOCATIONS` | Read by activitypub-service. Whether a post from another server is tagged with the places it names, Pixelfed's `location` above all: each matched to a location here, or made. `on` unless `off`. See [Federation](federation.md#posts-coming-in) |
| `LINK_PREVIEW_URL` | Where `link-preview-service` is, for the card of a link in a post from elsewhere. Left out, such posts have no card |
| `REMOTE_MEDIA_DAYS` | Optional, read by `activitypub-service` and `atproto-service`. How many days the pictures of a post from another server are kept in storage after the post was last touched; then they are removed, and copied again when somebody asks to see them. Thirty when left out; `0` keeps them for good. See [Pictures that are removed](federation.md#pictures-that-are-removed) |
| `REMOTE_UNFOLLOWED_DAYS` | Optional, read by the same two. How many days the posts of somebody on another server are kept once nobody here follows them and the posts have been left alone; then they are removed, and after them the person, when nothing here refers to them. Thirty when left out; `0` keeps them for good. See [Posts and people that are removed](federation.md#posts-and-people-that-are-removed) |
| `REMOTE_POSTS_DAYS` | Optional, read by the same two, and **off when left out**. Any post from another server that has been left alone this many days is removed, whoever wrote it and whoever follows them. What it removes is not got back by asking |
| `REMOTE_POSTS_KEPT` | Optional, read by the same two, and **off when left out**. The most posts kept of one person on another server, the newest. What it removes is not got back by asking |

Writing to Bluesky also needs `CRYPT_BACKEND` and `CRYPT_LOCAL_KEY` (or
`AWS_KMS_KEY_ID`), the same as auth-service's, and a site open to read
(`SITE_READ_ACCESS`).

## The web shell

Only for an installation that sends the web app through `web-shell`; see
[The web shell](web-shell.md#settings).

| Variable | Value |
| --- | --- |
| `PUBLIC_WEB_URL` | The address people reach the web app at, such as `https://www.example.org`. Left out, the host each request came to is used |
| `WEB_BUILD_PATH` | docker-compose only: the folder on the host with the web app's build in it. Defaults to `web-shell/web` |
| `WEB_BUILD_DIR` | Where web-shell looks for the build inside its container. Defaults to `/app/web`; change it only with a build put elsewhere |

## Photos

| Variable | Value |
| --- | --- |
| `PHOTO_MAX_FILES` | How many images one photo post may hold, from 1 to 20. 20 unless said. Read by auth-service (which tells the web app), photo-service and the two federation services (which keep no more of a post from elsewhere). Posts made before it was lowered keep their images. See [Photo posts](photos.md#one-post-several-images) |

## Video

Only where videos may be posted. What each does, and how they go together,
is in [Videos](videos.md#settings).

| Variable | Value |
| --- | --- |
| `VIDEO_UPLOADS` | `on` to let people post videos. Off unless said |
| `VIDEO_MAX_BYTES` | The largest file taken. 500 MB unless said |
| `VIDEO_MAX_SECONDS` | The longest video taken. Ten minutes unless said |
| `VIDEO_RENDITIONS` | The sizes made, by height: `1080,720,360` unless said, up to `2160` |
| `VIDEO_DAILY_UPLOADS` | How many videos one person may send in a day. 20 unless said |
| `ANIMATION_MAX_BYTES` | The largest GIF or animated PNG taken, posted as a video that loops. 25 MB unless said |
| `ANIMATION_MAX_SECONDS` | The longest one. 30 seconds unless said |

The bucket needs a CORS rule letting the web app's address `PUT` to it:
see [Videos](videos.md#storage).

## Download your data

Only where `export-service` and its worker run. See
[Download your data](data-export.md#settings).

| Variable | Value |
| --- | --- |
| `EXPORT_EVERY_DAYS` | Days between one export and the next. 7 unless said; 0 for no wait |
| `EXPORT_KEEP_DAYS` | Days an archive is kept. 7 unless said |
| `SITE_NAME` | The site's name, for the heading of an archive's page |
