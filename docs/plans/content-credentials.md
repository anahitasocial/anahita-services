# Content Credentials (roadmap row 131)

Asked for by the user on 2026-11-07, after asking how to tell AI images and
videos apart: "let's definitely implement C2PA checking". Planned the same
day. Not built.

## Contents

1. [Why](#why)
2. [What C2PA is](#what-c2pa-is)
3. [What is settled](#what-is-settled)
4. [Where the check runs](#where-the-check-runs)
5. [What is kept](#what-is-kept)
6. [What people see](#what-people-see)
7. [Out of scope](#out-of-scope)
8. [Phases](#phases)
9. [Documentation](#documentation)

## Why

A picture's camera details prove nothing: they are written in seconds, and
missing from most real photos. A **signed** record of where a picture came
from does: a camera that signs what it takes (recent Leica, Sony and Nikon
models, Google's Pixel phones), or a tool that signs what
it makes (OpenAI, Adobe Firefly, Google, Microsoft). Anahita can check that
signature on upload and say, quietly, what it proves.

What a file says it was made with AI, unsigned, is believed already, by row
122 ([content-warnings.md](content-warnings.md#made-with-ai)). This row is the
signed part.

## What C2PA is

The Coalition for Content Provenance and Authenticity's standard, called
Content Credentials where people see it. A file carries a manifest: what made
it, what was done to it, signed with a certificate. A manifest is believed
when its signature checks out, the file has not changed since it was signed,
and the signer's certificate is on a trust list (C2PA's own, of camera makers
and tool makers).

## What is settled

1. **Reading only.** Anahita checks what comes in; it does not sign what it
   makes. Its sizes of a photo are new files, so the original's signature does
   not hold for them, and Anahita says what it found instead
   ([Out of scope](#out-of-scope) for signing).
2. **Only where it proves something.** A picture with no manifest, or one
   that does not check out, looks like every other picture. A missing
   credential proves nothing, so it is never said.
3. **No Rust of our own.** The C2PA project's own library, built by them
   for each platform, through its C interface.

## Where the check runs

**The library.** The C2PA project publishes its library built for Linux on
both of our platforms (`c2pa-v<version>-x86_64-unknown-linux-gnu.zip` and
`aarch64-unknown-linux-gnu.zip`, from the `c2pa-rs` releases), with its C
header. Its command-line tool, `c2patool`, is published for x86_64 Linux only,
so it would not run on arm64 servers without building it from source, and
there is no image of it to use.

**A service of its own, `credentials-service`.** The library is built for
glibc; the photo and video services run on Alpine (musl) and are pure Go. So
the check is one small internal gRPC service on a glibc base
(`debian:stable-slim`), with cgo and the library, and nothing else:

1. **Asked with a file:** the photo service sends the uploaded original's
   bytes, before it makes its sizes; the video service sends the original's
   address in storage, before it is transcoded, and the service reads it from
   there. It answers with what the manifest proves, or nothing.
2. **Our own thin wrapper** over the library's C interface, reading only:
   about a hundred lines of Go, ours to keep. Of the Go bindings others have
   written, `duggaraju/c2pa-go` (cgo, the same library) could stand in for it,
   and `richardwooding/c2pa` (pure Go, JPEG and PNG only, young) could not
   yet; neither is the C2PA project's own.
3. **The trust list** is a file the service reads at start, from the
   image, refreshed with each release of it, and an operator may point it at
   their own (`C2PA_TRUST_LIST`).
4. **Updating** is changing the library's version and checksums in
   `dockerfiles/credentials-service.dockerfile`, as with any image.
5. **Off by default for a site that does not want it:** a site without the
   service running treats every file as having none. Its time is bounded
   (two seconds a photo); an upload never waits longer or fails because of it.

## What is kept

On the post's file, from the manifest's answer, only what is shown:

| Column | What it holds |
| --- | --- |
| `credential` | `captured` (a camera's), `generated` (made with AI), `edited` (changed by a tool after either), or empty |
| `credential_signer` | Whom the certificate names: "Leica Camera AG", "OpenAI" |
| `credential_tool` | The camera or tool the manifest names: "Leica M11-P", "DALL·E" |
| `credential_at` | When it was signed |

A `generated` answer also chooses row 122's Made with AI in the composer.

## What people see

On a photo or video with a credential, a small mark in a corner (the
Content Credentials "cr" sign, which C2PA asks to be used for it), and
pressed, a short dialog in plain words:

1. "Taken with a Leica M11-P. Signed by Leica Camera AG, November 7, 2026."
2. "Made with AI, with DALL·E. Signed by OpenAI."
3. "Taken with a Pixel 10, then edited. Signed by Google."

Then one line: "The original file's Content Credentials checked out when it
was posted here." A link to C2PA's own explanation.

## Out of scope

1. **Signing Anahita's own sizes of a picture,** so that its credentials go
   on to other sites: a certificate from an authority on the trust list for
   each site, a yearly cost, and worth it only when other networks read them.
   Mastodon and Bluesky remove them today.
2. **Pictures from other servers:** their credentials are removed before
   they reach us.
3. **Profile photos and covers.**
4. **Any judgement from a missing credential.**

## Phases

1. **The service:** the image for both platforms, the wrapper, the trust
   list, tests against C2PA's own sample files: about a session and a half.
2. **Photos and videos:** the photo and video services asking it, the
   columns and migration, the API's `credential`: half a session to a session.
3. **The web app:** the mark and its dialog, in every base language: half a
   session.

About three sessions in all, services and web.

## Documentation

1. anahita-services: `docs/photos.md` and `docs/videos.md` (what is read on
   upload), `docs/architecture.md` (the new service), `docs/deploying.md`
   (the service and `C2PA_TRUST_LIST`), the API reference, `CHANGELOG.md`.
2. anahita-web: `docs/architecture.md` (the mark), `CHANGELOG.md`.
