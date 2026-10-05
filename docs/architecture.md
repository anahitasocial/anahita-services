# Architecture

[Documentation](README.md) › Architecture

How the services, the gateway and the shared library fit together.

## Contents

- [Components](#components)
- [Services](#services)
- [Routing](#routing)
- [anahita-libs](#anahita-libs)
- [Naming dates and times](#naming-dates-and-times)

## Components

Three kinds of components:

**HTTP services** (JSON, behind nginx) own an API resource family. They do
not talk to each other over HTTP.

**gRPC services** are shared infrastructure the HTTP services call
directly: `graph-grpc-service` owns the node/edge graph and every read
against it, `file-grpc-service` owns file storage, `mail-grpc-service`
owns outbound email. These calls use mutual TLS: each side presents a
certificate from the installation's own authority, so a gRPC service answers
only the other services (`anahita-libs/grpctls`; see
[Internal calls](configuration.md#internal-calls)).

**RabbitMQ** carries asynchronous fan-out. Services publish domain events
to the `anahita_events` exchange; consumers (notifications, feed, search)
react. Event names are versioned constants in `anahita-libs/event`, shaped
`node.<type>.<service>.<entity>.<action>.v1` and
`edge.<type>.<service>.<action>.v1`.

## Services

| Service | HTTP port | gRPC port | Responsibility |
| --- | --- | --- | --- |
| `nginx` | 80 | — | Gateway, routing, CORS, token validation subrequest |
| `auth-service` | 8081 | — | OAuth 2.0 / OIDC provider, server-rendered sign-in and sign-up, email verification, registration applications, the administration list of accounts, agreements, password, TOTP, WebAuthn, invites, clients, NodeInfo |
| `actor-service` | 8082 | — | The profiles of people and of groups. One service for every kind of actor; see [below](#one-service-for-every-kind-of-actor) |
| `text-service` | 8084 | — | Text media: articles, notes, topics |
| `photo-service` | 8085 | — | Photos |
| `hashtag-service` | 8086 | — | Hashtags |
| `location-service` | 8087 | — | Locations and geocoding |
| `story-service` | 8090 | — | Stories |
| `notification-service` | 8091 | 50057 | Notifications |
| `socialgraph-service` | 8092 | — | Follow, block, subscribe relationships |
| `search-service` | 8093 | — | Search |
| `comment-service` | 8094 | — | Comments |
| `like-service` | 8095 | 50058 | Votes / likes |
| `avatar-service` | 8096 | — | Actor avatars |
| `cover-service` | 8097 | — | Actor cover images |
| `feed-service` | 8098 | 50059 | Feeds and reposts |
| `abuse-report-service` | 8099 | — | Reports of people and posts, and the cases administrators decide. See [Abuse reports](abuse-reports.md) |
| `graph-grpc-service` | — | 50054 | Node/edge graph: actors, media, edges, traversal, trending |
| `file-grpc-service` | — | 50055 | File storage on any S3-compatible object store; checks what each upload really is |
| `mail-grpc-service` | — | 50051 | Email delivery over SMTP |
| `migration-job` | — | — | One-shot database migrations |
| `account-sweeper` | — | — | Purges accounts past their 30-day deletion grace period, and finishes purges a super administrator asked for. Not started by `up`; run with `docker compose run --rm account-sweeper`. See [Account lifecycle](account-lifecycle.md) |

Backing services in `docker-compose.yml`:

- **MySQL** (3306) — all persistent data
- **Redis** (6379) — sessions
- **RabbitMQ** (5672) — the `anahita_events` exchange

## Routing

nginx maps public paths to services. The full map lives in
`nginx/nginx.conf`, and the same gateway runs in docker-compose and on
Kubernetes, where the Ingress sends everything to it
(see [Deploying](deploying.md)):

| Path prefix | Service |
| --- | --- |
| `/oauth/`, `/auth`, `/signup`, `/signup-requests`, `/accounts`, `/agreements`, `/onboarding`, `/password`, `/email`, `/username`, `/reauth`, `/totp`, `/webauthn`, `/invites`, `/clients`, `/auth-logs/`, `/oauth-signing-keys`, `/.well-known/`, `/nodeinfo/`, `/static/` | `auth-service` |
| `/articles/`, `/notes/`, `/topics/` | `text-service` |
| `/people/`, `/groups/` | `actor-service` |
| `/photos/` | `photo-service` |
| `/avatars/` | `avatar-service` |
| `/covers/` | `cover-service` |
| `/comments/` | `comment-service` |
| `/likes/` | `like-service` |
| `/hashtags/` | `hashtag-service` |
| `/locations/` | `location-service` |
| `/stories/` | `story-service` |
| `/feeds/`, `/reposts/` | `feed-service` |
| `/notifications/` | `notification-service` |
| `/abuse-reports` | `abuse-report-service` |
| `/socialgraph/` | `socialgraph-service` |
| `/search/` | `search-service` |

`/internal/oauth/validate` is an internal-only subrequest target used by
nginx to validate bearer tokens against `auth-service`. The web app keeps its
tokens server-side behind a session cookie; for the locations that need it,
nginx asks auth-service to translate that cookie into a `Bearer` token before
proxying, so the services only ever see bearer tokens.

A list of posts is always one profile's: `GET /notes/<owner id>/`, and the same
for `/articles/`, `/topics/` and `/photos/`. There is no list of every post of
a type across the site, and a request without an owner is refused. Posts are
discovered through feeds, search, hashtags and profiles.

A photo post holds up to four images, each uploaded in a request of its own
and kept in `photo_files`, a table beside the post. How that works, and what
is stored for each image, is in [Photo posts](photos.md).

## One service for every kind of actor

People and groups are both actors: each has a profile, an avatar and a cover,
followers, an audience, permissions, and the same life from enabled to
archived to deleted to purged. They were served by two services,
`person-service` and `group-service`, that were the same service written
twice, and a fix made in one was regularly missing from the other. They are
now one, `actor-service`.

- **Each kind keeps its own address.** `/people/` and `/groups/` are
  unchanged, and the gateway sends both to `actor-service`. A client cannot
  tell that anything moved.
- **The old names are still in the data, as names.** A person's type is
  `node.actor.person-service.person.v1` and a group's
  `node.actor.group-service.group.v1`. Those strings are in every node and
  edge ever stored and in where avatars and covers are kept, so they stay.
  They no longer say where anything runs.
- **What is shared is written once:** signing in the viewer, step-up before a
  deletion, presigning images, the answers for a refused change, the
  configuration, the start-up. **What differs stays apart:** a person is read
  by username and cannot be created here (that is signing up, in
  `auth-service`); a group can be created and restored and has
  administrators.
- **It has no database address.** It reads and writes through
  `graph-grpc-service` and owns no tables, so it is given no database
  credentials.
- **A new kind of actor** is a handler and its routes in this service, not a
  service of its own. Events are planned that way, and so is the Website
  actor ([plans/website-actor.md](plans/website-actor.md)).

## anahita-libs

The shared module every service imports:

| Package | Contents |
| --- | --- |
| `constants` | OAuth scopes, shared enums |
| `proto` | Generated gRPC stubs: `graphpb`, `filepb`, `mailpb`, `typespb` |
| `event` | RabbitMQ connect/publish/consume, event name constants |
| `sessions` | Redis-backed sessions and cookies |
| `crypt` | Encryption with pluggable local / AWS KMS backends |
| `data`, `requests`, `responses`, `validators` | Request/response shapes and validation |
| `middleware`, `cors` | HTTP middleware |
| `geocoding`, `utils`, `cmd` | Supporting helpers |

## Naming dates and times

One rule, in the database, in the API and in the web app:

- A date **with a time** ends in `_at`: `created_at`, `updated_at`,
  `starts_at`, `ends_at`, `last_visited_at`, `deleted_at`.
- A calendar date **with no time** ends in `_on`. Nothing uses one yet.

The same name is used all the way through: the column `created_at` is the
field `created_at` in a response and `createdAt` in the web app, and a list is
sorted with `ordering=created_at`. The last change to a row is `updated_at`,
never `modified_at`.

A test, `TestTimestampColumnsEndInAt` in graph-grpc-service, reads
`migration-job/schema/init.sql` and fails on a date or time column that does
not follow.
