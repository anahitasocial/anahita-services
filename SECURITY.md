# Security

## Reporting a vulnerability

Please do not open a public issue. Report it privately on GitHub instead:
the repository's **Security** tab, then **Report a vulnerability**
(<https://github.com/anahitasocial/anahita-services/security/advisories/new>).
Only the maintainers see it. Without a GitHub account, write to
<security@anahita.io>.

Say what is affected, how to reproduce it, and what somebody could do with
it. You will have a reply within seven days. When it is fixed, the advisory
is published, with your name in it if you would like.

## What this covers

| What | Where to report |
| --- | --- |
| The services, and the images built from them | [anahita-services](https://github.com/anahitasocial/anahita-services/security/advisories/new) |
| The web app | [anahita-web](https://github.com/anahitasocial/anahita-web/security/advisories/new) |
| The website, www.anahita.io | [anahita.io](https://github.com/anahitasocial/anahita.io/security/advisories/new) |
| The MCP server, on npm and at mcp.anahita.io | [anahita-mcp](https://github.com/anahitasocial/anahita-mcp/security/advisories/new) |
| A site that runs Anahita | The people who run it: their address is on the site's `/support` page |

The latest Birth release (a tag such as `v1.0.6`) and `main` (Embryo) are
supported: [Releases](docs/releases.md).

## Not vulnerabilities

These are in the repository on purpose, for development only:

1. The Garage key and RPC secret that `scripts/storage-init.sh` creates, and
   the same values in `docs/getting-started.md`.
2. The values in `*.example` files, and the keys made up for tests.
