# Architecture

[Documentation](README.md) › Architecture

How the services, the gateway and the shared library fit together.

## Contents

- [Components](#components)
- [Services](#services)
- [Routing](#routing)
- [anahita-libs](#anahita-libs)

## Components

Three kinds of components:

**HTTP services** (JSON, behind nginx) own an API resource family. They do
not talk to each other over HTTP.

**gRPC services** are shared infrastructure the HTTP services call
directly: `graph-grpc-service` owns the node/edge graph and every read
against it, `file-grpc-service` owns file storage, `mail-grpc-service`
owns outbound email.

**RabbitMQ** carries asynchronous fan-out. Services publish domain events
to the `anahita_events` exchange; consumers (notifications, feed, search)
react. Event names are versioned constants in `anahita-libs/event`, shaped
`node.<type>.<service>.<entity>.<action>.v1` and
`edge.<type>.<service>.<action>.v1`.

## Services

| Service | HTTP port | gRPC port | Responsibility |
| --- | --- | --- | --- |
| `nginx` | 80 | — | Gateway, routing, CORS, token validation subrequest |
| `auth-service` | 8081 | — | OAuth 2.0 / OIDC provider, server-rendered sign-in and sign-up, email verification, registration applications, agreements, password, TOTP, WebAuthn, invites, clients, NodeInfo |
| `group-service` | 8082 | — | Groups |
| `person-service` | 8083 | — | People |
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
| `graph-grpc-service` | — | 50054 | Node/edge graph: actors, media, edges, traversal, trending |
| `file-grpc-service` | — | 50055 | File upload and metadata (S3) |
| `mail-grpc-service` | — | 50051 | Email delivery over SMTP |
| `migration-job` | — | — | One-shot database migrations |
| `account-sweeper` | — | — | Purges accounts past their 30-day deletion grace period. Not started by `up`; run with `docker compose run --rm account-sweeper` |

Backing services in `docker-compose.yml`:

- **MySQL** (3306) — all persistent data
- **Redis** (6379) — sessions
- **RabbitMQ** (5672) — the `anahita_events` exchange

## Routing

nginx maps public paths to services. The full map lives in
`nginx/nginx.conf`:

| Path prefix | Service |
| --- | --- |
| `/oauth/`, `/auth`, `/signup`, `/signup-requests`, `/agreements`, `/onboarding`, `/password`, `/email`, `/username`, `/reauth`, `/totp`, `/webauthn`, `/invites`, `/clients`, `/auth-logs/`, `/oauth-signing-keys`, `/.well-known/`, `/nodeinfo/`, `/static/` | `auth-service` |
| `/articles/`, `/notes/`, `/topics/` | `text-service` |
| `/people/` | `person-service` |
| `/groups/` | `group-service` |
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
| `/socialgraph/` | `socialgraph-service` |
| `/search/` | `search-service` |

`/internal/oauth/validate` is an internal-only subrequest target used by
nginx to validate bearer tokens against `auth-service`. The web app keeps its
tokens server-side behind a session cookie; for the locations that need it,
nginx asks auth-service to translate that cookie into a `Bearer` token before
proxying, so the services only ever see bearer tokens.

## anahita-libs

The shared module every service imports:

| Package | Contents |
| --- | --- |
| `constants` | OAuth scopes, shared enums |
| `proto` | Generated gRPC stubs: `graphpb`, `filepb`, `mailpb`, `typespb` |
| `event` | RabbitMQ connect/publish/consume, event name constants |
| `sessions` | Redis-backed sessions and cookies |
| `storage` | S3 storage backend |
| `crypt` | Encryption with pluggable local / AWS KMS backends |
| `data`, `requests`, `responses`, `validators` | Request/response shapes and validation |
| `middleware`, `cors` | HTTP middleware |
| `geocoding`, `utils`, `cmd` | Supporting helpers |
