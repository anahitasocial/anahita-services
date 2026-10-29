# Dependency alerts

anahita-web roadmap row 113. Planned 2026-10-28, after Dependabot was turned
on for the organization (banu.io's launch plan, row 7) and found 744 alerts on
`main`: 171 critical, 181 high, 373 moderate and 19 low
(<https://github.com/anahitasocial/anahita-services/security/dependabot>).

## Why so many

The number counts each advisory once for every file that names the package.
This repository has 28 Go modules, one `go.mod` for each service and worker
and for `anahita-libs`, and most of them name the same packages (gRPC,
protobuf, `golang.org/x/*`, the AWS SDK, the MySQL and Redis drivers, Echo).
An old version named by twenty modules is twenty alerts. Twenty of the
modules still say `go 1.23.1`. The number of distinct packages to update is
far smaller than 744, and most will be fixed by a few updates made in every
module at once.

An alert is a version known to be vulnerable, not proof that the code calls
the vulnerable part. `govulncheck` says which are reached.

## How

| Step | What |
| --- | --- |
| 1 | List the alerts by package and module (the Dependabot page, or the API with a token), and run `govulncheck ./...` in each module, to see which are reached by the code |
| 2 | Raise Go to one supported version in every module and in the images' builders, the same everywhere |
| 3 | Update the packages named most, in every module at once, a family a commit: `golang.org/x/*`; gRPC and protobuf; the AWS SDK; the database and cache drivers; Echo and the rest of the web stack |
| 4 | Build every image, run every module's tests and the federation lab, and try the development stack, then banu.io, before a Birth release |
| 5 | The rest one by one: a package with no fixed version replaced or left with a note of why, the way `SECURITY.md` names what is not a vulnerability |
| 6 | Dependabot kept quiet afterwards: grouped security updates, one pull request a module family, so that the next advisory is one update and not twenty |

`examples/grpc/python/requirements.txt` is an example, not anything a site
runs; it is updated with the rest so that it does not count.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-28 |
