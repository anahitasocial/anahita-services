# Getting started

[Documentation](README.md) › Getting started

Running the whole stack locally with Docker Compose.

## Contents

- [Requirements](#requirements)
- [Configure](#configure)
- [Starting](#starting)
- [First run](#first-run)
- [Developing](#developing)
- [Tests](#tests)
- [Other targets](#other-targets)

## Requirements

- [GNU make](https://www.gnu.org/software/make/)
- A recent [Docker](https://www.docker.com/products/docker-desktop)

The web app is a separate repository, `anahita-web`, with its own README.

## Configure

Create a `.env` file at the project root. Every variable is described in [Configuration](configuration.md).

### Storage for uploads

docker-compose runs [Garage](https://garagehq.deuxfleurs.fr/), a small
S3-compatible object store, so no cloud account is needed. Add these lines to
`.env` to use it:

~~~sh
STORAGE_ENDPOINT=http://garage:3900
STORAGE_PUBLIC_ENDPOINT=http://localhost:3900
STORAGE_REGION=garage
STORAGE_BUCKET=anahita
STORAGE_ACCESS_KEY=GKa0b1c2d3e4f5a6b7c8d9e0f1
STORAGE_SECRET_KEY=a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1
STORAGE_PATH_STYLE=true
~~~

`make up` and `make up_build` prepare the store on first start (the bucket and
that access key) by running `scripts/storage-init.sh`, which is safe to repeat.
Its files are kept in `db-data/garage/`.

The key is published here and in the script, and Garage's own configuration
(`garage/garage.toml`) holds published secrets too. All of it is for local
development only.

To use AWS S3 or another provider instead, see
[Configuration › Storage](configuration.md#storage). Moving files from one
store to another is a job for [rclone](https://rclone.org/): `rclone sync`
between two S3 remotes copies a bucket as it is.

## Starting

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
repository (`anahita-web`) and runs at `http://localhost:3000`, which is
the origin nginx allows for CORS.

Start with `make`, not with `docker-compose up` directly. The services talk
to each other over mutual TLS, and `make up` and `make up_build` first create
the development certificates in `./grpc-certs` (`scripts/grpc-certs-init.sh`).
Without them the services stop at start-up and say so. See
[Internal calls](configuration.md#internal-calls).

## First run

On a fresh database:

1. **Set `SUPER_ADMIN_EMAIL`** before starting. auth-service creates the
   account with no usable password and prints a set-password link, valid for
   48 hours, to its log. It also emails it, but the log works before SMTP is
   configured:

   ~~~sh
   docker compose logs auth-service | grep -i "super admin"
   ~~~

2. **Choose a `REGISTRATION_MODE`.** Unset, the installation is closed and
   nobody can sign up or accept an invitation.
3. **Replace the placeholder Terms and Privacy Policy** in the web app before
   anyone joins — every member is asked to accept whatever is there.

## Developing

Rebuild a single service while working on it:

~~~sh
make auth
~~~

which stops that service, removes its image, rebuilds the binary, and
starts it again. Available service targets:

`abuse_report`, `activitypub`, `atproto`, `actor`, `auth`, `export`, `feed`, `file`, `graph`,
`hashtag`, `identity_image`, `like`, `link_preview`, `location`, `mail`,
`nginx`, `notification`, `photo`, `recommendation`, `search`, `socialgraph`,
`text`, `video`, `web_shell`

Each has a matching `build_*` target that only compiles the Linux binary,
and `make migration` runs the migration job on its own.

- **`video` and `export` each start two containers,** the service and its
  worker. The video worker's image has ffmpeg in it, so nothing is installed
  on your machine. Videos are off until `VIDEO_UPLOADS=on`: see
  [Videos](videos.md).
- **`web_shell` is not started by `make up`.** It sends the web app's build,
  so it has to be given one: `make web_shell WEB_BUILD=<the build folder>`.
  Developing the web app does not need it; see [The web shell](web-shell.md).
- **`make fedi_lab` starts the federation lab,** which is not part of `make up`
  either: see [The federation lab](federation-lab.md).
- **After a change to `anahita-libs` or to a `.proto` file,** rebuild
  everything with `make up_build`: every service is built with the library.

## Tests

~~~sh
make test
~~~

That runs every module's tests and the check of `legacy/upgrade.sql`. Tests
that need a database or a store are skipped unless they are told where one
is, so that nothing is ever run against the database you develop on. Give
them a throwaway MySQL with the schema loaded, and take it away afterwards:

~~~sh
docker run -d --name anahita-test -e MYSQL_ALLOW_EMPTY_PASSWORD=1 \
  -e MYSQL_DATABASE=fresh -p 13309:3306 mysql:9.6
# once it answers:
docker exec -i anahita-test mysql fresh < migration-job/schema/init.sql

cd graph-grpc-service && \
  ACCESS_TEST_DSN='root@tcp(127.0.0.1:13309)/fresh?parseTime=true' go test -p 1 ./...

docker rm -fv anahita-test
~~~

| Variable | Tests it turns on |
| --- | --- |
| `ACCESS_TEST_DSN` | `graph-grpc-service`: who can see what, purging, relationships. Run with `-p 1`, and without `-v` |
| `AUTH_TEST_DSN` | `auth-service`: codes |
| `ABUSE_TEST_DSN` | `abuse-report-service`: cases |
| `RECOMMENDATION_TEST_DSN` | `recommendation-service`: the signals |
| `VIDEO_TEST_DSN` | `video-service`: uploads |
| `EXPORT_TEST_DSN` | `export-service`: exports |
| `ACTIVITYPUB_TEST_DSN` | `activitypub-service`: what the sweep finds that the bus lost |
| `FEDERATION_TEST_DSN` | `anahita-libs/federation`: the queue of what is sent to other servers. It makes and drops a table of its own |
| `STORAGE_TEST_ENDPOINT`, `STORAGE_TEST_PUBLIC_ENDPOINT`, `STORAGE_TEST_REGION`, `STORAGE_TEST_BUCKET`, `STORAGE_TEST_ACCESS_KEY`, `STORAGE_TEST_SECRET_KEY` | `file-grpc-service` and `scripts/s3-migration` against a real bucket: see [Tested stores](configuration.md#tested-stores) |

## Other targets

Run `make help` for the full list, which also covers Docker Hub publishing
(`build_dockerfiles`, `push_dockerfiles`), Swarm (`swarm_up`,
`swarm_down`), and a local Kubernetes cluster (`k8s-start`, `k8s-build`,
`k8s-deploy`, `k8s-cleanup`). The manifests are in `k8s/`, and
[Deploying](deploying.md) covers them, for live sites and for trying them
locally.
