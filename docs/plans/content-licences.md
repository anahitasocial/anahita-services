# Creative Commons licences on posts

*Roadmap item 19, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
- **What we want:** people pick a licence when they post, or a default applies from their profile settings.
- **Display:** the licence shows next to the post, links to the licence deed, and, when the licence lets others share, offers **download** and a ready-to-copy **attribution** line.
- **Nothing like this exists today.**

## Decisions (item 19)
- **A field on media, not a separate service** (decided 2026-09-28). A licence is one value per post, plus a profile default and a fixed catalogue. A service would add a network call to every media read for one field.
- **All media types:** photos, videos (item 16), articles, notes, topics.

## Design
**Catalogue:** `license-defaults/licenses.json`, following the `feed-defaults` / `reasons.json` pattern (item 4b): baked into images at `/app/defaults`, mountable in compose, and read through `anahita-libs/licenses`.
- **Default list:**
  - All rights reserved (`arr`, the default);
  - CC0 1.0;
  - CC BY 4.0, BY-SA 4.0, BY-ND 4.0, BY-NC 4.0, BY-NC-SA 4.0, BY-NC-ND 4.0.
- **Each entry:**
  - `key`, a `label` per language, `url` (the deed), `spdx` (for example `CC-BY-4.0`) and an icon;
  - `allows_sharing` (false only for `arr`), `allows_commercial`, `allows_derivatives`, `requires_attribution`.
- **Instance settings:** the file can drop licences, and `DEFAULT_CONTENT_LICENSE` sets the instance default (normally `arr`).
- **Legacy keys:** unknown or removed keys still display for posts that already use them, but can't be chosen for new ones.

**Data**
- `nodes.license varchar(32) NULL`, where NULL means the owner's default at the time of posting was `arr`. The create handler always writes the resolved key, so NULL only appears on old rows, which read as `arr`.
- `nodes.license_changed_at datetime NULL`.
- **Profile default:** a person setting, `default_license`, on the person node (a column, or `meta` if we'd rather not widen `nodes`; decide during implementation). **Groups** get their own default too, so a group can say "everything here is CC BY-SA".

**API**
- Media create and edit accept `license`, validated against the catalogue.
- The default is resolved in this order: the composer's choice, then the default of the profile or group it's posted to, then the author's default, then the instance default.
- **Changing a licence later is allowed,** but the web app warns that CC licences can't be withdrawn for copies people already obtained. `license_changed_at` records when it changed.
- Media responses gain `license: { key, label, url, spdx, allows_sharing, … }` and an `attribution` string:
  - generated on the server, for example `"Sunset at Jericho Beach" by Ana (https://DOMAIN/photos/123-sunset) is licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/)`;
  - uses the title when the post has one, otherwise the first words.
- `GET /licenses` returns the catalogue, public and cached.

**Download** (only when `allows_sharing` is true, or for the owner)
- `GET /{ns}/:id/download` returns a 302 to a short-lived presigned URL with `Content-Disposition: attachment` and a sensible filename:
  - photos: the largest stored size (all images if there are several, as a small zip, or one link per image; item 17);
  - videos: the 720p rendition (item 16);
  - articles, notes and topics: a Markdown or plain-text file with the attribution as a header.
- **Access still applies.** A followers-only CC BY post is downloadable only by people who can see it.
- Later, and optional: write the licence into the downloaded image's XMP/EXIF metadata. Uploads are re-encoded and stripped (item 9), so this would be added back on purpose.

**Federation (item 8)**
- ActivityPub: add `"license": "<deed url>"` to Note, Article and Document objects (the schema.org property Pixelfed and PeerTube use). Mastodon ignores it harmlessly.
- Bluesky: no licence field, so append nothing. The licence stays visible on the Anahita post the link card points to.
- **Inbound remote posts:** keep a `license` URL if the remote object carries one, and display it.

**Open Graph (item 10):** optionally add `<link rel="license" href=…>` in the web shell's `<head>` for public posts.

## Web (anahita-web)
- **Composer:** a compact **licence picker** next to the audience picker (item 13). It shows the resolved default ("CC BY 4.0 (your default)") and a menu with short explanations ("Others can share and adapt, even commercially, if they credit you").
- **Settings:**
  - a person "Content licence" section: the default licence, with an "apply to all my existing posts" button that asks for confirmation;
  - a group Settings option for the group's default.
- **On each post:** a small licence badge (CC icons) linking to the deed. When sharing is allowed, it also shows **Download** and **Copy attribution**, which copies the server's attribution string.
- **Edit:** change the licence, with the warning about copies already obtained.
- i18n: licence labels come from the catalogue, and the UI strings are added in en-GB and fr-FR.

## Estimate

| Part | Time |
| --- | --- |
| Catalogue JSON + loader in `anahita-libs`, columns and migration, default resolution, validation, attribution string, `/licenses` | 4–5 h |
| Download endpoint for each media type (presigned attachment, text export, multi-image) | 2–3 h |
| Person and group defaults, "apply to existing posts" batch | 2 h |
| Web: composer picker, badge + download + copy attribution, settings sections, edit warning, i18n | 4–5 h |
| **Total** | **about 12–15 h, 3–4 sessions** |

The ActivityPub `license` property is a one-line addition inside item 8.

## Verification
- A post created with no choice gets the profile default, then the group default in a group, then the instance default. An explicit choice overrides all of them.
- An `arr` post shows no Download (except to its owner), and a CC BY post shows Download and a correct attribution line.
- A followers-only CC0 photo can't be downloaded by a stranger (403 or 404).
- Removing BY-ND from the mounted `licenses.json` hides it from the picker, while existing BY-ND posts still show their badge.
- The ActivityPub Note JSON (item 8) contains the `license` URL.

## Dependencies
- **Soft:**
  - item 13 (the composer, where the licence picker sits next to the audience picker);
  - item 17 and item 16 (which files to download);
  - item 9 (the defaults-path fix for k8s applies to `license-defaults` too);
  - item 8 (the `license` property in ActivityPub objects);
  - item 10 (`rel=license` in the web shell).

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `configuration.md`**: `license-defaults/licenses.json` (format, overriding it) and `DEFAULT_CONTENT_LICENSE`.
- **services `architecture.md`**: the licence field, default resolution, and the download endpoint.
- **web `architecture.md`**: the licence picker, badge, download and copy attribution.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
