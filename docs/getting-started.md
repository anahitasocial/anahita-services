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

The web app is a separate repository, `anahita-react`, with its own README.

## Configure

Create a `.env` file at the project root. Every variable is described in [Configuration](configuration.md).

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
repository (`anahita-react`) and runs at `http://localhost:3000`, which is
the origin nginx allows for CORS.

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

`auth`, `avatar`, `comment`, `cover`, `feed`, `file`, `graph`, `group`,
`hashtag`, `like`, `location`, `mail`, `nginx`, `notification`, `person`,
`photo`, `search`, `socialgraph`, `story`, `text`

Each has a matching `build_*` target that only compiles the Linux binary,
and `make migration` runs the migration job on its own.

## Tests

~~~sh
make test
~~~

## Other targets

Run `make help` for the full list, which also covers Docker Hub publishing
(`build_dockerfiles`, `push_dockerfiles`), Swarm (`swarm_up`,
`swarm_down`), and Kubernetes/Minikube (`k8s-build`, `k8s-deploy`,
`k8s-build-deploy`, `k8s-cleanup`, `minikube-cleanup`). Manifests are in
`k8s/`, ordered by filename prefix: config and secrets, then backing
services, then gRPC services, then HTTP services, then the ingress.
