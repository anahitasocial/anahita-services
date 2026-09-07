# anahita-services

Anahita as a microservices architecture — the Go rewrite of the Anahita
social networking platform.

The project is a Go workspace (`go.work`, Go 1.25) of loosely coupled
services plus one shared library. Everything is fronted by nginx, which is
the single public entry point; there is no broker service and no etcd —
service addresses are plain DNS names inside the Docker/Kubernetes network,
injected as environment variables.

## Architecture

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

### Services

| Service | HTTP port | gRPC port | Responsibility |
| --- | --- | --- | --- |
| `nginx` | 80 | — | Gateway, routing, CORS, token validation subrequest |
| `auth-service` | 8081 | — | OAuth 2.0 / OIDC provider, sign-up, password, TOTP, WebAuthn, invites, clients |
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

Backing services in `docker-compose.yml`:

- **MySQL** (3306) — all persistent data
- **Redis** (6379) — sessions
- **RabbitMQ** (5672) — the `anahita_events` exchange

### Routing

nginx maps public paths to services. The full map lives in
`nginx/nginx.conf`:

| Path prefix | Service |
| --- | --- |
| `/oauth/`, `/auth`, `/signups`, `/password`, `/totp/`, `/webauthn`, `/invites`, `/clients`, `/authlogs/`, `/oauth-signing-keys`, `/.well-known/`, `/static/` | `auth-service` |
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
nginx to validate bearer tokens against `auth-service`.

### anahita-libs

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

## Running the project

Requires [GNU make](https://www.gnu.org/software/make/) and a recent
[Docker](https://www.docker.com/products/docker-desktop). A `.env` file at
the project root is required before the first run and is not committed —
it supplies `DSN`, `REDIS_ADDR` and `AMQP_ADDR`, the AWS/S3 credentials, the
`SMTP_*` mail settings, the `CRYPT_*` encryption backend, the `OAUTH_*` issuer
and signing-key settings, and the site identity (`DOMAIN_NAME`,
`SYSTEM_EMAIL`, `SUPPORT_EMAIL`). Copy `.env.example` to `.env` and fill it in.
The signing-key and TLS `.pem` files it points at are likewise gitignored.

Mail goes out over SMTP and nothing else, so any provider works — Mailgun's or
SES's relay, Postmark, an internal MTA, or a local sink such as
[mailpit](https://mailpit.axllent.org/) — by pointing `SMTP_HOST`, `SMTP_PORT`
and optionally `SMTP_USER`/`SMTP_PASSWORD` at it. Message templates live in
`mail-grpc-service/templates/`.

From the root of the project:

~~~sh
make up_build
~~~

If the code has not changed, subsequent runs can just be `make up`. To stop
everything:

~~~sh
make down
~~~

The API is then at `http://localhost:80`. The front end lives in a separate
repository (`anahita-react`) and runs at `http://localhost:3000`, which is
the origin nginx allows for CORS.

While working on code you can rebuild a single service:

~~~sh
make auth
~~~

which stops that service, removes its image, rebuilds the binary, and
starts it again. Available service targets:

`auth`, `avatar`, `comment`, `cover`, `feed`, `file`, `graph`, `group`,
`hashtag`, `like`, `location`, `mail`, `nginx`, `notification`, `person`,
`photo`, `search`, `socialgraph`, `story`, `text`

Each has a matching `build_*` target that only compiles the Linux binary,
and `make migration` runs the migration job on its own.

Run `make help` for the full list, which also covers Docker Hub publishing
(`build_dockerfiles`, `push_dockerfiles`), Swarm (`swarm_up`,
`swarm_down`), and Kubernetes/Minikube (`k8s-build`, `k8s-deploy`,
`k8s-build-deploy`, `k8s-cleanup`, `minikube-cleanup`). Manifests are in
`k8s/`, ordered by filename prefix: config and secrets, then backing
services, then gRPC services, then HTTP services, then the ingress.

Tests:

~~~sh
make test
~~~

## OAuth scopes and clients

Scope identifiers are defined **once**, in Go, at
`anahita-libs/constants/oauth_scopes.go`. Every permission check must
reference an `OAuthScope*` constant — never a string literal. `make test`
runs a guard that walks the whole workspace and fails on any
`HasScope("...")` / `RequireScope("...")` with a literal argument.

That guard exists because the fleet once carried three generations of
scope names simultaneously. Since `HasScope` is a plain slice lookup, a
literal matching no constant fails closed and silently — the gate returns
"insufficient scope" forever, for everyone, with nothing in the logs
naming the cause. The compiler catches a mistyped constant; only the
guard catches a reintroduced literal.

Scopes describe **API surface**. Authority (admin / super-admin) is a
separate role check inside each handler's permission method, layered on
top of the write scope. There is deliberately no `admin:*` scope tier.

A scope family covers its sub-resources rather than giving each one its
own scope: `actors:*` covers avatars, covers, access, features and group
admins; `media:*` covers photos, articles, notes, topics and their
hashtag/location tags. Reads of public content need no scope at all — a
read scope gates the authenticated-visibility uplift, not the public
surface — which is why `taxonomy` has no read half.

The current catalog: `openid`, `profile`, `email`, `offline_access`,
`actors:read`, `actors:write`, `media:read`, `media:write`,
`comments:write`, `likes:write`, `socialgraph:read`, `socialgraph:write`,
`notifications:read`, `notifications:write`, `taxonomy:write`,
`account:read`, `account:write`, `oauth:read`, `oauth:write`.

`oauth:*` is the sole super-admin-only family. A token carrying
`oauth:write` is equivalent to owning the identity provider — its holder
can add a redirect URI to any client and harvest authorization codes for
every person on the site.

### Seeding clients

`auth-defaults/oauth_seed_clients.json` bootstraps the first-party
clients (`anahita-web`, `anahita-mobile`) on first boot.

**The seed file is bootstrap-only, not a source of truth.** A client_id
that already exists is left untouched, so editing `scopes` in this file
and restarting does *nothing* — the only signal is a single
`already exists, skipping` log line. That is the right default: it
guarantees a redeploy never silently reverts an edit made through the
`/clients` admin endpoint.

To apply a changed shipped scope list:

~~~sh
docker compose exec mysql mysql -uroot -proot anahita \
  -e "DELETE FROM oauth_clients WHERE client_id = 'anahita-web';"
docker compose restart auth-service
~~~

This cascades to that client's refresh tokens and authorizations, logging
every one of its users out. On a live site, `PATCH /clients/:id` instead.

Scopes are validated against the catalog both at boot (a typo fails
startup) and at the `/clients` BREAD boundary (a bad scope returns 400
naming it).

## Resources

- Anahita nodes and entities examples on the Go playground: https://go.dev/play/p/96bs5YHt84x
- Website: https://www.anahita.io

## License

Apache License 2.0. See [LICENSE](LICENSE).
Copyright 2025 Rastin Mehr Design Studio Inc. (rmdStudio).
