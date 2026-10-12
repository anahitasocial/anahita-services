# Downloading what a post holds

*Roadmap item 37, raised by the user 2026-10-11. To be built with the video service (item 16), in M8. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md).*

## Context

Somebody looking at a photo, and later a video or a file attached to a post, should be able to download it **when whoever posted it has said so**. Without that, a picture is still there to be saved by anybody who can see it; the point of the switch is to say what the author is happy with, and to give a proper file, at full size and with a sensible name, in place of whatever the browser happened to be shown.

## What exists

- **Photos** are the only thing there is to download today. Each image of a photo post is kept at several sizes, the largest being `original` (`anahita-libs/data/entities/node_portrait.go`), re-encoded when it was uploaded. A post has up to 4 (`photo_files`).
- **Video** is item 16, not built. **Attached files** are not built or planned: the web app has a `documents` namespace and a Download button for it (`src/containers/controls/medium/Download.jsx`, `api.documents.download`) left from the old application, which call an address no service answers.
- **Pictures are read through signed addresses** that last a few minutes (`file-grpc-service`), sent as part of the post. There is no address for "this file, to keep".
- **Licences** (item 19, kept for later) had downloading as something a licence allows. That plan is not needed for this one; see [Licences](#licences).

## Decisions

Made with the user on 2026-10-11.

1. **A switch on each post**, "Allow download", off unless turned on.
2. **A default on each profile**: a person and a group each say what their new posts start with. A photographer, or a group that shares its pictures, sets it once.
3. **Whoever can edit the post decides**: its author, and the admins of the group it is on.
4. **Planned now, built with video.** The switch is on the post and says nothing about what kind of file it holds, so photos, video and files all use it.

## Design

### The switch

- `nodes.allow_download tinyint(1) NOT NULL DEFAULT 0`, on a post. One migration, in the three places a migration goes (the goose file, `schema/init.sql`, `legacy/upgrade.sql`), checked with `check_sql_order.py`. Every post there is starts with it off: nobody is taken to have agreed.
- `allow_download` on the requests that make and edit a post that can hold a file (photos now; video and files when they exist), and its own address for changing it alone, `PATCH /photos/:id/download` `{ allow }`, beside the ones for access and for who can reply.
- Allowed to whoever may edit the post, by the permission that decides that today. No new rule.
- Sent with the post as `allow_download`, and `authorized.download` for the viewer: true when the switch is on and they can see the post, and always for whoever may edit it, who can have their own file back whatever the switch says.

### The profile's default

- `default_allow_download` on an actor, kept with its other settings, off unless set.
- A new post takes, in this order: what the composer sent; the default of the profile it is posted on, a group's for a post in a group; the author's own default; off.
- Changing the default changes nothing already posted. A separate action, "apply to everything I have posted", is **not** in this plan: it is a way to agree to a great deal at once by accident.

### The download itself

- `GET /photos/:id/download?file=<position>`: answers `302` to a signed address for the largest size kept, signed with `Content-Disposition: attachment` and a file name made from the post's title, or its author's alias and its id, with the right extension. `file-grpc-service`'s Presign takes the disposition; S3-compatible stores honour it in the signature (`response-content-disposition`).
- One image at a time. A post with several offers each; there is no zip. A zip means building one on the server or in the browser for the sake of at most four files.
- **The same access as the post.** The handler reads the post as the viewer and refuses what they cannot see with `404`, then refuses with `403` when the switch is off and they may not edit it. A followers-only photo that allows download is downloadable by its followers and nobody else.
- A visitor on a site in `preview` mode is sent small pictures only (`docs/privacy.md`), and no download.
- Rate-limited for each person with the shared limiter, generously: 60 a minute.
- Nothing is counted and nobody is told. There is no list of who downloaded what, in keeping with there being none of who looked.

### What is downloaded

| Post | File |
| --- | --- |
| Photo | Each image at `original`: the largest copy kept, already re-encoded and stripped of what the camera wrote into it |
| Video (item 16) | The 720p rendition. The upload itself is deleted after it is processed |
| File attached to a post | Not built. When it is, the file as stored, always as an attachment and never shown in the page |

### Web

- **Composer and edit:** "Allow download" in the post's options beside audience, language and who can reply, shown for a kind of post that holds a file, starting on the profile's default.
- **On the post:** a Download button in the row with like, reply, repost and share, where `authorized.download` is true. On a post with several images it downloads the one in view, and in the lightbox the one open.
- **Settings › Access**, for a person and for a group: "Let people download what I post", the default.
- The `documents` leftovers (`controls/medium/Download.jsx`, `api.documents`) are replaced by this or removed.
- Strings in en-GB and fr-FR.

### Licences

If item 19 is built later, a licence is a second way of saying the same thing: choosing one that allows sharing turns the switch on, and "All rights reserved" leaves it as it is. The switch stays the one thing the download address asks about.

### Federation

Nothing is sent. ActivityPub and AT Protocol posts carry their pictures as attachments anybody's server copies; there is no "may download" either would honour. The switch is about this installation's own Download button.

## Estimate

| Part | Time |
| --- | --- |
| Column and migration, the switch on add, edit and its own address, `authorized.download`, the profile default and its order, tests | 3–4 h |
| The download address: access, disposition and file name, rate limit, preview mode, tests | 2–3 h |
| Web: the post option, the button on posts and in the lightbox, the settings switch, removing the `documents` leftovers, strings | 3–4 h |
| Docs (`photos.md`, `permissions.md`, `privacy.md`) | 1 h |
| **Total** | **about 9–12 h, 2–3 sessions** |

Video adds an hour inside item 16: one more address, the same switch.

## Verification

- A post made with nothing said takes the profile's default; in a group, the group's; an explicit choice wins over both.
- With the switch off, a viewer gets `403` and sees no button; the author still downloads their own.
- With it on, a follower downloads a followers-only photo; a stranger gets `404`, the same as for the post.
- The file arrives as a download, not opened in the tab, named for the post, at the largest size kept.
- A post with three images downloads the one asked for; a position that is not there is `404`.
- A group admin can switch it for a post in the group; another member cannot.
- Changing the profile's default leaves existing posts as they were.
- A visitor on a `preview` site is offered no download.

## Dependencies

- **With:** item 16 (video), which it is to be built alongside.
- **Uses:** the shared rate limiter; `file-grpc-service` presigning, which gains a disposition.
- **Later:** item 19 (licences) can set the switch.

## Documentation

- **services `photos.md`**: the switch, the download address, what is downloaded.
- **services `permissions.md`**: who may switch it, and who may download.
- **services `privacy.md`**: no download in preview mode; nothing recorded.
- **web `architecture.md`**: the post option, the button, the setting.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-11 |
| Implementation | not started; with item 16 |
