# WebP, HEIC and AVIF photos (roadmap row 135)

From FediDB's comparison with Mastodon, Pixelfed and Loops (2026-11-07).
Planned on 2026-11-08. Not built.

## Contents

1. [Why](#why)
2. [What there is now](#what-there-is-now)
3. [What changes](#what-changes)
4. [Phases](#phases)

## Why

An iPhone takes its photos as HEIC, and Android phones and websites more
and more as WebP and AVIF. Today a person posting one is refused, and a
picture in one of them from another server is not kept, so a post from
Mastodon or Pixelfed can arrive without its picture.

## What there is now

Only JPEG and PNG are taken (`anahita-libs/utils/images.go`, `Accept`),
by photo-service for uploads and by the federation Keeper for pictures from
elsewhere, and every photo is kept at Anahita's sizes as JPEG or PNG.

## What changes

1. **Taken in, kept as JPEG.** A WebP, HEIC or AVIF photo is read, turned
   the right way up, stripped of its metadata as any photo is, and kept at
   the same sizes, as JPEG (or PNG when it has transparency). Nothing
   downstream changes: what is kept and sent is what it is now.
2. **WebP** is read by Go's own `golang.org/x/image/webp`.
3. **HEIC and AVIF** are read by `heif-convert` from Alpine's
   `libheif-tools`, added to photo-service's image, called with a time
   limit on a file in a scratch folder, as video-service calls ffmpeg. The
   services stay built as they are (no cgo). Row 131's content credentials
   may later want a service of their own on glibc; this does not wait for
   it.
4. **Everywhere a picture is taken:** photo uploads, profile photos and
   covers, and pictures from other servers, which most often are WebP.
5. **The web app** offers them in the file picker (`image/webp`,
   `image/heic`, `image/heif`, `image/avif`), and shows a preview of a HEIC
   one from what the server makes, since most browsers cannot draw it.

## Phases

1. WebP, everywhere: half a session.
2. HEIC and AVIF, with the image's new package and the web app's picker
   and preview: one session.
