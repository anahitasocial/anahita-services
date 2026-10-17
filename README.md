# anahita-services

Anahita as a microservices architecture — the Go rewrite of the Anahita
social networking platform.

The project is a Go workspace (`go.work`, Go 1.25) of loosely coupled
services plus one shared library. Everything is fronted by nginx, which is
the single public entry point; there is no broker service and no etcd —
service addresses are plain DNS names inside the Docker/Kubernetes network,
injected as environment variables.

The web app lives in a separate repository, `anahita-web`.

Changing the code, as a person or with an agent: begin with
[AGENTS.md](AGENTS.md), the project's working rules.

## Quick start

Requires GNU make and a recent Docker.

1. Create a `.env` at the project root — see [Configuration](docs/configuration.md).
2. Set `SUPER_ADMIN_EMAIL` and `REGISTRATION_MODE` in it.
3. Start everything:

   ~~~sh
   make up_build
   ~~~

4. Find the link to set the first super administrator's password:

   ~~~sh
   docker compose logs auth-service | grep "super admin seed"
   ~~~

The API is at `http://localhost`, and the web app at `http://localhost:3000`.
The full walk-through is in [Getting started](docs/getting-started.md).

## Documentation

1. [Getting started](docs/getting-started.md) — running the stack locally, first run, developing, tests
2. [Configuration](docs/configuration.md) — every environment variable
3. [Architecture](docs/architecture.md) — services, ports, routing and anahita-libs
4. [OAuth scopes and clients](docs/oauth.md) — how scopes are enforced and clients seeded
5. [Registration and agreements](docs/registration.md) — registration modes, invitations, the approval queue, bot protection, Terms and Privacy Policy

Those are the first five. The [documentation index](docs/README.md) lists
every page: permissions, privacy, signing in, feeds, photos, videos,
deploying and the rest. It also lists the design documents in
[docs/plans](docs/plans/).

## Resources

- Anahita nodes and entities examples on the Go playground: https://go.dev/play/p/96bs5YHt84x
- Website: https://www.anahita.io

## License

Apache License 2.0. See [LICENSE](LICENSE).
Copyright 2025 Rastin Mehr Design Studio Inc. (rmdStudio).
