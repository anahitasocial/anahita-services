# Releases

[Documentation](README.md) › Releases

Anahita's releases have two names, as they have had since its first years:
**Embryo** and **Birth**. Not alpha, beta or release candidate.

| Release | What it is | Where it is | For |
| --- | --- | --- | --- |
| **Embryo** | The `main` branch, always. It changes with every commit: new features, fixes, and now and then something experimental that is taken out again | `main` on GitHub. No images are published from it; `make build_dockerfiles` builds your own | Working on Anahita, and trying what is next |
| **Birth** | A version tagged when a milestone is reached and the code is stable: `v1.0.0`, `v1.1.0` | The tag on GitHub, a release named after it ("v1.0.0 Birth"), and the images for it on GitHub's registry | Running a site |

A site runs a Birth release. It names the version in its overlay, so that
it changes only when its operator changes it: [Images](deploying.md#images).

## Version numbers

`v<major>.<minor>.<patch>`, as [semantic versioning](https://semver.org) has
them:

| Part | Goes up when |
| --- | --- |
| Major | Something a site, an app or a service of its own relies on changes in a way it must follow: a route removed or answering differently, a setting renamed, a step to take by hand when upgrading |
| Minor | Something is added, and what was there still works the same |
| Patch | Something is fixed, and nothing else changes |

What each Birth release changes, and anything to do when upgrading to it,
is in its notes on GitHub, under
[Releases](https://github.com/anahitasocial/anahita-services/releases).

## The history

The PHP Anahita's Birth releases ran from 3.0.0 to 4.8.0, its `master`
branch the Embryo. These services, written in Go from 2023, began again at
**v1.0.0**, the first Birth release of Anahita Services, on 2026-10-27.
Anahita Web, Mobile and CLI use the same names.

## Making one

For whoever maintains anahita-services:

1. On an up-to-date `main` that passes `make test`, tag it and push the tag:

   ~~~sh
   git tag v1.0.0 && git push origin v1.0.0
   ~~~

2. The tag starts `.github/workflows/images.yml`, which builds every image
   for amd64 and arm64 and publishes it as
   `ghcr.io/anahitasocial/<image>:v1.0.0`, and as `latest`.
3. On GitHub, Releases › Draft a new release, from the tag, named
   "v1.0.0 Birth": what it adds, changes and fixes, and what to do when
   upgrading.

Upgrading a site to it: [Upgrading](deploying.md#upgrading).
