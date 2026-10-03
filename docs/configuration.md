# Configuration

[Documentation](README.md) › Configuration

Create a `.env` file at the project root before the first run. It is not
committed, and neither are the signing-key and TLS `.pem` files it points at.
`docker-compose.yml` is the reference for which service reads which variable.

## Contents

- [Shared](#shared)
- [Client addresses](#client-addresses)
- [Site identity](#site-identity)
- [Registration](#registration)
- [First super administrator](#first-super-administrator)
- [OAuth signing and encryption](#oauth-signing-and-encryption)
- [Storage](#storage)
- [Mail](#mail)

## Shared

| Variable | Used by | Value |
| --- | --- | --- |
| `ENV` | all | `dev`, `staging` or `prod`. Decides the default public URLs below, and in `dev` all mail goes to `DEVELOPER_EMAIL` |
| `DSN` | services with a database | MySQL DSN, e.g. `root:root@(mysql:3306)/anahita?parseTime=true` |
| `REDIS_ADDR`, `REDIS_PASSWORD` | HTTP services | Redis for sessions, e.g. `redis:6379` |
| `SECURE_COOKIE_KEY` | HTTP services | Key that signs session cookies |
| `AMQP_ADDR` | publishers and consumers | RabbitMQ, e.g. `amqp://rabbitmq:5672` |
| `OAUTH_ISSUER` | all HTTP services | Issuer URL that tokens are signed with and checked against |
| `OAUTH_JWKS_URL` | all HTTP services | Where public signing keys are fetched. Defaults to auth-service inside the network |

### Client addresses

auth-service takes a request's address from `X-Forwarded-For`, reading from the
right and skipping proxies, so an entry a client typed is never used. It treats
loopback, link-local and private addresses as proxies. That fits nginx in
docker-compose and an ingress in a cluster. A proxy on a public address in
front of auth-service would be recorded as the client.

Each proxy has to append to the header, which nginx does with
`$proxy_add_x_forwarded_for`. The address keys the per-address rate limits and
is recorded in the sign-in log.

A sign-in's city and country are looked up at ipapi.co, with a 3 second limit.
Private addresses are never sent, and are recorded as `localhost`,
`development`. Seeing that in a deployed sign-in log means the address reaching
auth-service is a proxy's, not the client's.

## Site identity

Read by auth-service, and mail-grpc-service for the branding.

| Variable | Value |
| --- | --- |
| `DOMAIN_NAME` | The installation's domain. Public URLs are derived from it on staging and production |
| `SITE_NAME` | Name shown on auth pages and in email |
| `ANAHITA_INSTANCE_NAME` | How this installation introduces itself to other servers, published as `nodeName` in NodeInfo. Optional — falls back to `SITE_NAME`, then `DOMAIN_NAME` |
| `ANAHITA_INSTANCE_DESCRIPTION` | A sentence about this installation, published as `nodeDescription`. Optional, and left empty rather than generated when unset |
| `SITE_LOGO_URL` | Logo at the top of every email (mail-grpc-service) |
| `SYSTEM_EMAIL` | The From address |
| `SUPPORT_EMAIL`, `SUPPORT_PHONE`, `SUPPORT_WEBSITE` | Contact details on the web app's public `/support` page, published through NodeInfo. Phone and website are optional |
| `TOTP_ISSUER` | Optional. The label authenticator apps show; derived from `DOMAIN_NAME` when unset |
| `FRONTEND_URL` | Optional, not in `docker-compose.yml`. The web app's URL used in links. Defaults to `http://localhost:3000` in dev and `https://DOMAIN_NAME` in production |
| `AUTH_URL` | Optional, not in `docker-compose.yml`. auth-service's public URL. Defaults to `http://localhost` in dev and `https://auth.DOMAIN_NAME` in production |

## Registration

Read by auth-service.

| Variable | Value |
| --- | --- |
| `REGISTRATION_MODE` | `open`, `approval`, `invite` or `closed`. **Unset means `closed`**: no self-serve sign-up, and outstanding invitations stop working |
| `INVITES_FROM` | `registered`, `administrators` (default) or `super-administrators` — the minimum role that may invite. Each admits that role and every role above it. In `approval` mode, an invitation from somebody who is not an administrator goes to the approval queue |
| `INVITE_PENDING_MAX` | Outstanding invitations one person may have at once. Default `5` |
| `INVITE_PENDING_MAX_ADMIN` | The same for administrators. Default `100` |
| `TOS_URL`, `PRIVACY_URL` | Where the Terms and Privacy Policy are read: a path on the web app or a full URL. Default `/legal/tos` and `/legal/privacy`. Their versions are held by the web app, not here |

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
| `CRYPT_LOCAL_KEY` | Key for `local` |
| `AWS_KMS_KEY_ID` | Key for `kms`; also used by file-grpc-service |
| `MAX_DB_CONNECTIONS` | Optional database pool size |

## Storage

Read by file-grpc-service (and auth-service for AWS credentials).

| Variable | Value |
| --- | --- |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | AWS credentials |
| `AWS_DEFAULT_REGION` | Region, passed to file-grpc-service as `AWS_REGION` |
| `AWS_PROFILE` | Optional named profile |
| `AWS_FILES_S3_BUCKET` | Bucket for uploaded files |
| `AWS_FILES_S3_ASSETS_DIR` | Directory inside the bucket |

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

Service addresses (`MAIL_SERVICE_ADDR`, `FILE_SERVICE_ADDR`,
`GRAPH_SERVICE_ADDR`) and the defaults directories (`DEFAULTS_PATH`,
`ACTOR_FEATURES_PATH`) are set in `docker-compose.yml` itself and need no
entry in `.env`.
