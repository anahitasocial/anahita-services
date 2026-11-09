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

**Embryo on banu.io first** (the user, 2026-11-08, as in the PHP Anahita's
days): Anahita's maintainers run the web app's Embryo, anahita-web's
`main`, on their own site, banu.io, and tag a Birth release only once what
it brings has been tried there and works. A site may run the web app's
Embryo too, for what is next before it is released; otherwise it runs the
tagged releases. The services' Embryo has no images, so banu.io runs their
Birth releases.

## With Anahita Web

A Birth release is of both repositories: anahita-web is tagged with the
same version as anahita-services, on the same day. The latest tagged
anahita-web works with the latest tagged anahita-services, and a site runs
the two with the same number, or a web app a maintenance release ahead. Their `main` branches go together the same
way: the web app's `main` may call routes that only the services' `main`
has.

A fix to the web app alone may be a maintenance release of anahita-web
alone, a patch that works with the services' release of the same minor
(the user, 2026-11-08): anahita-web v1.2.1 with anahita-services v1.2.0.
Its changelog names the services' release it goes with. The next release
of both is tagged after the newest of either, so the services may skip a
number, their changelog saying so: from v1.2.0 to v1.2.2, say. A fix that
needs the services as well is a release of both, the services' images that
did not change tagged again, not built ([What is rebuilt](#what-is-rebuilt)).
The same holds the other way: a fix to the services alone, which the web
app's latest release works with, may be a maintenance release of the
services alone (the user, 2026-11-09: anahita-services v1.2.3, beside
anahita-web v1.2.2), its changelog naming the web app's release it goes
with.

## Version numbers

`v<major>.<minor>.<patch>`. Anahita is built and released continuously, and
moves forward a release at a time; the number says what a site has to do
about each (the user, 2026-11-08):

| Part | Goes up when |
| --- | --- |
| Major | A milestone: Anahita stays at 1 until it has most of what other fediverse software offers, and 2.0.0 says so |
| Minor | Something a site, an app or a service of its own relies on changes in a way it must follow: a route removed or answering differently, a setting or an image renamed, a step to take by hand when upgrading |
| Patch | Everything else: new features and fixes, which a site takes by changing the version and nothing more |

So a site goes from one patch to the next without reading more than the
changelog, and stops to read the Upgrading notes at a new minor.

What each Birth release changes, and anything to do when upgrading to it,
is in [CHANGELOG.md](../CHANGELOG.md), from v1.0.17, and in its notes on
GitHub, under
[Releases](https://github.com/anahitasocial/anahita-services/releases).
What `main` has that the last release has not is the changelog's Embryo
section.

## The history

The PHP Anahita's Birth releases ran from 3.0.0 to 4.8.0, its `master`
branch the Embryo. These services, written in Go from 2023, began again at
**v1.0.0**, the first Birth release of Anahita Services, on 2026-10-27.
Anahita Web, Mobile and CLI use the same names; Anahita Web is tagged
with the services' versions from the first Birth release after 2026-11-08,
[With Anahita Web](#with-anahita-web).

## Making one

For whoever maintains anahita-services and anahita-web, which are released
together, with the same version:

1. With both on an up-to-date `main` and nothing uncommitted, make it, from
   anahita-services, saying where anahita-web is checked out (best set once
   in one's shell profile):

   ~~~sh
   ANAHITA_WEB=~/projects/anahita-web make release v1.2.0
   ~~~

   It looks at both first and stops, changing nothing in either, unless
   each is on `main`, clean, level with GitHub, the version after its
   newest tag, and its tests pass (`make test`, `yarn test`). Then, in
   each, it raises the version (`softwareVersion` here, `package.json`
   there), names the changelog's Embryo section after the version and the
   day, commits that, pushes `main`, tags the commit and pushes the tag
   (`scripts/release.sh`). An anahita-web with nothing new is tagged all
   the same, its changelog saying so.

   Or by hand, the same steps, where one would rather see each one done
   ([By hand](#by-hand)).
2. `softwareVersion` is what the site tells the fediverse it runs. It is in
   the code and not taken from the tag, since an image that did not change
   is tagged again rather than built ([What is rebuilt](#what-is-rebuilt)),
   and would go on saying the version it was built at; auth-service is so
   built at every release. The workflow checks it: a tag whose
   `softwareVersion` says another version stops before any image is built,
   and says so.
3. The tag starts `.github/workflows/images.yml`, which publishes every
   image for amd64 and arm64 as `ghcr.io/anahitasocial/<image>:v1.2.0`, and
   as `latest`. It builds only the images that changed since the Birth
   release before: [What is rebuilt](#what-is-rebuilt).
4. On GitHub, in each repository, Releases › Draft a new release, from the
   tag, named "v1.2.0 Birth": its changelog's section for the version, with
   a line saying which release of the other it goes with.
5. In anahita-recipes, once the images are built: the manifests' `ref=`
   and every image in `commons/k8s/kustomization.yml` at the new version,
   renamed images by their new names, and `kubectl kustomize` to see it
   builds. Pinned, not `latest`: a site changes version only when its
   operator does, and reads the Upgrading notes at a new minor.

### By hand

What `make release` does, one step at a time, for v1.2.0 on 2026-11-08:

1. In both, on `main`, clean and level with GitHub, the tests pass:
   `make test` here, `yarn test` in anahita-web.
2. Here: `var softwareVersion = "1.2.0"` in
   `auth-service/cmd/api/handlers/node_info.go`, and `## Embryo` in
   `CHANGELOG.md` renamed `## v1.2.0 Birth, 2026-11-08`; committed as
   `release - v1.2.0`.
3. In anahita-web: `"version": "1.2.0"` in `package.json`, and its
   changelog's `## Embryo` renamed the same, or, with nothing new, a
   section of that name saying "Nothing changed here; tagged with Anahita
   Services."; committed the same.
4. Each pushed, then tagged, anahita-services first:

   ~~~sh
   git push origin main
   git tag v1.2.0 && git push origin v1.2.0
   ~~~

   The tag goes on the commit that says the version: the image workflow
   refuses one whose `softwareVersion` says another. A tag is never moved
   once pushed.

Then steps 2 to 5 above.

Upgrading a site to it: [Upgrading](deploying.md#upgrading).

## What is rebuilt

A fix to one service should publish one new image, not thirty, yet nothing
that changed may be left out. So each image carries a hash of everything it
is made from, as the label `org.anahita.inputs`: its Dockerfile, the image
it starts from, and every file it copies, the compiled binaries among them
(`scripts/image-inputs.sh`).

| When an image's hash is | The release |
| --- | --- |
| The same as in the Birth release before | Tags that release's image with the new version too. Nothing is built, and the image is the very one that ran before |
| Different, or there is no release before | Builds it and publishes it |

The hash is taken from what is built, not from which folders a commit
touched, so a change to `anahita-libs`, to `go.mod` or to a base image
reaches every image it changes, however many commits lie between the two
releases. The binaries are built with `GOFLAGS="-buildvcs=false -trimpath"`,
so that the same code makes the same binary. The run's summary on GitHub
lists what was built and what was kept.

To build everything regardless: Actions › images › Run workflow, with the
version and **rebuild_all** ticked.
