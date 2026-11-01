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

## What step 1 found (2026-11-01)

`govulncheck` in each module, in the workspace as the images are built, Go
1.26.5: 26 modules. The other two, the root module and `examples/grpc/go`, do
not build alone, and no site runs them. The 744 alerts are 12 packages:

| Package | Advisories | Modules | Fixed in | Reached by the code |
| --- | --- | --- | --- | --- |
| Go's standard library | 22 | 26 | Go 1.26.9 | 20 of them: `net/http`, `crypto/tls`, `crypto/x509`, `encoding/xml`, `net/url` |
| `golang.org/x/net` | 14 | 25 | 0.62.0 | HTTP/2 through gRPC, in 25; `html`, in 5 |
| `github.com/rabbitmq/amqp091-go` | 11 | 12 | 1.14.0 | Dialling and channels, in 12. Its advisories need a hostile broker, and the broker is ours |
| `golang.org/x/image` | 9 | 21 | 0.45.0 | `tiff.Decode` in 21, and `bmp` in 1 |
| `google.golang.org/grpc` | 5 | 25 | 1.83.1 | Transport in 25, the server's authorization check in 3 |
| `golang.org/x/crypto` | 4 | 21 | 0.56.0 | Not reached |
| `golang.org/x/text` | 2 | 25 | 0.41.0 | Not reached |
| `github.com/labstack/echo/v4` | 1 | 21 | 4.15.3 | `Static`, in 2: an encoded slash past a route's protection |
| `github.com/yuin/goldmark` | 1 | 20 | 1.7.17 | The link renderer, in 2: cross-site scripting |
| `filippo.io/edwards25519` | 1 | 21 | 1.1.1 | Not reached |
| `github.com/aws/aws-sdk-go-v2` `service/s3`, `eventstream` | 1 | 2 | 1.97.3, 1.7.8 | In 2 |

The images were built with Go 1.25.0, which `go.work` names and
`.github/workflows/images.yml` reads, so step 2 matters most: the standard
library's are fixed by it alone, and they are the ones reached from the
internet.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-28 |
| Step 1 | done 2026-11-01 |
| Steps 2 and 3, Go, gRPC, protobuf and `golang.org/x/*` | done 2026-11-01, together: gRPC 1.76 does not build on Go 1.27 |
| Step 3, Echo, RabbitMQ's client, goldmark, `edwards25519` and the AWS SDK | done 2026-11-01. `govulncheck` now names one advisory, in 21 modules: `golang.org/x/crypto/openpgp`, unmaintained, with no fix, and not called |
| Step 4 | every module's tests, the development stack and the federation lab, against GoToSocial, 2026-11-01: following and posts both ways. On banu.io with `v1.0.9`, 2026-11-01 |
| Step 5 | done 2026-11-01: `SECURITY.md` says why `openpgp` is not a vulnerability here |
| Step 6 | done 2026-11-01: `.github/dependabot.yml`, security updates in one pull request |
| Finished | 2026-11-01, `v1.0.9` |
