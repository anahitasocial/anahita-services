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
| The services in this repository, on `main`, and the images built from it | Here |
| The web app | [anahita-web](https://github.com/anahitasocial/anahita-web), the same way |
| A site that runs Anahita | The people who run it: their address is on the site's `/support` page |

There are no releases yet, so only `main` is supported.

## Not vulnerabilities

These are in the repository on purpose, for development only:

1. The Garage key and RPC secret that `scripts/storage-init.sh` creates, and
   the same values in `docs/getting-started.md`.
2. The values in `*.example` files, and the keys made up for tests.
